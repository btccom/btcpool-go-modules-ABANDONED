package main

import (
	"encoding/json"
	"io/ioutil"
)

// DatabaseConfig 数据库配置项
type DatabaseConfig struct {
	URI       string
	UserName  string
	Password  string
	DBName    string
	TableName string
}

// MiningIPStatisticsConfig 挖矿IP统计配置项
type MiningIPStatisticsConfig struct {
	Enable                  bool
	ShareLogDirectory       string
	ShareLogFilePattern     string
	MaxAliveIntervalSeconds uint64
	Database                DatabaseConfig
}

// ConfigData 配置项
type ConfigData struct {
	MiningIPStatistics MiningIPStatisticsConfig
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
