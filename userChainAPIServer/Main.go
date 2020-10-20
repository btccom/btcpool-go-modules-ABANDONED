package main

import (
	"flag"
	"sync"

	"github.com/golang/glog"
)

func main() {
	// 用于等待goroutine结束
	var waitGroup sync.WaitGroup

	// 解析命令行参数
	configFilePath := flag.String("config", "./config.json", "Path of config file")
	flag.Parse()

	configData, err := ReadConfigFile(*configFilePath)
	if err != nil {
		glog.Fatal(err)
	}

	zookeeper, err := NewZookeeper(configData)
	if err != nil {
		glog.Fatal(err)
	}

	manager := NewUserChainManager(configData, zookeeper)

	for chain := range configData.UserListAPI {
		err = manager.FetchUserIDList(chain, false)
		if err != nil {
			glog.Fatal("FetchUserIDList(", chain, ") failed: ", err)
		}
	}

	if len(configData.UserCoinMapURL) > 0 {
		err = manager.FetchUserCoinMap(false)
		if err != nil {
			glog.Fatal("FetchUserCoinMap() failed: ", err)
		}
	}

	// 初始化完成，写入用户币种记录
	err = manager.FlushAllToZK()
	if err != nil {
		glog.Fatal("FlushAllToZK() failed: ", err)
	}

	// 启动用户列表定时更新任务
	for chain := range configData.UserListAPI {
		waitGroup.Add(1)
		go manager.RunFetchUserIDListCronJob(&waitGroup, chain)
	}

	// 启动用户币种映射表定时更新任务
	if len(configData.UserCoinMapURL) > 0 {
		waitGroup.Add(1)
		go manager.RunFetchUserCoinMapCronJob(&waitGroup)
	}

	// 启动API服务器
	if configData.EnableAPIServer {
		waitGroup.Add(1)
		go manager.runAPIServer(&waitGroup)
	}

	// 启动自动注册
	if configData.EnableUserAutoReg {
		waitGroup.Add(1)
		go manager.RunUserAutoReg(&waitGroup)
	}

	waitGroup.Wait()
}
