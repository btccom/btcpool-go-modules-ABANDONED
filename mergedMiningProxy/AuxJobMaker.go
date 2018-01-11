package main

import (
	"container/list"
	"errors"
	"sync"
	"time"

	"github.com/golang/glog"

	"hash"
	"merkle"
)

// AuxPowInfo 辅助工作量证明的信息
// @see <https://en.bitcoin.it/wiki/Merged_mining_specification#Aux_proof-of-work_block>
type AuxPowInfo struct {
	Height           uint32
	Target           hash.Byte32
	BlockchainBranch merkle.MerklePath
}

// AuxPowJob 合并挖矿的任务
type AuxPowJob struct {
	MinBits   string
	MinTarget hash.Byte32

	MerkleRoot  hash.Byte32
	MerkleSize  uint32
	MerkleNonce uint32

	AuxPows []AuxPowInfo

	// padding to RPC response
	PrevBlockHash hash.Byte32
	CoinbaseValue uint64
	Height        uint32
}

// AuxJobMaker 辅助挖矿任务生成器
type AuxJobMaker struct {
	chains           []ChainRPCInfo
	currentAuxBlocks map[int]AuxBlockInfo
	auxPowJobs       map[hash.Byte32]AuxPowJob
	auxPowJobIndex   list.List
	config           AuxJobMakerInfo
	lock             sync.Mutex

	merkleNonce       uint32
	merkleSize        uint32
	chainIDIndexSlots map[uint32]uint32
}

// NewAuxJobMaker 创建辅助挖矿任务构造器
func NewAuxJobMaker(config AuxJobMakerInfo, chains []ChainRPCInfo) (maker *AuxJobMaker) {
	maker = new(AuxJobMaker)
	maker.chains = chains
	maker.currentAuxBlocks = make(map[int]AuxBlockInfo)
	maker.auxPowJobs = make(map[hash.Byte32]AuxPowJob)
	maker.config = config
	return
}

// Run 运行辅助挖矿任务构造器
func (maker *AuxJobMaker) Run() {
	maker.updateAuxBlockAllChains()
}

// GetAuxJob 获取辅助挖矿任务
func (maker *AuxJobMaker) GetAuxJob() (job AuxPowJob, err error) {
	job, err = maker.makeAuxJob()
	if err != nil {
		return
	}

	maker.lock.Lock()
	defer maker.lock.Unlock()

	_, exists := maker.auxPowJobs[job.MerkleRoot]
	if exists {
		if glog.V(2) {
			glog.Info("[GetAuxJob] job not changed")
		}
		return
	}

	maker.auxPowJobs[job.MerkleRoot] = job
	maker.auxPowJobIndex.PushBack(job.MerkleRoot)

	if uint(maker.auxPowJobIndex.Len()) > maker.config.AuxPowJobListSize {
		oldJobMerkleRoot := maker.auxPowJobIndex.Front()
		maker.auxPowJobIndex.Remove(oldJobMerkleRoot)
		delete(maker.auxPowJobs, oldJobMerkleRoot.Value.(hash.Byte32))
	}

	return
}

// FindAuxJob 查找辅助挖矿任务
func (maker *AuxJobMaker) FindAuxJob(merkleRoot hash.Byte32) (job AuxPowJob, err error) {
	job, ok := maker.auxPowJobs[merkleRoot]
	if !ok {
		err = errors.New("AuxJob " + merkleRoot.Hex() + " not found")
	}
	return
}

// updateAuxBlock 更新辅助区块
func (maker *AuxJobMaker) updateAuxBlock(index int) {
	chain := maker.chains[index]
	auxBlockInfo, err := RPCCallCreateAuxBlock(chain)
	if err != nil {
		glog.Warning("CreateAuxBlock for ", chain.Name, " failed: ", err)
	}

	maker.lock.Lock()
	maker.currentAuxBlocks[index] = auxBlockInfo
	maker.lock.Unlock()

	if glog.V(3) {
		glog.Info("[UpdateAuxBlock] <", chain.Name, "> height:", auxBlockInfo.Height, ", bits:", auxBlockInfo.Bits, ", target:", auxBlockInfo.Target.Hex(),
			", coinbaseValue:", auxBlockInfo.CoinbaseValue, ", hash:", auxBlockInfo.Hash.Hex(), ", prevHash:", auxBlockInfo.PrevBlockHash.Hex())
	}
}

// updateAuxBlockAllChains 持续更新所有链的辅助区块
func (maker *AuxJobMaker) updateAuxBlockAllChains() {
	for i := 0; i < len(maker.chains); i++ {
		go func(index int) {
			for {
				maker.updateAuxBlock(index)
				time.Sleep(time.Duration(maker.config.CreateAuxBlockIntervalSeconds) * time.Second)
			}
		}(i)
	}
}

// makeAuxJob 构造辅助挖矿任务
func (maker *AuxJobMaker) makeAuxJob() (job AuxPowJob, err error) {
	maker.lock.Lock()
	defer maker.lock.Unlock()

	blockNum := len(maker.currentAuxBlocks)

	if blockNum < 1 {
		err = errors.New("makeAuxJob failed: currentAuxBlocks is empty")
		return
	}

	if maker.chainIDIndexSlots == nil {
		chainIDs := make([]uint32, blockNum)
		for index, block := range maker.currentAuxBlocks {
			chainIDs[index] = block.ChainID
		}

		maker.merkleNonce, maker.merkleSize, maker.chainIDIndexSlots, _ = assignChainSlots(chainIDs)
		glog.Info("[AssignChainSlots] merkleNonce: ", maker.merkleNonce, ", merkleSize: ", maker.merkleSize,
			", chainIDIndexSlots: ", maker.chainIDIndexSlots)
	}

	job.MerkleNonce = maker.merkleNonce
	job.MerkleSize = maker.merkleSize
	job.AuxPows = make([]AuxPowInfo, blockNum)
	// set default value of Bits and Target
	job.MinBits = maker.currentAuxBlocks[0].Bits
	job.MinTarget = maker.currentAuxBlocks[0].Target
	// set fields that padding to response
	job.PrevBlockHash = maker.currentAuxBlocks[0].PrevBlockHash
	job.Height = maker.currentAuxBlocks[0].Height
	job.CoinbaseValue = maker.currentAuxBlocks[0].CoinbaseValue

	bottomRow := make(merkle.Row, maker.merkleSize)

	for _, block := range maker.currentAuxBlocks {
		bottomRow[maker.chainIDIndexSlots[block.ChainID]] = block.Hash

		// the hex of the target larger, the difficulty of the job smaller
		if block.Target.Hex() > job.MinTarget.Hex() {
			job.MinTarget = block.Target
			job.MinBits = block.Bits
		}
	}

	merkleTree := merkle.NewMerkleTree(bottomRow)
	job.MerkleRoot = merkleTree.MerkleRoot()

	for index, block := range maker.currentAuxBlocks {
		slot := int(maker.chainIDIndexSlots[block.ChainID])
		job.AuxPows[index].Height = block.Height
		job.AuxPows[index].Target = block.Target
		job.AuxPows[index].BlockchainBranch = merkleTree.MerklePathForLeaf(slot)
	}

	return
}
