package main

import (
	"flag"
	"net"
	"net/http"
	_ "net/http/pprof"

	"github.com/golang/glog"
)

func main() {
	// 解析命令行参数
	configFilePath := flag.String("config", "./config.json", "Path of config file")
	flag.Parse()

	// 读取配置文件
	var configData ConfigData
	err := configData.LoadFromFile(*configFilePath)

	if err != nil {
		glog.Fatal("load config failed: ", err)
		return
	}

	// 开启HTTP Debug
	if configData.EnableHTTPDebug {
		go func() {
			glog.Info("HTTP debug enabled: ", configData.HTTPDebugListenAddr)
			http.ListenAndServe(configData.HTTPDebugListenAddr, nil)
		}()
	}

	// TCP监听
	glog.Info("Listen TCP ", configData.ListenAddr)
	ln, err := net.Listen("tcp", configData.ListenAddr)

	if err != nil {
		glog.Fatal("listen failed: ", err)
		return
	}

	err = StratumSessionGlobalInit(configData.ServerID, configData.StratumServerMap, configData.ZKBroker, configData.ZKSwitcherWatchDir)

	if err != nil {
		glog.Fatal("init failed: ", err)
		return
	}

	for {
		conn, err := ln.Accept()

		if err != nil {
			continue
		}

		session, err := NewStratumSession(conn)

		if err != nil {
			conn.Close()
			glog.Error("NewStratumSession failed: ", err)
		}

		go session.Run()
	}
}
