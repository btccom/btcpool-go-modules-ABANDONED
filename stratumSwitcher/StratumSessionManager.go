package main

import (
	"errors"
	"net"
	"strings"
	"sync"

	"github.com/golang/glog"
)

// StratumServerInfo Stratum服务器的信息
type StratumServerInfo struct {
	URL        string
	UserSuffix string
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
	// Zookeeper管理器
	zookeeperManager *ZookeeperManager
	// zookeeperSwitcherWatchDir 切换服务监控的zookeeper目录路径
	// 具体监控的路径为 zookeeperSwitcherWatchDir/子账户名
	zookeeperSwitcherWatchDir string
	// enableUserAutoReg 是否打开子账户自动注册功能
	enableUserAutoReg bool
	// zookeeperAutoRegWatchDir 自动注册服务监控的zookeeper目录路径
	// 具体监控的路径为 zookeeperAutoRegWatchDir/子账户名
	zookeeperAutoRegWatchDir string
	// 当前允许的自动注册用户数（注册一个减1，完成后加回来，到0拒绝自动注册，以防DDoS）
	autoRegAllowUsers int64
	// 监听的IP和TCP端口
	tcpListenAddr string
	// TCP监听对象
	tcpListener net.Listener
	// 无停机升级对象
	upgradable *Upgradable
	// 区块链类型
	chainType ChainType
	// 用于在错误信息中展示的serverID
	serverID uint8
}

// NewStratumSessionManager 创建Stratum会话管理器
func NewStratumSessionManager(conf ConfigData) (manager *StratumSessionManager, err error) {
	var chainType ChainType
	var indexBits uint8

	switch strings.ToLower(conf.ChainType) {
	case "bitcoin":
		chainType = ChainTypeBitcoin
		indexBits = 24
		break
	case "ethereum":
		chainType = ChainTypeEthereum
		indexBits = 16
		break
	default:
		err = errors.New("Unknown ChainType: " + conf.ChainType)
		return
	}

	manager = new(StratumSessionManager)

	manager.serverID = conf.ServerID
	manager.sessions = make(StratumSessionMap)
	manager.stratumServerInfoMap = conf.StratumServerMap
	manager.zookeeperSwitcherWatchDir = conf.ZKSwitcherWatchDir
	manager.enableUserAutoReg = conf.EnableUserAutoReg
	manager.zookeeperAutoRegWatchDir = conf.ZKAutoRegWatchDir
	manager.autoRegAllowUsers = conf.AutoRegMaxWaitUsers
	manager.tcpListenAddr = conf.ListenAddr
	manager.chainType = chainType

	manager.sessionIDManager, err = NewSessionIDManager(conf.ServerID, indexBits)
	if err != nil {
		return
	}
	manager.zookeeperManager, err = NewZookeeperManager(conf.ZKBroker)
	if err != nil {
		return
	}

	if manager.chainType == ChainTypeEthereum {
		// 由于SessionID是预分配的，为了与要求extraNonce不超过2字节的NiceHash以太坊客户端取得兼容，
		// 默认采用较大的ID分配间隔，以减少挖矿空间重叠的影响。
		manager.sessionIDManager.setAllocInterval(256)
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
	session.Resume(sessionData, serverConn)
}

// RegisterStratumSession 注册Stratum会话（在Stratum会话开始正常代理之后调用）
func (manager *StratumSessionManager) RegisterStratumSession(session *StratumSession) {
	manager.lock.Lock()
	manager.sessions[session.sessionID] = session
	manager.lock.Unlock()
}

// UnRegisterStratumSession 解除Stratum会话注册（在Stratum会话重连时调用）
func (manager *StratumSessionManager) UnRegisterStratumSession(session *StratumSession) {
	manager.lock.Lock()
	// 删除已注册的会话
	delete(manager.sessions, session.sessionID)
	manager.lock.Unlock()

	// 从Zookeeper管理器中删除币种监控
	manager.zookeeperManager.ReleaseW(session.zkWatchPath, session.sessionID)
}

// ReleaseStratumSession 释放Stratum会话（在Stratum会话停止时调用）
func (manager *StratumSessionManager) ReleaseStratumSession(session *StratumSession) {
	manager.lock.Lock()
	// 删除已注册的会话
	delete(manager.sessions, session.sessionID)
	manager.lock.Unlock()

	// 释放会话ID
	manager.sessionIDManager.FreeSessionID(session.sessionID)
	// 从Zookeeper管理器中删除币种监控
	manager.zookeeperManager.ReleaseW(session.zkWatchPath, session.sessionID)
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
		if runtimeData.TCPListenerFD != 0 {
			glog.Info("Resume TCP Listener: fd ", runtimeData.TCPListenerFD)
			manager.tcpListener, err = newListenerFromFd(runtimeData.TCPListenerFD)

			if err != nil {
				glog.Error("resume failed: ", err)
				manager.tcpListener = nil
			}
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
