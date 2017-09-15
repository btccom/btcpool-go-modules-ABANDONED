package main

import (
	"encoding/json"
	"flag"
	"io/ioutil"
	"net"
	"net/http"
	_ "net/http/pprof"

	"github.com/golang/glog"
)

// ConfigData 配置数据
type ConfigData struct {
	ServerID            uint8
	ListenAddr          string
	StratumServerMap    StratumServerInfoMap
	ZKBroker            []string
	ZKSwitcherWatchDir  string // 以斜杠结尾
	EnableHTTPDebug     bool
	HTTPDebugListenAddr string
}

func main() {
	// 解析命令行参数
	configFilePath := flag.String("config", "./config.json", "Path of config file")
	flag.Parse()

	// 读取配置文件
	configJSON, err := ioutil.ReadFile(*configFilePath)

	if err != nil {
		glog.Fatal("read config failed: ", err)
		return
	}

	configData := new(ConfigData)
	err = json.Unmarshal(configJSON, configData)

	if err != nil {
		glog.Fatal("parse config failed: ", err)
		return
	}

	// 开启HTTP Debug
	if configData.EnableHTTPDebug {
		go func() {
			glog.Info("HTTP debug enabled: ", configData.HTTPDebugListenAddr)
			http.ListenAndServe(configData.HTTPDebugListenAddr, nil)
		}()
	}

	// 若zookeeper路径不以“/”结尾，则添加
	if configData.ZKSwitcherWatchDir[len(configData.ZKSwitcherWatchDir)-1] != '/' {
		configData.ZKSwitcherWatchDir += "/"
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
