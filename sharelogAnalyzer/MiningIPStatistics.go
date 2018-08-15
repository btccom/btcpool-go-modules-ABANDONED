package main

import (
	"io"
	"os"

	"github.com/golang/glog"
)

// MiningIPRecord IP记录
type MiningIPRecord struct {
	IPv4         uint32
	shareDiffSum uint64
	beginTime    uint64
	endTime      uint64
}

// MiningIPMap IP记录映射
type MiningIPMap map[ /*WorkerID*/ int64]MiningIPRecord

// MiningIPStatistics 挖矿IP统计器
type MiningIPStatistics struct {
	config           MiningIPStatisticsConfig
	shareLogFilePath string
	shareLogReader   io.Reader
	ipMap            MiningIPMap
}

// NewMiningIPStatistics 创建挖矿IP统计器
func NewMiningIPStatistics(config MiningIPStatisticsConfig) *MiningIPStatistics {
	stats := new(MiningIPStatistics)
	stats.config = config
	stats.ipMap = make(MiningIPMap)

	return stats
}

func (stat *MiningIPStatistics) openFile() (err error) {
	stat.shareLogFilePath = GetCurrentShareLogFile(stat.config.ShareLogDirectory, stat.config.ShareLogFilePattern)
	stat.shareLogReader, err = os.Open(stat.shareLogFilePath)
	return
}

// Run 运行挖矿IP统计器
func (stat *MiningIPStatistics) Run() {
	err := stat.openFile()
	if err != nil {
		glog.Warning("cannot open sharelog: ", err)
		return
	}

	var share ShareBitcoinV1

	err = share.Load(stat.shareLogReader)
	for err == nil {
		workerID := share.GetWorkerID()
		record, ok := stat.ipMap[workerID]
		if !ok {
			// 首次创建
			record.beginTime = share.GetTime()
			record.endTime = share.GetTime()
			record.IPv4 = share.GetIPv4()
			record.shareDiffSum = share.GetShareDiff()
			// 首次发现该矿机，写入记录
			stat.writeIPRecord(workerID, record)
		} else {
			// 更新
			ipv4 := share.GetIPv4()
			if ipv4 != record.IPv4 {
				// IP地址改变，写入记录
				stat.writeIPRecord(workerID, record)
				// 更新/重置部分字段
				record.IPv4 = ipv4
				record.shareDiffSum = 0
			}

			record.endTime = share.GetTime()
			record.shareDiffSum += share.GetShareDiff()
		}

		// load next share
		err = share.Load(stat.shareLogReader)
	}

	glog.Info("load share errmsg: ", err)
}

func (stat *MiningIPStatistics) writeIPRecord(workerID int64, record MiningIPRecord) (err error) {
	glog.Info("ip record -> worker: ", workerID, ", IP: ", record.IPv4, ", begin: ", record.beginTime, ", end: ", record.endTime, ", shares: ", record.shareDiffSum)
	return
}
