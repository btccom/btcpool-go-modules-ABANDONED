package main

import (
	"flag"

	"github.com/golang/glog"
)

func main() {
	// parse command args
	flag.Parse()

	glog.Info("begin")

	chainIDs := []uint32{1, 33}
	_, chainIndex, _ := assignChainSlots(chainIDs)

	for chainID, slot := range chainIndex {
		glog.Info("chain id: ", chainID, ", slot: ", slot)
	}

	glog.Info("end")
}
