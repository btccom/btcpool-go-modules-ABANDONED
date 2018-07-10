package main

import (
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

func assignChainSlots(chainIDs map[int]uint32) (merkleNonce uint32, merkleSize uint32, chainIDIndexSlots map[uint32]uint32, slotIndexChainIDs map[uint32]uint32) {
	chainIDIndexSlots = make(map[uint32]uint32)
	slotIndexChainIDs = make(map[uint32]uint32)

	var chainSize uint32
	var slotConflict bool

	chainSize = uint32(len(chainIDs))
	merkleSize = 1
	for merkleSize < chainSize {
		merkleSize *= 2
	}

	glog.Info("[assignChainSlots] init merkleSize: ", merkleSize)

	slotConflict = true
	for retryTimes := 1; slotConflict; retryTimes++ {
		merkleNonce = rand.Uint32()
		slotConflict = false

		for _, chainID := range chainIDs {
			slot := getChainSlot(chainID, merkleSize, merkleNonce)

			if conflictedChainID, ok := slotIndexChainIDs[slot]; ok {
				glog.Info("[assignChainSlots] slot conflicted: chain ", conflictedChainID, " and ", chainID, " got the same slot ", slot)

				slotConflict = true
				// clear maps
				chainIDIndexSlots = make(map[uint32]uint32)
				slotIndexChainIDs = make(map[uint32]uint32)

				// retry too many times, increase the merkle size
				if retryTimes >= 5 {
					retryTimes = 0
					merkleSize *= 2
					glog.Info("[assignChainSlots] merkleSize increased to ", merkleSize)
				}

				break
			}

			slotIndexChainIDs[slot] = chainID
			chainIDIndexSlots[chainID] = slot
		}
	}

	return
}
