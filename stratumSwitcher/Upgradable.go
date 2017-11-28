package main

import (
	"os"

	"github.com/golang/glog"
)

// 保存运行时状态文件的变量
const runtimeFilePath = "./runtime.json"

// Upgradable 不停机升级StratumSwitcher进程
type Upgradable struct {
	sessionManager *StratumSessionManager
}

// NewUpgradable 创建Upgradable对象
func NewUpgradable(sessionManager *StratumSessionManager) (upgradable *Upgradable) {
	upgradable = new(Upgradable)
	upgradable.sessionManager = sessionManager
	return
}

// 升级StratumSwitcher进程
func (upgradable *Upgradable) upgradeStratumSwitcher() (err error) {
	var runtimeData RuntimeData

	runtimeData.Action = "upgrade"

	runtimeData.TCPListenerFD, err = getListenerFd(upgradable.sessionManager.tcpListener)
	if err != nil {
		return
	}
	setNoCloseOnExec(runtimeData.TCPListenerFD)

	upgradable.sessionManager.lock.Lock()
	for _, session := range upgradable.sessionManager.sessions {
		var sessionData StratumSessionData

		sessionData.SessionID = session.sessionID
		sessionData.MiningCoin = session.miningCoin
		sessionData.StratumSubscribeRequest = session.stratumSubscribeRequest
		sessionData.StratumAuthorizeRequest = session.stratumAuthorizeRequest
		sessionData.StratumAuthorizeRequest.Params[0] = session.fullWorkerName

		sessionData.ClientConnFD, err = getConnFd(session.clientConn)
		if err != nil {
			glog.Error("getConnFd Failed: ", err)
			continue
		}

		sessionData.ServerConnFD, err = getConnFd(session.serverConn)
		if err != nil {
			glog.Error("getConnFd Failed: ", err)
			continue
		}

		setNoCloseOnExec(sessionData.ClientConnFD)
		setNoCloseOnExec(sessionData.ServerConnFD)

		runtimeData.SessionDatas = append(runtimeData.SessionDatas, sessionData)
	}
	upgradable.sessionManager.lock.Unlock()

	err = runtimeData.SaveToFile(runtimeFilePath)
	if err != nil {
		return
	}

	upgradable.sessionManager.zookeeperConn.Close()

	var args []string
	for _, arg := range os.Args[1:] {
		if len(arg) < 9 || arg[0:9] != "-runtime=" {
			args = append(args, arg)
		}
	}
	args = append(args, "-runtime="+runtimeFilePath)

	err = execNewBin(os.Args[0], args)
	return
}
