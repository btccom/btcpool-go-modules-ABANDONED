package main

import (
	"container/list"
	"encoding/hex"
	"errors"
	"sync"
	"time"

	"github.com/golang/glog"

	"merkle-tree-and-bitcoin/hash"
	"merkle-tree-and-bitcoin/merkle"

	zmq "github.com/pebbe/zmq4"
)

// AuxPowInfo 辅助工作量证明的信息
// @see <https://en.bitcoin.it/wiki/Merged_mining_specification#Aux_proof-of-work_block>
type AuxPowInfo struct {
	Height           uint32
	Hash             hash.Byte32
	Target           hash.Byte32
	BlockchainBranch merkle.MerklePath
}

// AuxPowJob 合并挖矿的任务
type AuxPowJob struct {
	MinBits   string
	MaxTarget hash.Byte32

	MerkleRoot  hash.Byte32
	MerkleSize  uint32
	MerkleNonce uint32

	AuxPows map[int]AuxPowInfo

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

	minJobBits   string
	maxJobTarget hash.Byte32
	blockHashChnel     chan string
}

// NewAuxJobMaker 创建辅助挖矿任务构造器
func NewAuxJobMaker(config AuxJobMakerInfo, chains []ChainRPCInfo) (maker *AuxJobMaker) {
	maker = new(AuxJobMaker)
	maker.chains = chains
	maker.currentAuxBlocks = make(map[int]AuxBlockInfo)
	maker.auxPowJobs = make(map[hash.Byte32]AuxPowJob)
	maker.config = config

	// set max job target and min job bits
	if len(config.MaxJobTarget) != 64 {
		// unlimited
		config.MaxJobTarget = "ffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffff"
	}
	hexBytes, _ := hex.DecodeString(config.MaxJobTarget)
	maker.maxJobTarget.Assign(hexBytes)
	maker.minJobBits, _ = TargetToBits(maker.maxJobTarget.Hex())
	glog.Info("Max Job Target: ", maker.maxJobTarget.Hex(), ", Bits: ", maker.minJobBits)
	maker.blockHashChnel = make(chan string)

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
		return
	}

	maker.lock.Lock()
	// 检查chainID是否更新，如已更新(或oldAuxBlock不存在)，则重置chainIDIndex
	oldAuxBlock, ok := maker.currentAuxBlocks[index]
	if !ok || oldAuxBlock.ChainID != auxBlockInfo.ChainID {
		maker.chainIDIndexSlots = nil
	}

	oldAuxBlockInfo := maker.currentAuxBlocks[index];

	maker.currentAuxBlocks[index] = auxBlockInfo

	if auxBlockInfo.Height >  oldAuxBlockInfo.Height {
		// glog.Info("send blockhash : ", auxBlockInfo.Hash.Hex())
		auxBlockInfo.Hash = auxBlockInfo.Hash.Reverse()
		maker.blockHashChnel <- auxBlockInfo.Hash.Hex()
		auxBlockInfo.Hash = auxBlockInfo.Hash.Reverse()
	}
	maker.lock.Unlock()

	if glog.V(3) {
		glog.Info("[UpdateAuxBlock] <", chain.Name, "> height:", auxBlockInfo.Height, ", bits:", auxBlockInfo.Bits, ", target:", auxBlockInfo.Target.Hex(),
			", coinbaseValue:", auxBlockInfo.CoinbaseValue, ", hash:", auxBlockInfo.Hash.Hex(), ", prevHash:", auxBlockInfo.PrevBlockHash.Hex())
	}
}

// updateAuxBlockAllChains 持续更新所有链的辅助区块
func (maker *AuxJobMaker) updateAuxBlockAllChains() {


   	go func () {
   		txHashChnel := make(chan string)
   		defer close(txHashChnel)
   		notifyPublisher, err := zmq.NewSocket(zmq.PUB)
   		defer notifyPublisher.Close()
		if err != nil {
			glog.Info(" create notifyPublisher handle failed！", err)
			return
		}
		address := "tcp://*:" + maker.config.BlockHashPublishPort
		glog.Info("notifyPublisher address : ", address)
		err = notifyPublisher.Bind(address)
		if err != nil {
			glog.Info(" bind notifyPublisher handle failed！", err)
			return
		}

		go func (out chan<- string) {
			for {
				time.Sleep(time.Duration(maker.config.CreateAuxBlockIntervalSeconds) * time.Second)
				out <- "connect ok!"
			}
		}(txHashChnel)

   		for {
   			select {
   			case txhashmsg := <- txHashChnel:
				notifyPublisher.Send("hashtx", zmq.SNDMORE)
				notifyPublisher.Send(txhashmsg, 0)
			case blockhashmsg := <- maker.blockHashChnel:
				hashByte, _ := hex.DecodeString(blockhashmsg)
				notifyPublisher.Send("hashblock", zmq.SNDMORE)
				notifyPublisher.SendBytes(hashByte, 0)
   			}
   		}
   	}()

	for i := 0; i < len(maker.chains); i++ {
		go func(index int) {
			zmqsignalchanel := make(chan string)
			timeoutchanel := make(chan string)
			go func(out chan<- string) {
				chainsupportzmq := maker.chains[index].IsSupportZmq
				subscriber, _ := zmq.NewSocket(zmq.SUB)
				connected := true
				defer subscriber.Close()
				if chainsupportzmq {
					ip := maker.chains[index].SubBlockHashAddress
					port := maker.chains[index].SubBlockHashPort
					address := "tcp://" + ip + ":"+ port
					glog.Info("address : ",address)
					err := subscriber.Connect(address)
					if err != nil {
	        			glog.Info("[error] ", maker.chains[index].Name, " cannot connect to : ", address)
	        			connected = false
					}
					glog.Info("[OK] ", maker.chains[index].Name, " connected to : ", address)
					subscriber.SetSubscribe("hashblock")
				}
				if chainsupportzmq && connected {
					for {
						msgtype, err := subscriber.Recv(0)
						if err != nil {
							glog.Info("[error] when ", maker.chains[index].Name, " recv type msg ", msgtype)
							continue
						}
						//glog.Info("[OK] ", maker.chains[index].Name, " receive msgtype : ", msgtype)
						content, e := subscriber.Recv(0)
						if e != nil {
							glog.Info("[error] when ", maker.chains[index].Name, " recv content msg ", content)
							continue
						}
						//glog.Info("[OK] ", maker.chains[index].Name, " receive first msgcontent : ", content)

						content, e = subscriber.Recv(0)
						if e != nil {
							glog.Info("[error] when ", maker.chains[index].Name, " recv content msg ", content)
							continue
						}
						//glog.Info("[OK] ", maker.chains[index].Name, " receive second msgcontent : ", content)

						if msgtype != "hashblock" {
							glog.Info("[ERROR] ", maker.chains[index].Name, " receive msgcontent : ", msgtype, "is not hashblock")
							continue
						}

						out <- "ok"
					}
				}
			}(zmqsignalchanel)
			go func(out chan<- string) {
				for {
					time.Sleep(time.Duration(maker.config.CreateAuxBlockIntervalSeconds) * time.Second)
					out <- "ok"
				}
			}(timeoutchanel)

			for {
				select {
					case <- zmqsignalchanel:
						//glog.Info("[ok] recv msg from zmq chanel ---> ")
						maker.updateAuxBlock(index)
					case <- timeoutchanel:
						//glog.Info("[ok] recv msg from timeout chanel ")
						maker.updateAuxBlock(index)
				}
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
		chainIDs := make(map[int]uint32)
		for index, block := range maker.currentAuxBlocks {
			chainIDs[index] = block.ChainID
		}

		maker.merkleNonce, maker.merkleSize, maker.chainIDIndexSlots, _ = assignChainSlots(chainIDs)
		glog.Info("[AssignChainSlots] merkleNonce: ", maker.merkleNonce, ", merkleSize: ", maker.merkleSize,
			", chainIDIndexSlots: ", maker.chainIDIndexSlots)
	}

	job.MerkleNonce = maker.merkleNonce
	job.MerkleSize = maker.merkleSize
	job.AuxPows = make(map[int]AuxPowInfo)
	// set default value of Bits and Target
	job.MinBits = maker.currentAuxBlocks[0].Bits
	job.MaxTarget = maker.currentAuxBlocks[0].Target
	// set fields that padding to response
	job.PrevBlockHash = maker.currentAuxBlocks[0].PrevBlockHash
	job.Height = maker.currentAuxBlocks[0].Height
	job.CoinbaseValue = maker.currentAuxBlocks[0].CoinbaseValue

	bottomRow := make(merkle.Row, maker.merkleSize)

	for _, block := range maker.currentAuxBlocks {
		bottomRow[maker.chainIDIndexSlots[block.ChainID]] = block.Hash

		// the hex of the target larger, the difficulty of the job smaller
		if block.Target.Hex() > job.MaxTarget.Hex() {
			job.MaxTarget = block.Target
			job.MinBits = block.Bits
		}
	}

	merkleTree := merkle.NewMerkleTree(bottomRow)
	job.MerkleRoot = merkleTree.MerkleRoot()

	for index, block := range maker.currentAuxBlocks {
		slot := int(maker.chainIDIndexSlots[block.ChainID])
		var auxPow AuxPowInfo
		auxPow.Height = block.Height
		auxPow.Hash = block.Hash
		auxPow.Target = block.Target
		auxPow.BlockchainBranch = merkleTree.MerklePathForLeaf(slot)
		job.AuxPows[index] = auxPow
	}

	if job.MaxTarget.Hex() > maker.maxJobTarget.Hex() {
		glog.Info("Job target ", job.MaxTarget.Hex(), " too high, replaced to ", maker.maxJobTarget.Hex())
		job.MaxTarget = maker.maxJobTarget
		job.MinBits = maker.minJobBits
	}

	return
}
