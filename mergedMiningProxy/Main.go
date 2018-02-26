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

	/*
		glog.Info("hello")

		// 测试
		hash := hash.Hash([]byte{1, 2, 5, 8, 0})
		tree := merkle.NewMerkleTree(merkle.Row{hash})
		root := tree.MerkleRoot()
		path := tree.MerklePathForLeaf(0)
		glog.Info("hash: ", hash.Hex())
		glog.Info("root: ", root.Hex())
		glog.Info("path: len ", len(path))

		// 测试
		auxPowDataHex := "02000000010000000000000000000000000000000000000000000000000000000000000000ffffffff6303ae3313040d20575a726567696f6e312f50726f6a65637420425443506f6f6c2f20202020202020202020fabe6d6d68ad61d3e33851b9a68cf188036b5e9fa4369dfea8d6a914632df5f77a356875737265693120202001000006000000012e860000ffffffff0297799a09000000001976a914c0174e89bd93eacd1d5a1af4ba1802d412afc08688ac0000000000000000266a24aa21a9ed4da4992830437b84b45f652ff1023484f656be1161673f266ace43e5542362d5000000000000000000005731252ff669b3fcc644c37e50651eeb0e6e32e14a37814ba42907033fa7bc0751dde737fff3a90818c49d1e111c76f002e28c5c61497fdafed663333dd22cea0d2d73ed731d97b4377b756fed6f21d9397416d281b14080f3b95bdb0a8b3d2b47cd165e7e0ebaa19d7d3382fb6c8f24e9f2bc40e4e820b13c6b99fc8f92ffb57371d96082b06fc50783a7b4d5b793d52f2d4ebf919cc19ba55dc9a9f3202a75c3073b23048919132c8edf7461664f54601a8c20b8b05c121587da1e5c5aeda09b803b0c7fe8db388e45b7c3f6f3cbd1278a17746dfd2efbfa05671b85f8d3b02c56950f122c254049c5d58a73033f0966f3ca8dded25dc6241ccd00000000000000000002000020198575f8992ed09c15514add9a07c8c42c51b0e5c9097e562a03000000000000582064d99b09ceb79d8929d9850bfbe8465c3e6be23489a2b62509eb94b54fd00d20575a2548081ae9c57d8f"
		auxPowData, err := ParseAuxPowData(auxPowDataHex)
		glog.Info(err)
		hex := auxPowData.ToHex()

		if hex != auxPowDataHex {
			glog.Fatal("Not equal!\n", auxPowDataHex, "\n", hex)
		} else {
			glog.Info("Equal!")
		}

		bits := "207fffff"
		target, err := BitsToTarget(bits)
		glog.Info("BitsToTarget: ", bits, " -> ", target)
		if err != nil {
			glog.Info("failed: ", err)
		}
	*/

	// 运行任务生成器
	auxJobMaker := NewAuxJobMaker(configData.AuxJobMaker, configData.Chains)
	auxJobMaker.Run()

	// 启动 RPC Server
	runHTTPServer(configData.RPCServer, auxJobMaker)
}
