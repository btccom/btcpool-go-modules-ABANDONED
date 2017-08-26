package main

import (
	"bufio"
	"bytes"
	"encoding/binary"
	"encoding/hex"
	"errors"
	"net"
	"strings"
	"time"

	"github.com/golang/glog"
)

// StratumSession 是一个 Stratum 会话，包含了到客户端和到服务端的连接及状态信息
type StratumSession struct {
	clientConn   net.Conn
	clientReader *bufio.Reader
	clientWriter *bufio.Writer

	// sessionID 会话ID，也做为矿机挖矿时的 Extranonce1
	sessionID       uint32
	sessionIDString string

	fullWorkerName string
	subaccountName string

	stratumSubscribeRequest *JSONRPCRequest
	stratumAuthorizeRequest *JSONRPCRequest
}

// sessionIDManager 会话ID管理器实例
var sessionIDManager *SessionIDManager

// StratumSessionGlobalInit StratumSession功能的全局初始化
// 需要在使用StratumSession功能之前调用且仅调用一次
func StratumSessionGlobalInit(serverID uint8) {
	sessionIDManager = NewSessionIDManager(serverID)
}

// NewStratumSession 创建一个新的 Stratum 会话
func NewStratumSession(clientConn net.Conn) (*StratumSession, error) {
	session := new(StratumSession)

	session.clientConn = clientConn
	session.clientReader = bufio.NewReader(clientConn)
	session.clientWriter = bufio.NewWriter(clientConn)

	// 产生 sessionID （Extranonce1）
	sessionID, success := sessionIDManager.AllocSessionID()

	if !success {
		return session, errors.New("Session ID is Full")
	}

	session.sessionID = sessionID

	// uint32 to string
	bytesBuffer := bytes.NewBuffer([]byte{})
	binary.Write(bytesBuffer, binary.BigEndian, sessionID)
	session.sessionIDString = hex.EncodeToString(bytesBuffer.Bytes())

	glog.Info("Session ID: ", sessionID)

	return session, nil
}

// Run 启动一个 Stratum 会话
func (session *StratumSession) Run() {
	session.protocolDetect()
}

// Stop 停止一个 Stratum 会话
func (session *StratumSession) Stop() {
	session.clientWriter.Flush()
	session.clientConn.Close()

	// 释放sessionID
	sessionIDManager.FreeSessionID(session.sessionID)
}

func (session *StratumSession) protocolDetect() {
	magicNumber, err := session.peekWithTimeout(1, 30*time.Second)

	if err != nil {
		glog.Warning("read failed: ", err)
		session.Stop()
		return
	}

	if magicNumber[0] == 0x7F {
		glog.Info("Found BTC Agent Protocol")
		session.agentFindWorkerName()

	} else if magicNumber[0] == '{' {
		glog.Info("Found Stratum Protocol")
		session.stratumFindWorkerName()

	} else {
		glog.Warning("Unknown Protocol")
		session.Stop()
	}
}

func (session *StratumSession) stratumFindWorkerName() {
	e := make(chan error)

	go func() {
		isSubscribed := false
		response := new(JSONRPCResponse)

		for true {
			requestJSON, err := session.clientReader.ReadBytes('\n')

			if err != nil {
				e <- errors.New("read line failed: " + err.Error())
				break
			}

			request, err := NewJSONRPCRequest(requestJSON)

			// ignore the json decode error
			if err != nil {
				glog.Warning("JSON decode failed: ", err.Error(), string(requestJSON))
				continue
			}

			response.ID = request.ID
			response.Result = nil
			response.Error = nil

			if request.Method == "mining.subscribe" {
				if isSubscribed {
					response.Result = nil
					response.Error = JSONRPCArray{24, "Duplicate Subscribed", nil}
				} else {
					isSubscribed = true
					// 保存原始请求以便转发给Stratum服务器
					session.stratumSubscribeRequest = request

					response.Result = JSONRPCArray{JSONRPCArray{JSONRPCArray{"mining.set_difficulty", session.sessionIDString}, JSONRPCArray{"mining.notify", session.sessionIDString}}, session.sessionIDString, 8}
					response.Error = nil
				}
			} else if request.Method == "mining.authorize" {
				if isSubscribed {
					if len(request.Params) >= 1 {
						fullWorkerName, ok := request.Params[0].(string)

						if ok {
							// 矿工名
							session.fullWorkerName = strings.TrimSpace(fullWorkerName)

							if strings.Contains(session.fullWorkerName, ".") {
								// 截取“.”之前的做为子账户名
								session.subaccountName = session.fullWorkerName[0:strings.Index(session.fullWorkerName, ".")]
							} else {
								session.subaccountName = session.fullWorkerName
							}

							if len(session.subaccountName) >= 1 {

								glog.Info(session.fullWorkerName, " ", session.subaccountName)

								// 保存原始请求以便转发给Stratum服务器
								session.stratumAuthorizeRequest = request

								// 发送一个空错误到channel，表示成功
								e <- nil
								// 跳出循环，不发送响应给矿机，该响应直接由Stratum服务器发送
								break

							} else {
								response.Result = nil
								response.Error = JSONRPCArray{23, "Worker Name Cannot Start with '.'", nil}
							}

						} else {
							response.Result = nil
							response.Error = JSONRPCArray{23, "Worker Name is Not a String", nil}
						}

					} else {
						response.Result = nil
						response.Error = JSONRPCArray{22, "Too Few Params", nil}
					}

				} else {
					response.Result = nil
					response.Error = JSONRPCArray{25, "Not Subscribed", nil}
				}
			} else {
				response.Result = nil
				response.Error = JSONRPCArray{20, "Unknown Method", nil}
			}

			session.writeJSONResponse(response)
		}

		close(e)
	}()

	select {
	case err := <-e:
		if err != nil {
			glog.Warning(err)
			session.Stop()
		} else {
			glog.Info("FindWorkerName Success: ", session.fullWorkerName)
			session.connectStratumServer()
		}
	case <-time.After(90 * time.Second):
		glog.Warning("FindWorkerName Timeout")
		session.Stop()
	}
}

func (session *StratumSession) connectStratumServer() {

}

func (session *StratumSession) agentFindWorkerName() {
	glog.Error("proxy of BTC Agent Protocol is not implement now!")
	session.Stop()
}

func (session *StratumSession) peekWithTimeout(len int, timeout time.Duration) ([]byte, error) {
	d := make(chan []byte)
	e := make(chan error)

	go func() {
		data, err := session.clientReader.Peek(len)
		if err != nil {
			e <- err
		} else {
			d <- data
		}
		close(d)
		close(e)
	}()

	select {
	case data := <-d:
		return data, nil
	case err := <-e:
		return nil, err
	case <-time.After(timeout):
		return nil, errors.New("Peek Timeout")
	}
}

func (session *StratumSession) readLineWithTimeout(timeout time.Duration) ([]byte, error) {
	d := make(chan []byte)
	e := make(chan error)

	go func() {
		data, err := session.clientReader.ReadBytes('\n')
		if err != nil {
			e <- err
		} else {
			d <- data
		}
		close(d)
		close(e)
	}()

	select {
	case data := <-d:
		return data, nil
	case err := <-e:
		return nil, err
	case <-time.After(timeout):
		return nil, errors.New("ReadLine Timeout")
	}
}

func (session *StratumSession) writeJSONResponse(jsonData *JSONRPCResponse) (int, error) {
	bytes, err := jsonData.ToJSONBytes()

	if err != nil {
		return 0, err
	}

	defer session.clientWriter.Flush()
	defer session.clientWriter.WriteByte('\n')
	return session.clientWriter.Write(bytes)
}
