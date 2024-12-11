package main

import (
	"flag"
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
	var gameport int
	flag.IntVar(&gameport, "gameport", 27015, "The port where the gameserver listens, aka. the port you want to protect")
	var apiport int
	flag.IntVar(&apiport, "apiport", 3531, "The port of the http web API")
	var listenip string
	flag.StringVar(&listenip, "listenip", "127.0.0.1", "On which IP should the http web API listen on")
	var maxrate int
	flag.IntVar(&maxrate, "maxrate", 34, "How many packets are allowed per player per second, best is tickrate+1")
	var logpath string
	flag.StringVar(&logpath, "logpath", "stdout", "Where the logs should be written to, default: stdout")
	var preroute bool
	flag.BoolVar(&preroute, "preroute", false, "Use raw/PREROUTING chain instead of filter/INPUT, use this when using pterodactyl")

	flag.Parse()

	table := "filter"
	mainChain := "INPUT"
	if preroute {
		table = "raw"
		mainChain = "PREROUTING"
	}

	targetport := strconv.Itoa(gameport)
	ginport := strconv.Itoa(apiport)

	starttime := time.Now()
	//Logger
	cfg := zap.NewProductionConfig()
	cfg.OutputPaths = []string{
		logpath,
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
		err = ipt.InsertUnique(table, "GMOD", 5, "-s", hip.IP+"/32", "-p", "udp", "-m", "udp", "--dport", targetport, "-j", "ACCEPT")
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
			err = ipt.InsertUnique(table, "GMOD", 5, "-s", ip+"/32", "-p", "udp", "-m", "udp", "--dport", targetport, "-j", "ACCEPT")
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
		err = ipt.Delete(table, "GMOD", "-s", hip.IP+"/32", "-p", "udp", "-m", "udp", "--dport", targetport, "-j", "ACCEPT")
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
		err = ipt.InsertUnique(table, "GMOD", 1, "-p", "udp", "--dport", targetport, "-m", "hashlimit", "--hashlimit-name", "mainmain", "--hashlimit-above", strconv.Itoa(maxrate)+"/sec", "--hashlimit-mode", "srcip", "-j", "DROP")
		if err != nil {
			panic(err)
		}
		err = ipt.AppendUnique(table, "GMOD", "-p", "udp", "-m", "udp", "--dport", targetport, "-m", "hashlimit", "--hashlimit-above", "1/min", "--hashlimit-burst", "5", "--hashlimit-mode", "srcip", "--hashlimit-name", "main", "-j", "DROP")
		if err != nil {
			panic(err)
		}
		var srcPortsBlocked = []int{53, 123, 1900}
		for _, port := range srcPortsBlocked {
			err = ipt.InsertUnique(table, "GMOD", 1, "-p", "udp", "-m", "udp", "--sport", strconv.Itoa(port), "-j", "DROP")
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
		err = ipt.ClearChain(table, "GMOD")
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
	err = initipt.ClearChain(table, "GMOD")
	if err != nil {
		panic(err)
	}
	//General rule: Max ticks/second packets allowed
	err = initipt.InsertUnique(table, mainChain, 1, "-m", "udp", "-p", "udp", "--dport", targetport, "-j", "GMOD")
	if err != nil {
		logger.Errorw("error inserting main jump rule")
	} else {
		logger.Infow("Added main jump rule successfully")
	}

	/*
	   defer func(){

	   }()
	*/
	donetime := time.Now()
	logger.Infow("Startup finished", "time", donetime.Sub(starttime))
	fmt.Println("Luctus Netprotect started, time taken: ", donetime.Sub(starttime))
	fmt.Println("Listening on " + listenip + ":" + ginport)
	err = app.Run(listenip + ":" + ginport)
	if err != nil {
		panic(err)
	}
}
