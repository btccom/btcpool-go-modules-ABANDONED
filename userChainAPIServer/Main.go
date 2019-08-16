package main

import (
	"flag"

	"github.com/btccom/btcpool-go-modules/userChainAPIServer/initUserCoin"
	"github.com/btccom/btcpool-go-modules/userChainAPIServer/switcherAPIServer"
)

func main() {
	// 解析命令行参数
	configFilePath := flag.String("config", "./config.json", "Path of config file")
	flag.Parse()

	go initusercoin.Main(*configFilePath)
	switcherapiserver.Main(*configFilePath)
}
