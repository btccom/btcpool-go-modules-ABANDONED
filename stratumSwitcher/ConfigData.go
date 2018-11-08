package main

import (
	"encoding/json"
	"io/ioutil"

	"github.com/golang/glog"
)

// ChainType 区块链类型
type ChainType uint8

const (
	// ChainTypeBitcoin 比特币或类似区块链
	ChainTypeBitcoin ChainType = iota
	// ChainTypeEthereum 以太坊或类似区块链
	ChainTypeEthereum
)

// ConfigData 配置数据
type ConfigData struct {
	ServerID            uint8
	ChainType           string
	ListenAddr          string
	StratumServerMap    StratumServerInfoMap
	ZKBroker            []string
	ZKSwitcherWatchDir  string // 以斜杠结尾
	EnableHTTPDebug     bool
	HTTPDebugListenAddr string
}

// LoadFromFile 从文件载入配置
func (conf *ConfigData) LoadFromFile(file string) (err error) {

	configJSON, err := ioutil.ReadFile(file)

	if err != nil {
		return
	}

	err = json.Unmarshal(configJSON, conf)

	// 若zookeeper路径不以“/”结尾，则添加
	if conf.ZKSwitcherWatchDir[len(conf.ZKSwitcherWatchDir)-1] != '/' {
		conf.ZKSwitcherWatchDir += "/"
	}

	// 若UserSuffix为空，设为与币种相同
	for k, v := range conf.StratumServerMap {
		if v.UserSuffix == "" {
			v.UserSuffix = k
			conf.StratumServerMap[k] = v
		}
		glog.Info("Chain: ", k, ", UserSuffix: ", conf.StratumServerMap[k].UserSuffix)
	}

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

// StratumSessionData Stratum会话数据
type StratumSessionData struct {
	// 会话ID
	SessionID uint32
	// 用户所挖的币种
	MiningCoin string

	ClientConnFD uintptr
	ServerConnFD uintptr

	StratumSubscribeRequest *JSONRPCRequest
	StratumAuthorizeRequest *JSONRPCRequest

	// 比特币AsicBoost挖矿版本掩码
	VersionMask uint32 `json:",omitempty"`
}

// RuntimeData 运行时数据
type RuntimeData struct {
	Action        string
	TCPListenerFD uintptr
	SessionDatas  []StratumSessionData
}

// LoadFromFile 从文件载入配置
func (conf *RuntimeData) LoadFromFile(file string) (err error) {

	configJSON, err := ioutil.ReadFile(file)

	if err != nil {
		return
	}

	err = json.Unmarshal(configJSON, conf)
	return
}

// SaveToFile 保存配置到文件
func (conf *RuntimeData) SaveToFile(file string) (err error) {

	configJSON, err := json.Marshal(conf)

	if err != nil {
		return
	}

	err = ioutil.WriteFile(file, configJSON, 0644)
	return
}
