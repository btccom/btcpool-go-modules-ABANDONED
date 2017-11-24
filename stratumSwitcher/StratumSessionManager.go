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
	// 修改StratumSessionMap时加的锁
	lock sync.Mutex
	// 所有处于正常代理状态的会话
	sessions StratumSessionMap
	// 会话ID管理器
	sessionIDManager *SessionIDManager
	// Stratum服务器列表
	stratumServerInfoMap StratumServerInfoMap
	// Zookeeper连接
	zookeeperConn *zk.Conn
	// zookeeperSwitcherWatchDir 切换服务监控的zookeeper目录路径
	// 具体监控的路径为 zookeeperSwitcherWatchDir/子账户名
	zookeeperSwitcherWatchDir string
	// 监听的IP和TCP端口
	tcpListenAddr string
	// TCP监听对象
	tcpListener net.Listener
}

// NewStratumSessionManager 创建Stratum会话管理器
func NewStratumSessionManager(conf ConfigData) (manager *StratumSessionManager, err error) {
	manager = new(StratumSessionManager)

	manager.sessionIDManager = NewSessionIDManager(conf.ServerID)
	manager.stratumServerInfoMap = conf.StratumServerMap
	manager.zookeeperSwitcherWatchDir = conf.ZKSwitcherWatchDir
	manager.tcpListenAddr = conf.ListenAddr

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

// Run 开始运行StratumSwitcher服务
func (manager *StratumSessionManager) Run() {
	var err error

	// TCP监听
	glog.Info("Listen TCP ", manager.tcpListenAddr)
	manager.tcpListener, err = net.Listen("tcp", manager.tcpListenAddr)

	if err != nil {
		glog.Fatal("listen failed: ", err)
		return
	}

	if err != nil {
		glog.Fatal("init failed: ", err)
		return
	}

	for {
		conn, err := manager.tcpListener.Accept()

		if err != nil {
			continue
		}

		go manager.RunStratumSession(conn)
	}
}
