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
	Target        string
	Height        string
	PrevBlockHash string
	CoinbaseValue string
}

// RPCCreateAuxBlockInfo RPC方法 createauxblock 的请求和响应信息
type RPCCreateAuxBlockInfo struct {
	Method       string
	Params       []interface{}
	ResponseKeys RPCCreateAuxBlockResultKeys
}

// RPCSubmitAuxBlockInfo RPC方法 submitauxblock 的请求和响应信息
type RPCSubmitAuxBlockInfo struct {
	Method string
	Params []interface{}
}

// CoinRPCInfo 合并挖矿币种的RPC信息
type CoinRPCInfo struct {
	ChainID        uint32
	Name           string
	RPCUrl         string
	RPCUser        string
	RPCPasswd      string
	CreateAuxBlock RPCCreateAuxBlockInfo
	SubmitAuxBlock RPCSubmitAuxBlockInfo
}

// ConfigData 配置文件的数据结构
type ConfigData struct {
	RPCUser       string
	RPCPasswd     string
	RPCListenAddr string
	Chains        []CoinRPCInfo
}

// Check 检查配置的合法性
func (conf *ConfigData) Check() (err error) {
	if len(conf.RPCUser) < 1 {
		return errors.New("RPCUser cannot be empty")
	}

	if len(conf.RPCPasswd) < 1 {
		return errors.New("RPCPasswd cannot be empty")
	}

	if len(conf.RPCListenAddr) < 1 {
		return errors.New("RPCListenAddr cannot be empty")
	}

	if len(conf.Chains) < 1 {
		return errors.New("Chains cannot be empty")
	}

	// 检查每个Chain
	for index, chain := range conf.Chains {
		if len(chain.Name) < 1 {
			return errors.New("Chains[" + strconv.Itoa(index) + "].Name cannot be empty")
		}

		if len(chain.RPCUrl) < 1 {
			return errors.New("Chains[" + strconv.Itoa(index) + "].RPCUrl cannot be empty")
		}

		if len(chain.CreateAuxBlock.Method) < 1 {
			return errors.New("Chains[" + strconv.Itoa(index) + "].CreateAuxBlock.Method cannot be empty")
		}

		if len(chain.CreateAuxBlock.ResponseKeys.Hash) < 1 {
			return errors.New("Chains[" + strconv.Itoa(index) + "].CreateAuxBlock.ResponseKeys.Hash cannot be empty")
		}

		if len(chain.CreateAuxBlock.ResponseKeys.Bits) < 1 {
			return errors.New("Chains[" + strconv.Itoa(index) + "].CreateAuxBlock.ResponseKeys.Bits cannot be empty")
		}

		if len(chain.CreateAuxBlock.ResponseKeys.Target) < 1 {
			return errors.New("Chains[" + strconv.Itoa(index) + "].CreateAuxBlock.ResponseKeys.Target cannot be empty")
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
