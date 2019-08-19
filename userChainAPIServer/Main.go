package main

import (
	"flag"

	initusercoin "github.com/btccom/btcpool-go-modules/userChainAPIServer/initUserCoin"
	switcherapiserver "github.com/btccom/btcpool-go-modules/userChainAPIServer/switcherAPIServer"
)

func main() {
	// 解析命令行参数
	configFilePath := flag.String("config", "./config.json", "Path of config file")
	flag.Parse()

	go switcherapiserver.Main(*configFilePath)
	initusercoin.Main(*configFilePath)
}
