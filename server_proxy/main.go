package main

import (
	"fmt"
	"net"
	"os"
	"syscall"

	"github.com/google/gopacket"
	"github.com/google/gopacket/layers"
	//"os/signal"
	"flag"
	"github.com/gin-gonic/gin"
	"strconv"
	"time"
	//Logging
	ginzap "github.com/gin-contrib/zap"
	"go.uber.org/zap"
	//Metrics
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	//Connectionpool
	"sync"
	"sync/atomic"
)

// Avg Time taken to forward a packet: 85.061µs

type HIP struct {
	IP string `json:"ip" xml:"ip" binding:"required,ip"`
}

type HIPs struct {
	IPs []string `json:"ips" xml:"ips" binding:"required,dive,ip"`
}

func debugDo(adr *net.UDPAddr, size int, text string) {
	fmt.Println(time.Now().Format("15:04:05.00000000"), text, size, "\t", adr.String())
}

func debugNoop(adr *net.UDPAddr, size int, text string) {}

func createForwardSocket(ifName string) (net.PacketConn, error) {
	fd, err := syscall.Socket(syscall.AF_INET, syscall.SOCK_RAW, syscall.IPPROTO_RAW)
	if err != nil {
		return nil, fmt.Errorf("Failed open socket(syscall.AF_INET, syscall.SOCK_RAW, syscall.IPPROTO_RAW): %s", err)
	}
	err = syscall.SetsockoptInt(fd, syscall.IPPROTO_IP, syscall.IP_HDRINCL, 1)
	if err != nil {
		return nil, err
	}
	if ifName != "" {
		_, err = net.InterfaceByName(ifName)
		if err != nil {
			return nil, fmt.Errorf("Failed to find interface: %s: %s", ifName, err)
		}
		err = syscall.SetsockoptString(fd, syscall.SOL_SOCKET, syscall.SO_BINDTODEVICE, ifName)
		if err != nil {
			return nil, err
		}
	}

	conn, err := net.FilePacketConn(os.NewFile(uintptr(fd), fmt.Sprintf("fd %d", fd)))
	if err != nil {
		return nil, err
	}
	return conn, nil
}

func buildUDPPacket(dst *net.UDPAddr, data []byte, src *net.UDPAddr) ([]byte, error) {
	buffer := gopacket.NewSerializeBuffer()
	payload := gopacket.Payload(data)
	ip := &layers.IPv4{
		DstIP:    dst.IP,
		SrcIP:    src.IP,
		Version:  4,
		TTL:      64,
		Protocol: layers.IPProtocolUDP,
	}
	udp := &layers.UDP{
		SrcPort: layers.UDPPort(src.Port),
		DstPort: layers.UDPPort(dst.Port),
	}
	if err := udp.SetNetworkLayerForChecksum(ip); err != nil {
		return nil, fmt.Errorf("Failed calc checksum: %s", err)
	}
	if err := gopacket.SerializeLayers(buffer, gopacket.SerializeOptions{ComputeChecksums: true, FixLengths: true}, ip, udp, payload); err != nil {
		return nil, fmt.Errorf("Failed serialize packet: %s", err)
	}
	return buffer.Bytes(), nil
}

func main() {
	var initAllowedPackets int
	var maxPerSecond int
	var initCooldown int
	var udpport int
	var tcpport int
	var target string
	var loglocation string
	var isDebug bool
	flag.IntVar(&maxPerSecond, "rate.max", 34, "Max packets per second, should be tickrate+1")
	flag.IntVar(&initAllowedPackets, "rate.init", 3, "How many packets to initially let through, should be 3 for gmod")
	flag.IntVar(&initCooldown, "rate.delay", 60, "How long to block/ban packets for, in seconds")
	flag.IntVar(&udpport, "port.udp", 22000, "Port for UDP proxy to listen on")
	flag.IntVar(&tcpport, "port.tcp", 3531, "Port for TCP webserver to listen on")
	flag.StringVar(&target, "target", "0", "Proxy-Target, format '<ip>:<port>', example: 45.142.177.13:27015")
	flag.StringVar(&loglocation, "log", "./netprotect.log", "Location of logs from UDP and TCP servers, can use 'stdout'")
	flag.BoolVar(&isDebug, "debug", false, "Debug: Print packages and timestamps to stdout")
	flag.Parse()
	debugPrint := debugNoop
	if isDebug {
		debugPrint = debugDo
	}

	//Logger

	cfg := zap.NewProductionConfig()
	cfg.OutputPaths = []string{
		loglocation,
	}
	flogger, err := cfg.Build()
	if err != nil {
		panic(err)
	}
	logger := flogger.Sugar()

	//Metrics

	requestCounter := prometheus.NewCounter(
		prometheus.CounterOpts{
			Namespace: "netprotect",
			Name:      "udp_requests_total",
			Help:      "Number of forwarded udp messages",
		},
	)
	prometheus.MustRegister(requestCounter)

	sizeCounter := prometheus.NewCounter(
		prometheus.CounterOpts{
			Namespace: "netprotect",
			Name:      "udp_requests_size_bytes",
			Help:      "Amount of bytes received and sent (per forward only counted once)",
		},
	)
	prometheus.MustRegister(sizeCounter)

	timeCounter := prometheus.NewCounter(
		prometheus.CounterOpts{
			Namespace: "netprotect",
			Name:      "udp_requests_seconds_sum",
			Help:      "Time taken between 'after receiving packet' and 'sent packet out'",
		},
	)
	prometheus.MustRegister(timeCounter)

	//UDP proxy

	addr := net.UDPAddr{
		Port: udpport,
		IP:   net.ParseIP("0.0.0.0"),
	}
	svdst, err := net.ResolveUDPAddr("udp", target)
	if err != nil {
		panic(err)
	}

	sock, err := net.ListenUDP("udp", &addr)
	if err != nil {
		panic(err)
	}
	defer sock.Close()
	recvbuffer := make([]byte, 65000)
	var whitelist sync.Map
	var isWhitelistActive atomic.Bool
	isWhitelistActive.Store(false)

	conn, err := createForwardSocket("eth0")
	if err != nil {
		panic(err)
	}
	dstServer := &net.UDPAddr{
		IP:   net.ParseIP(target),
		Port: 27015,
	}
	dstServerIP := &net.IPAddr{IP: dstServer.IP}

	cooldownTime := time.Duration(initCooldown) * time.Second
	oneSecond := 1 * time.Second

	go func() {
		nextResetAll := time.Now()
		nextReset := make(map[string]time.Time)
		packetCount := make(map[string]int)

		fmt.Println("Netprotect UDP proxy listening on:", sock.LocalAddr(), "forwarding to:", target)
		for {
			n, udpaddr, err := sock.ReadFromUDP(recvbuffer)
			if err != nil {
				sock.Close()
				panic(err)
			}
			sCurTime := time.Now()
			debugPrint(udpaddr, n, strconv.FormatBool(isWhitelistActive.Load()))

			//whitelist, distributed dos prevention
			srcIP := udpaddr.IP.String()
			if _, ok := whitelist.Load(srcIP); !ok && isWhitelistActive.Load() {
				//clear packetCount every 60s
				if sCurTime.After(nextResetAll) {
					packetCount = make(map[string]int)
					nextResetAll = sCurTime.Add(cooldownTime)
				}
				//Do not continue if too many packets
				packetCount[srcIP] += 1
				if packetCount[srcIP] > initAllowedPackets {
					debugPrint(udpaddr, n, "^Above initAllowedPackets")
					continue
				}
			}

			//single dos prevention
			packetCount[srcIP] += 1
			rtime, ok := nextReset[srcIP]
			if !ok {
				nextReset[srcIP] = time.Now()
			}
			if ok && sCurTime.After(rtime) {
				packetCount[srcIP] = 0
				nextReset[srcIP] = sCurTime.Add(oneSecond)
			}
			if packetCount[srcIP] > maxPerSecond {
				debugPrint(udpaddr, n, "^Above maxPerSecond")
				continue
			}

			//Forward packet
			b, err := buildUDPPacket(svdst, recvbuffer[:n], udpaddr)
			if err != nil {
				panic(err)
			}
			_, err = conn.WriteTo(b, dstServerIP)
			if err != nil {
				panic(err)
			}
			requestCounter.Inc()
			sizeCounter.Add(float64(n))
			debugPrint(svdst, n, "<-")
			timeCounter.Add(float64(time.Since(sCurTime)))
		}
	}()

	//TCP webserver

	gin.SetMode(gin.ReleaseMode)
	app := gin.New()
	app.Use(ginzap.RecoveryWithZap(flogger, true))
	app.Use(ginzap.Ginzap(flogger, time.RFC3339, true))

	app.GET("/", func(c *gin.Context) {
		c.String(200, "ok")
	})

	app.GET("/metrics", gin.WrapH(promhttp.Handler()))

	app.POST("/add", func(c *gin.Context) {
		var hip HIP
		if err := c.ShouldBind(&hip); err != nil {
			panic(err)
		}
		whitelist.Store(hip.IP, true)
		c.String(200, "ok")
	})

	app.POST("/addmany", func(c *gin.Context) {
		var hips HIPs
		if err := c.ShouldBind(&hips); err != nil {
			panic(err)
		}
		for _, ip := range hips.IPs {
			whitelist.Store(ip, true)
		}

		c.String(200, "ok")
	})

	app.POST("/del", func(c *gin.Context) {
		var hip HIP
		if err := c.ShouldBind(&hip); err != nil {
			panic(err)
		}
		whitelist.Delete(hip.IP)
		c.String(200, "ok")
	})

	app.POST("/start", func(c *gin.Context) {
		isWhitelistActive.Store(true)
		c.String(200, "ok")
	})

	app.POST("/stop", func(c *gin.Context) {
		isWhitelistActive.Store(false)
		c.String(200, "ok")
	})

	logger.Infow("Startup finished", "tcpport", tcpport, "udpport", udpport, "target", target, "loglocation", loglocation)
	fmt.Println("Netprotect TCP webserver listening on:", "0.0.0.0:"+strconv.Itoa(tcpport))
	fmt.Println(app.Run("0.0.0.0:" + strconv.Itoa(tcpport)))
}
