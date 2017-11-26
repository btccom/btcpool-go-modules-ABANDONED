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
	// 不停机升级时保存的运行状态文件
	runtimeFilePath := flag.String("runtime", "", "Path of runtime file, use for zero downtime upgrade.")
	flag.Parse()

	// 读取配置文件
	var configData ConfigData
	err := configData.LoadFromFile(*configFilePath)

	if err != nil {
		glog.Fatal("load config failed: ", err)
		return
	}

	// 读取运行时状态
	var runtimeData RuntimeData

	if len(*runtimeFilePath) > 0 {
		runtimeData.LoadFromFile(*runtimeFilePath)
	}

	// 开启HTTP Debug
	if configData.EnableHTTPDebug {
		go func() {
			glog.Info("HTTP debug enabled: ", configData.HTTPDebugListenAddr)
			http.ListenAndServe(configData.HTTPDebugListenAddr, nil)
		}()
	}

	sessionManager, err := NewStratumSessionManager(configData)
	if err != nil {
		glog.Fatal("create session manager failed: ", err)
		return
	}
	sessionManager.Run(runtimeData)
}
