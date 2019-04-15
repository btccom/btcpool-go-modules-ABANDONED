package main

import (
	"encoding/json"
	"errors"
	"io/ioutil"
	"strconv"

	"github.com/golang/glog"
)

// RPCResponseKeys RPC响应的key和本程序要求的key之间的映射
type RPCResponseKeys map[string]string

// RPCCreateAuxBlockResultKeys 映射RPC方法createauxblock的返回结果的key
type RPCCreateAuxBlockResultKeys struct {
	Hash          string
	ChainID       string
	Bits          string
	Height        string
	PrevBlockHash string
	CoinbaseValue string
	Target        string
}

// RPCCreateAuxBlockInfo RPC方法 createauxblock 的请求和响应信息
type RPCCreateAuxBlockInfo struct {
	Method       string
	Params       interface{}
	ResponseKeys RPCCreateAuxBlockResultKeys
}

// RPCSubmitAuxBlockInfo RPC方法 submitauxblock 的请求和响应信息
type RPCSubmitAuxBlockInfo struct {
	Method string
	Params interface{}
}

// ChainRPCServer 合并挖矿的链的RPC服务器
type ChainRPCServer struct {
	URL    string
	User   string
	Passwd string
}

type DBConnectionInfo struct {
	Host       string
	Port       string
	Username   string
	Password   string
	Dbname     string
}


// ChainRPCInfo 合并挖矿币种的RPC信息
type ChainRPCInfo struct {
	ChainID        uint32
	Name           string
	AuxTableName   string
	RPCServer      ChainRPCServer
	CreateAuxBlock RPCCreateAuxBlockInfo
	SubmitAuxBlock RPCSubmitAuxBlockInfo
	SubBlockHashAddress string
	SubBlockHashPort    string
	IsSupportZmq        bool

}

// ProxyRPCServer 该代理的RPC服务器信息
type ProxyRPCServer struct {
	ListenAddr string
	User       string
	Passwd     string
	MainChain  string
	PoolDb     DBConnectionInfo
}

// AuxJobMakerInfo 辅助挖矿任务生成配置
type AuxJobMakerInfo struct {
	CreateAuxBlockIntervalSeconds uint
	AuxPowJobListSize             uint
	MaxJobTarget                  string
	BlockHashPublishPort          string
}

// ConfigData 配置文件的数据结构
type ConfigData struct {
	RPCServer   ProxyRPCServer
	AuxJobMaker AuxJobMakerInfo
	Chains      []ChainRPCInfo
}

// Check 检查配置的合法性
func (conf *ConfigData) Check() (err error) {
	if len(conf.RPCServer.User) < 1 {
		return errors.New("RPCServer.User cannot be empty")
	}

	if len(conf.RPCServer.Passwd) < 1 {
		return errors.New("RPCServer.Passwd cannot be empty")
	}

	if len(conf.RPCServer.ListenAddr) < 1 {
		return errors.New("RPCServer.ListenAddr cannot be empty")
	}

	if len(conf.RPCServer.PoolDb.Host) < 1 {
		return errors.New("RPCServer.PoolDb.Host cannot be empty")
	}

	if len(conf.RPCServer.PoolDb.Port) < 1 {
		return errors.New("RPCServer.PoolDb.Port cannot be empty")
	}

	if len(conf.RPCServer.PoolDb.Username) < 1 {
		return errors.New("RPCServer.PoolDb.Username cannot be empty")
	}

	if len(conf.RPCServer.PoolDb.Password) < 1 {
		return errors.New("RPCServer.PoolDb.Password cannot be empty")
	}

	if len(conf.RPCServer.PoolDb.Dbname) < 1 {
		return errors.New("RPCServer.PoolDb.Dbname cannot be empty")
	}

	if len(conf.Chains) < 1 {
		return errors.New("Chains cannot be empty")
	}

	// 检查每个Chain
	for index, chain := range conf.Chains {
		if len(chain.Name) < 1 {
			return errors.New("Chains[" + strconv.Itoa(index) + "].Name cannot be empty")
		}
		
		if len(chain.AuxTableName) < 1 {
			return errors.New("Chains[" + strconv.Itoa(index) + "].AuxTableName cannot be empty")
		}


		if len(chain.RPCServer.URL) < 1 {
			return errors.New("Chains[" + strconv.Itoa(index) + "].RPCServer.URL cannot be empty")
		}

		if len(chain.CreateAuxBlock.Method) < 1 {
			return errors.New("Chains[" + strconv.Itoa(index) + "].CreateAuxBlock.Method cannot be empty")
		}

		if len(chain.CreateAuxBlock.ResponseKeys.Hash) < 1 {
			return errors.New("Chains[" + strconv.Itoa(index) + "].CreateAuxBlock.ResponseKeys.Hash cannot be empty")
		}

		if len(chain.CreateAuxBlock.ResponseKeys.Bits) < 1 && len(chain.CreateAuxBlock.ResponseKeys.Target) < 1 {
			return errors.New("Chains[" + strconv.Itoa(index) + "].CreateAuxBlock.ResponseKeys.Bits and chain.CreateAuxBlock.ResponseKeys.Target cannot be empty together")
		}

		if chain.ChainID == 0 && len(chain.CreateAuxBlock.ResponseKeys.ChainID) < 1 {
			return errors.New("Chains[" + strconv.Itoa(index) + "].ChainID and Chains[" + strconv.Itoa(index) + "].CreateAuxBlock.ResponseKeys.ChainID all missing")
		}

		if chain.ChainID != 0 && len(chain.CreateAuxBlock.ResponseKeys.ChainID) >= 1 {
			glog.Info("Chains[" + strconv.Itoa(index) + "].ChainID and Chains[" + strconv.Itoa(index) + "].CreateAuxBlock.ResponseKeys.ChainID all defined, use Chains[" + strconv.Itoa(index) + "].CreateAuxBlock.ResponseKeys.ChainID first")
		}
	}

	return nil
}

// LoadFromFile 从文件载入配置
func (conf *ConfigData) LoadFromFile(file string) (err error) {

	configJSON, err := ioutil.ReadFile(file)

	if err != nil {
		return
	}

	err = json.Unmarshal(configJSON, conf)
	if err != nil {
		return
	}

	err = conf.Check()
	return
}

// SaveToFile 保存配置到文件
func (conf *ConfigData) SaveToFile(file string) (err error) {

	configJSON, err := json.Marshal(conf)

	if err != nil {
		return
	}

	err = ioutil.WriteFile(file, configJSON, 0644)
	return
}
