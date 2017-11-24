package main

import (
	"flag"
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

	sessionManager, err := NewStratumSessionManager(configData)
	sessionManager.Run()
}
