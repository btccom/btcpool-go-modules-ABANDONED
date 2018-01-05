package main

import (
	"flag"
	"math/rand"

	"github.com/golang/glog"
)

func getChainSlot(chainID uint32, merkleSize uint32, merkleNonce uint32) (slotNum uint32) {
	rand := merkleNonce*1103515245 + 12345
	rand += chainID
	rand = rand*1103515245 + 12345
	slotNum = rand % merkleSize
	return
}

func assignChainSlots(chainIDs []uint32) (merkleSize uint32, chainIndex map[uint32]uint32, slotIndex map[uint32]uint32) {
	chainIndex = make(map[uint32]uint32)
	slotIndex = make(map[uint32]uint32)

	var merkleNonce uint32
	var chainSize uint32
	var slotConflict bool

	chainSize = uint32(len(chainIDs))
	merkleSize = 1
	for merkleSize < chainSize {
		merkleSize *= 2
	}

	glog.Info("assign chain slots. merkleSize: ", merkleSize)

	slotConflict = true
	for retryTimes := 1; slotConflict; retryTimes++ {
		merkleNonce = rand.Uint32()
		slotConflict = false

		for _, chainID := range chainIDs {
			slot := getChainSlot(chainID, merkleSize, merkleNonce)

			if conflictedChainID, ok := slotIndex[slot]; ok {
				glog.Info("slot conflicted: chain ", conflictedChainID, " and ", chainID, " got the same slot ", slot)

				slotConflict = true
				// clear maps
				chainIndex = make(map[uint32]uint32)
				slotIndex = make(map[uint32]uint32)

				// retry too many times, increase the merkle size
				if retryTimes >= 5 {
					retryTimes = 0
					merkleSize *= 2
					glog.Info("merkleSize increased to ", merkleSize)
				}

				break
			}

			slotIndex[slot] = chainID
			chainIndex[chainID] = slot
		}
	}

	return
}

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
