package main

import (
	"bufio"
	"errors"
	"net"

	"github.com/golang/glog"
)

// StratumSession 是一个 Stratum 会话，包含了到客户端和到服务端的连接及状态信息
type StratumSession struct {
	clientConn   net.Conn
	clientReader *bufio.Reader
	clientWriter *bufio.Writer

	// sessionID 会话ID，也做为矿机挖矿时的 Extranonce1
	sessionID uint32
}

// sessionIDManager 会话ID管理器实例
// TODO: 将serverID转移到配置文件
var sessionIDManager = NewSessionIDManager(1)

// NewStratumSession 创建一个新的 Stratum 会话
func NewStratumSession(clientConn net.Conn) (StratumSession, error) {
	var session StratumSession

	session.clientConn = clientConn
	session.clientReader = bufio.NewReader(clientConn)
	session.clientWriter = bufio.NewWriter(clientConn)

	// 产生 sessionID （Extranonce1）
	sessionID, success := sessionIDManager.AllocSessionID()

	if !success {
		return session, errors.New("Session ID is Full")
	}

	session.sessionID = sessionID
	glog.Info("Session ID: ", sessionID)

	return session, nil
}

// Run 启动一个 Stratum 会话
func (session StratumSession) Run() {
	session.protocolDetect()
}

// Stop 停止一个 Stratum 会话
func (session StratumSession) Stop() {
	session.clientWriter.Flush()
	session.clientConn.Close()

	// 释放sessionID
	sessionIDManager.FreeSessionID(session.sessionID)
}

func (session StratumSession) protocolDetect() {
	magicNumber, err := session.clientReader.Peek(1)

	if err != nil {
		glog.Error("read failed: ", err)
		return
	}

	if magicNumber[0] == 0x7F {
		glog.Info("Found BTC Agent Protocol")
		session.agentFindWorkerName()

	} else if magicNumber[0] == '{' {
		glog.Info("Found Stratum Protocol")
		session.stratumFindWorkerName()

	} else {
		glog.Info("Unknown Protocol")
		session.Stop()
	}
}

func (session StratumSession) stratumFindWorkerName() {
	rpcJSON, err := session.clientReader.ReadBytes('\n')

	if err != nil {
		glog.Error("read line failed: ", err)
		session.Stop()
		return
	}

	rpcData, err := NewJSONRPCData(rpcJSON)

	if err != nil {
		glog.Error("JSON decode failed: ", err, rpcJSON)
		session.Stop()
		return
	}

	glog.Info(rpcData.Method)
	jsonBytes, _ := rpcData.ToJSONBytes()
	glog.Info(string(jsonBytes))
}

func (session StratumSession) agentFindWorkerName() {
	glog.Error("proxy of BTC Agent Protocol is not implement now!")
	session.Stop()
}
