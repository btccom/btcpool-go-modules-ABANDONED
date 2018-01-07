package main

import (
	"encoding/json"
	"io/ioutil"
)

// RPCResponseKeys RPC响应的key和本程序要求的key之间的映射
type RPCResponseKeys map[string]string

// RPCMethodInfo RPC方法的请求和响应信息
type RPCMethodInfo struct {
	Method       string
	Params       []string
	ResponseKeys RPCResponseKeys
}

// CoinRPCInfo 合并挖矿币种的RPC信息
type CoinRPCInfo struct {
	Name           string
	RPCUrl         string
	RPCUser        string
	RPCPasswd      string
	CreateAuxBlock RPCMethodInfo
}

// ConfigData 配置文件的数据结构
type ConfigData struct {
	RPCUser       string
	RPCPasswd     string
	RPCListenAddr string
	Chains        []CoinRPCInfo
}

// LoadFromFile 从文件载入配置
func (conf *ConfigData) LoadFromFile(file string) (err error) {

	configJSON, err := ioutil.ReadFile(file)

	if err != nil {
		return
	}

	err = json.Unmarshal(configJSON, conf)
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
