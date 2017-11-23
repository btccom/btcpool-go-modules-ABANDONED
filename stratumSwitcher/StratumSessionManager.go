package main

import (
	"net"
	"sync"
	"time"

	"github.com/golang/glog"
	"github.com/samuel/go-zookeeper/zk"
)

// StratumServerInfo Stratum服务器的信息
type StratumServerInfo struct {
	URL string
}

// StratumServerInfoMap Stratum服务器的信息散列表
type StratumServerInfoMap map[string]StratumServerInfo

// StratumSessionMap Stratum会话散列表
type StratumSessionMap map[uintptr]StratumSession

// StratumSessionManager Stratum会话管理器
type StratumSessionManager struct {
	lock                 sync.Mutex
	sessions             StratumSessionMap
	sessionIDManager     *SessionIDManager
	stratumServerInfoMap StratumServerInfoMap
	zookeeperConn        *zk.Conn
	// zookeeperSwitcherWatchDir 切换服务监控的zookeeper目录路径
	// 具体监控的路径为 zookeeperSwitcherWatchDir/子账户名
	zookeeperSwitcherWatchDir string
}

// NewStratumSessionManager 创建Stratum会话管理器
func NewStratumSessionManager(conf ConfigData) (manager *StratumSessionManager, err error) {
	manager = new(StratumSessionManager)

	manager.sessionIDManager = NewSessionIDManager(conf.ServerID)
	manager.stratumServerInfoMap = conf.StratumServerMap
	manager.zookeeperSwitcherWatchDir = conf.ZKSwitcherWatchDir

	// 建立到Zookeeper集群的连接
	manager.zookeeperConn, _, err = zk.Connect(conf.ZKBroker, time.Duration(zookeeperConnTimeout)*time.Second)
	return
}

// RunStratumSession 运行一个Stratum会话
func (manager *StratumSessionManager) RunStratumSession(conn net.Conn) {
	// 产生 sessionID （Extranonce1）
	sessionID, err := manager.sessionIDManager.AllocSessionID()

	if err != nil {
		conn.Close()
		glog.Error("NewStratumSession failed: ", err)
		return
	}

	session := NewStratumSession(manager, conn, sessionID)
	session.Run()
}

// ReleaseStratumSession 释放Stratum会话（在Stratum会话停止时调用）
func (manager *StratumSessionManager) ReleaseStratumSession(sessionID uint32) {
	// 释放sessionID
	manager.sessionIDManager.FreeSessionID(sessionID)
}
