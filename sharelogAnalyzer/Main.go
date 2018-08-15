package main

import (
	"flag"

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

	if configData.MiningIPStatistics.Enable {
		stat := NewMiningIPStatistics(configData.MiningIPStatistics)
		stat.Run()
	}
}
