package main

import (
	"encoding/json"
	"io/ioutil"
)

// ConfigData 配置数据
type ConfigData struct {
	ServerID            uint8
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
