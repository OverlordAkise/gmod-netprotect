package main

import (
	"fmt"
	"github.com/gin-gonic/gin"
	"time"
	//Logging
	ginzap "github.com/gin-contrib/zap"
	"go.uber.org/zap"
	//firewall
	"github.com/coreos/go-iptables/iptables"
	"strconv"
)

var logger *zap.SugaredLogger

type HIP struct {
	IP string `json:"ip" xml:"ip" binding:"required,ip"`
}

type HIPs struct {
	IPs []string `json:"ips" xml:"ips" binding:"required,dive,ip"`
}

func main() {
	starttime := time.Now()
	//Logger
	cfg := zap.NewProductionConfig()
	cfg.OutputPaths = []string{
		"stdout",
	}
	flogger, _ := cfg.Build()
	logger = flogger.Sugar()

	//Webserver
	gin.SetMode(gin.ReleaseMode)
	app := gin.New()
	app.Use(ginzap.RecoveryWithZap(flogger, true))
	app.Use(ginzap.Ginzap(flogger, time.RFC3339, true))

	app.GET("/", func(c *gin.Context) {
		c.String(200, "ok")
	})

	app.POST("/add", func(c *gin.Context) {
		var hip HIP
		if err := c.ShouldBind(&hip); err != nil {
			panic(err)
		}
		ipt, err := iptables.New()
		if err != nil {
			panic(err)
		}
		err = ipt.InsertUnique("filter", "GMOD", 5, "-s", hip.IP+"/32", "-p", "udp", "-m", "udp", "--dport", "27015", "-j", "ACCEPT")
		if err != nil {
			panic(err)
		}
		c.String(200, "ok")
	})

	app.POST("/addmany", func(c *gin.Context) {
		var hips HIPs
		if err := c.ShouldBind(&hips); err != nil {
			panic(err)
		}
		ipt, err := iptables.New()
		if err != nil {
			panic(err)
		}
		for _, ip := range hips.IPs {
			err = ipt.InsertUnique("filter", "GMOD", 5, "-s", ip+"/32", "-p", "udp", "-m", "udp", "--dport", "27015", "-j", "ACCEPT")
			if err != nil {
				panic(err)
			}
		}

		c.String(200, "ok")
	})

	app.POST("/del", func(c *gin.Context) {
		var hip HIP
		if err := c.ShouldBind(&hip); err != nil {
			panic(err)
		}
		ipt, err := iptables.New()
		if err != nil {
			panic(err)
		}
		err = ipt.Delete("filter", "GMOD", "-s", hip.IP+"/32", "-p", "udp", "-m", "udp", "--dport", "27015", "-j", "ACCEPT")
		if err != nil {
			panic(err)
		}
		c.String(200, "ok")
	})

	app.POST("/start", func(c *gin.Context) {
		ipt, err := iptables.New()
		if err != nil {
			panic(err)
		}
		err = ipt.InsertUnique("filter", "GMOD", 1, "-p", "udp", "--dport", "27015", "-m", "hashlimit", "--hashlimit-name", "mainmain", "--hashlimit-above", "35/sec", "--hashlimit-mode", "srcip", "-j", "DROP")
		if err != nil {
			panic(err)
		}
		err = ipt.AppendUnique("filter", "GMOD", "-p", "udp", "-m", "udp", "--dport", "27015", "-m", "hashlimit", "--hashlimit-above", "1/min", "--hashlimit-burst", "5", "--hashlimit-mode", "srcip", "--hashlimit-name", "main", "-j", "DROP")
		if err != nil {
			panic(err)
		}
		var srcPortsBlocked = []int{53, 123, 1900}
		for _, port := range srcPortsBlocked {
			err = ipt.InsertUnique("filter", "GMOD", 1, "-p", "udp", "-m", "udp", "--sport", strconv.Itoa(port), "-j", "DROP")
			if err != nil {
				panic(err)
			}
		}

		c.String(200, "ok")
	})

	app.POST("/stop", func(c *gin.Context) {
		ipt, err := iptables.New()
		if err != nil {
			panic(err)
		}
		err = ipt.ClearChain("filter", "GMOD")
		if err != nil {
			panic(err)
		}
		c.String(200, "ok")
	})

	initipt, err := iptables.New()
	if err != nil {
		panic(err)
	}

	//ClearChain clears a chain AND creates it if it doesnt exist yet
	err = initipt.ClearChain("filter", "GMOD")
	if err != nil {
		panic(err)
	}
	//General rule: Max ticks/second packets allowed
	err = initipt.InsertUnique("filter", "INPUT", 1, "-m", "udp", "-p", "udp", "--dport", "27015", "-j", "GMOD")
	if err != nil {
		logger.Infow("INPUT to GMOD rule already installed")
	} else {
		logger.Infow("Added INPUT to GMOD rule successfully")
	}

	/*
	   defer func(){

	   }()
	*/
	donetime := time.Now()
	logger.Infow("Startup finished", "time", donetime.Sub(starttime))
	fmt.Println("Luctus Netprotect started, time taken: ", donetime.Sub(starttime))
	fmt.Println("Listening on 127.0.0.1:3531")
	fmt.Println(app.Run("127.0.0.1:3531"))
}
