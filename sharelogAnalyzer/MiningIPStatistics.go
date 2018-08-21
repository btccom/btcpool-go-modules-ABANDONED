package main

import (
	"database/sql"
	"io"
	"os"
	"time"

	_ "github.com/go-sql-driver/mysql"
	"github.com/golang/glog"
)

// MiningIPRecord IP记录
type MiningIPRecord struct {
	puid         int32
	ipv4         uint32
	shareDiffSum uint64
	beginTime    uint64
	endTime      uint64
}

// MiningIPMap IP记录映射
type MiningIPMap map[ /*WorkerID*/ int64]MiningIPRecord

// MiningIPStatistics 挖矿IP统计器
type MiningIPStatistics struct {
	config            MiningIPStatisticsConfig
	shareLogFilePath  string
	shareLogReader    io.Reader
	ipMap             MiningIPMap
	db                *sql.DB
	stmtWriteIPRecord *sql.Stmt

	// counter, only for logs. Don't need thread safe.
	readShareNum      uint64
	writeRecordNum    uint64
	countingBeginTime int64
}

// NewMiningIPStatistics 创建挖矿IP统计器
func NewMiningIPStatistics(config MiningIPStatisticsConfig) (stats *MiningIPStatistics, err error) {
	stats = new(MiningIPStatistics)
	stats.config = config
	stats.ipMap = make(MiningIPMap)
	stats.resetCounter()

	dbConfig := config.Database
	stats.db, err = sql.Open("mysql", dbConfig.UserName+":"+dbConfig.Password+"@tcp("+dbConfig.URI+")/"+dbConfig.DBName+"?charset=utf8")
	if err != nil {
		return
	}

	err = stats.prepareDBQueries()
	return
}

func (stat *MiningIPStatistics) resetCounter() {
	stat.readShareNum = 0
	stat.writeRecordNum = 0
	stat.countingBeginTime = time.Now().Unix()
}

// PrintCountingLog 打印统计日志
func (stat *MiningIPStatistics) PrintCountingLog() {
	t := time.Now().Unix() - stat.countingBeginTime
	glog.Info("read ", stat.readShareNum, " shares, write ", stat.writeRecordNum, " records in ", t, " seconds")
}

func (stat *MiningIPStatistics) prepareDBQueries() (err error) {
	stat.stmtWriteIPRecord, err = stat.db.Prepare("INSERT `" + stat.config.Database.TableName + "`(puid, worker_id, ip, begin_timestamp, end_timestamp, share_sum) VALUES(?, ?, ?, ?, ?, ?) ON DUPLICATE KEY UPDATE end_timestamp=VALUES(end_timestamp), share_sum=VALUES(share_sum)")
	return
}

func (stat *MiningIPStatistics) writeIPRecord(workerID int64, record MiningIPRecord) (err error) {
	if glog.V(3) {
		glog.Info("ip record -> worker: ", workerID, ", IP: ", record.ipv4, ", begin: ", record.beginTime, ", end: ", record.endTime, ", shares: ", record.shareDiffSum)
	}

	_, err = stat.stmtWriteIPRecord.Exec(record.puid, workerID, Long2IP(record.ipv4), record.beginTime, record.endTime, record.shareDiffSum)
	if err != nil {
		glog.Error("Write IP record failed: ", err)
	} else {
		stat.writeRecordNum++
	}

	return
}

func (stat *MiningIPStatistics) openFile() (err error) {
	stat.shareLogFilePath = GetCurrentShareLogFile(stat.config.ShareLogDirectory, stat.config.ShareLogFilePattern)
	glog.Info("open sharelog: ", stat.shareLogFilePath)
	stat.shareLogReader, err = os.Open(stat.shareLogFilePath)
	return
}

// RunCountingThread 运行统计线程
func (stat *MiningIPStatistics) RunCountingThread() {
	stat.resetCounter()
	for {
		time.Sleep(10 * time.Second)
		stat.PrintCountingLog()
		stat.resetCounter()
	}
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
		stat.readShareNum++

		workerID := share.GetWorkerID()
		record, ok := stat.ipMap[workerID]
		if !ok {
			// 首次创建
			record.puid = share.GetUserID()
			record.beginTime = share.GetTime()
			record.endTime = share.GetTime()
			record.ipv4 = share.GetIPv4()
			record.shareDiffSum = share.GetShareDiff()
			// 首次发现该矿机，写入记录
			// 为了性能考虑，不写入。等待最后再写入
			//stat.writeIPRecord(workerID, record)
		} else {
			// 更新
			ipv4 := share.GetIPv4()
			if ipv4 != record.ipv4 || share.GetTime()-record.endTime > stat.config.MaxAliveIntervalSeconds {
				// IP地址改变或矿机长时间不活跃，写入记录
				stat.writeIPRecord(workerID, record)
				// 更新/重置部分字段
				record.beginTime = share.GetTime()
				record.ipv4 = ipv4
				record.shareDiffSum = 0
			}

			record.endTime = share.GetTime()
			record.shareDiffSum += share.GetShareDiff()
		}
		// 保存记录
		stat.ipMap[workerID] = record

		// load next share
		err = share.Load(stat.shareLogReader)
	}

	glog.Info("read sharelog finished: ", err)

	// 写入剩余的记录
	for workerID, record := range stat.ipMap {
		stat.writeIPRecord(workerID, record)
	}
}
