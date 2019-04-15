package main

import (
	"flag"

	"github.com/golang/glog"
)

func main() {
	// parse command args
	configFilePath := flag.String("config", "./config.json", "Path of config file")
	flag.Parse()

	// 读取配置文件
	var configData ConfigData
	err := configData.LoadFromFile(*configFilePath)
	if err != nil {
		glog.Fatal("load config failed: ", err)
		return
	}

	// 运行任务生成器
	auxJobMaker := NewAuxJobMaker(configData.AuxJobMaker, configData.Chains)
	auxJobMaker.Run()
	// 启动 RPC Server
	runHTTPServer(configData.RPCServer, auxJobMaker)
}
