package main

import (
	"errors"
	"net"
	"sync"
	"time"

	"github.com/golang/glog"
	"github.com/samuel/go-zookeeper/zk"
)

// zookeeper连接超时时间
const zookeeperConnectingTimeoutSeconds = 60

// StratumServerInfo Stratum服务器的信息
type StratumServerInfo struct {
	URL string
}

// StratumServerInfoMap Stratum服务器的信息散列表
type StratumServerInfoMap map[string]StratumServerInfo

// StratumSessionMap Stratum会话散列表
type StratumSessionMap map[uint32]*StratumSession

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
	// 无停机升级对象
	upgradable *Upgradable
}

// NewStratumSessionManager 创建Stratum会话管理器
func NewStratumSessionManager(conf ConfigData) (manager *StratumSessionManager, err error) {
	manager = new(StratumSessionManager)

	manager.sessions = make(StratumSessionMap)
	manager.sessionIDManager = NewSessionIDManager(conf.ServerID)
	manager.stratumServerInfoMap = conf.StratumServerMap
	manager.zookeeperSwitcherWatchDir = conf.ZKSwitcherWatchDir
	manager.tcpListenAddr = conf.ListenAddr

	// 建立到Zookeeper集群的连接
	var event <-chan zk.Event
	manager.zookeeperConn, event, err = zk.Connect(conf.ZKBroker, time.Duration(zookeeperConnTimeout)*time.Second)
	if err != nil {
		return
	}

	zkConnected := make(chan bool, 1)

	go func() {
		glog.Info("Zookeeper: waiting for connecting to ", conf.ZKBroker, "...")
		for {
			e := <-event
			glog.Info("Zookeeper: ", e)

			if e.State == zk.StateConnected {
				zkConnected <- true
				return
			}
		}
	}()

	select {
	case <-zkConnected:
		break
	case <-time.After(zookeeperConnectingTimeoutSeconds * time.Second):
		err = errors.New("Zookeeper: connecting timeout")
		break
	}

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

// ResumeStratumSession 恢复一个Stratum会话
func (manager *StratumSessionManager) ResumeStratumSession(sessionData StratumSessionData) {
	clientConn, clientErr := newConnFromFd(sessionData.ClientConnFD)
	serverConn, serverErr := newConnFromFd(sessionData.ServerConnFD)

	if clientErr != nil {
		glog.Error("Resume client conn failed: ", clientErr)
		return
	}

	if serverErr != nil {
		glog.Error("Resume server conn failed: ", clientErr)
		return
	}

	if clientConn.RemoteAddr() == nil {
		glog.Error("Resume client conn failed: downstream exited.")
		return
	}

	if serverConn.RemoteAddr() == nil {
		glog.Error("Resume client conn failed: upstream exited.")
		return
	}

	//恢复sessionID
	err := manager.sessionIDManager.ResumeSessionID(sessionData.SessionID)
	if err != nil {
		glog.Error("Resume server conn failed: ", err)
	}

	session := NewStratumSession(manager, clientConn, sessionData.SessionID)
	go session.Resume(sessionData, serverConn)
}

// RegisterStratumSession 注册Stratum会话（在Stratum会话开始正常代理之后调用）
func (manager *StratumSessionManager) RegisterStratumSession(session *StratumSession) {
	manager.lock.Lock()
	manager.sessions[session.sessionID] = session
	manager.lock.Unlock()
}

// ReleaseStratumSession 释放Stratum会话（在Stratum会话停止时调用）
func (manager *StratumSessionManager) ReleaseStratumSession(session *StratumSession) {
	// 删除已注册的会话
	manager.lock.Lock()
	delete(manager.sessions, session.sessionID)
	manager.lock.Unlock()
	// 释放会话ID
	manager.sessionIDManager.FreeSessionID(session.sessionID)
}

// Run 开始运行StratumSwitcher服务
func (manager *StratumSessionManager) Run(runtimeData RuntimeData) {
	var err error

	if runtimeData.Action == "upgrade" {
		// 恢复 TCP 会话
		for _, sessionData := range runtimeData.SessionDatas {
			manager.ResumeStratumSession(sessionData)
		}

		// 恢复之前的TCP监听
		// 可能会恢复失败。若恢复失败，则重新监听。
		glog.Info("Resume TCP Listener: fd ", runtimeData.TCPListenerFD)
		manager.tcpListener, err = newListenerFromFd(runtimeData.TCPListenerFD)

		if err != nil {
			glog.Error("resume failed: ", err)
			manager.tcpListener = nil
		}
	}

	// 全新监听，或在恢复监听失败时重新监听
	if manager.tcpListener == nil {
		// TCP监听
		glog.Info("Listen TCP ", manager.tcpListenAddr)
		manager.tcpListener, err = net.Listen("tcp", manager.tcpListenAddr)

		if err != nil {
			glog.Fatal("listen failed: ", err)
			return
		}
	}

	manager.Upgradable()

	for {
		conn, err := manager.tcpListener.Accept()

		if err != nil {
			continue
		}

		go manager.RunStratumSession(conn)
	}
}

// Upgradable 使StratumSwitcher可无停机升级
func (manager *StratumSessionManager) Upgradable() {
	manager.upgradable = NewUpgradable(manager)

	go signalUSR2Listener(func() {
		err := manager.upgradable.upgradeStratumSwitcher()
		if err != nil {
			glog.Error("Upgrade Failed: ", err)
		}
	})

	glog.Info("Stratum Switcher is Now Upgradable.")
}
