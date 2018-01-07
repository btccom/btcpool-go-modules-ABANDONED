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

	glog.Info("begin")

	result, err := RPCCallCreateAuxBlock(configData.Chains[0])
	if err != nil {
		glog.Fatal(err)
	}

	glog.Info(result.Hash.Hex())
	glog.Info(result.ChainID)
	glog.Info(result.Bits)
	glog.Info(result.Target.Hex())
	glog.Info(result.Height)
	glog.Info(result.PrevBlockHash.Hex())
	glog.Info(result.CoinbaseValue)

	glog.Info(result.RPCRawResult)

	//--------------------

	/*chainIDs := []uint32{1, 33}
	_, chainIndex, _ := assignChainSlots(chainIDs)

	for chainID, slot := range chainIndex {
		glog.Info("chain id: ", chainID, ", slot: ", slot)
	}*/

	glog.Info("end")
}
