package main

import (
	"bufio"
	"bytes"
	"encoding/binary"
	"encoding/hex"
	"errors"
	"net"
	"strings"
	"sync"
	"time"

	"github.com/golang/glog"
)

// 协议检测超时时间
const protocolDetectTimeoutSeconds = 15

// 矿工名获取超时时间
const findWorkerNameTimeoutSeconds = 60

// 服务器响应subscribe消息的超时时间
const readSubscribeResponseTimeoutSeconds = 10

// StratumSession 是一个 Stratum 会话，包含了到客户端和到服务端的连接及状态信息
type StratumSession struct {
	// 是否在运行
	isRunning bool
	// 改变运行状态时进行加锁
	lock sync.Mutex

	clientConn   net.Conn
	clientReader *bufio.Reader
	clientWriter *bufio.Writer

	serverConn   net.Conn
	serverReader *bufio.Reader
	serverWriter *bufio.Writer

	// sessionID 会话ID，也做为矿机挖矿时的 Extranonce1
	sessionID       uint32
	sessionIDString string

	fullWorkerName string
	subaccountName string

	stratumSubscribeRequest *JSONRPCRequest
	stratumAuthorizeRequest *JSONRPCRequest

	// 用户所挖的币种
	miningCoin string
}

// StratumServerInfo Stratum服务器的信息
type StratumServerInfo struct {
	URL string
}

// StratumServerInfoMap Stratum服务器的信息散列表
type StratumServerInfoMap map[string]StratumServerInfo

// sessionIDManager 会话ID管理器实例
var sessionIDManager *SessionIDManager

var stratumServerInfoMap StratumServerInfoMap

// StratumSessionGlobalInit StratumSession功能的全局初始化
// 需要在使用StratumSession功能之前调用且仅调用一次
func StratumSessionGlobalInit(serverID uint8, serverMap StratumServerInfoMap) {
	sessionIDManager = NewSessionIDManager(serverID)
	stratumServerInfoMap = serverMap
}

// NewStratumSession 创建一个新的 Stratum 会话
func NewStratumSession(clientConn net.Conn) (*StratumSession, error) {
	session := new(StratumSession)

	session.isRunning = false

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

// IsRunning 检查会话是否在运行（线程安全）
func (session *StratumSession) IsRunning() bool {
	defer session.lock.Unlock()
	session.lock.Lock()

	return session.isRunning
}

// Run 启动一个 Stratum 会话
func (session *StratumSession) Run() {
	session.lock.Lock()
	session.isRunning = true
	session.lock.Unlock()

	session.protocolDetect()
}

// Stop 停止一个 Stratum 会话
func (session *StratumSession) Stop() {
	session.lock.Lock()

	if !session.isRunning {
		defer session.lock.Unlock()
		return
	}

	session.isRunning = false
	session.lock.Unlock()

	if session.serverWriter != nil {
		session.serverWriter.Flush()
	}

	if session.serverConn != nil {
		session.serverConn.Close()
	}

	if session.clientWriter != nil {
		session.clientWriter.Flush()
	}

	if session.clientConn != nil {
		session.clientConn.Close()
	}

	// 释放sessionID
	sessionIDManager.FreeSessionID(session.sessionID)
}

func (session *StratumSession) protocolDetect() {
	magicNumber, err := session.peekFromClientWithTimeout(1, protocolDetectTimeoutSeconds*time.Second)

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
								response.Error = JSONRPCArray{105, "Worker Name Cannot Start with '.'", nil}
							}

						} else {
							response.Result = nil
							response.Error = JSONRPCArray{104, "Worker Name is Not a String", nil}
						}

					} else {
						response.Result = nil
						response.Error = JSONRPCArray{103, "Too Few Params", nil}
					}

				} else {
					response.Result = nil
					response.Error = JSONRPCArray{102, "Not Subscribed", nil}
				}
			} else {
				response.Result = nil
				response.Error = JSONRPCArray{101, "Unknown Method", nil}
			}

			_, err = session.writeJSONResponseToClient(response)

			if err != nil {
				e <- errors.New("Write JSON Response Failed: " + err.Error())
				break
			}
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

			// 找到用户想挖的币种
			session.findMiningCoin()
		}
	case <-time.After(findWorkerNameTimeoutSeconds * time.Second):
		glog.Warning("FindWorkerName Timeout")
		session.Stop()
	}
}

func (session *StratumSession) findMiningCoin() {
	// TODO: 从zookeeper读取用户想挖的币种

	session.miningCoin = "btc"

	session.connectStratumServer()
}

func (session *StratumSession) connectStratumServer() {
	serverInfo := stratumServerInfoMap[session.miningCoin]
	glog.Info("ServerInfo: ", serverInfo)

	serverConn, err := net.Dial("tcp", serverInfo.URL)

	if err != nil {
		glog.Error("Connect Stratum Server Failed: ", err)

		response := JSONRPCResponse{nil, nil, JSONRPCArray{201, "Connect Stratum Server Failed", nil}}
		session.writeJSONResponseToClient(&response)
		session.Stop()
		return
	}

	glog.Info("Connect Stratum Server Success")

	session.serverConn = serverConn
	session.serverReader = bufio.NewReader(serverConn)
	session.serverWriter = bufio.NewWriter(serverConn)

	// 发送mining.subscribe请求给服务器
	// TODO: 一并发送 sessionID 给服务器
	_, err = session.writeJSONRequestToServer(session.stratumSubscribeRequest)

	if err != nil {
		glog.Warning("Write Subscribe Request Failed: ", err)
		session.Stop()
		return
	}

	// TODO: 检查服务器返回的 sessionID 与当前保存的是否一致
	requestJSON, err := session.readLineFromServerWithTimeout(readSubscribeResponseTimeoutSeconds * time.Second)

	if err != nil {
		glog.Warning("Write JSON Request Failed: ", err)
		session.Stop()
		return
	}

	glog.Info("Subscribe Success: ", string(requestJSON))

	// 发送mining.authorize请求给服务器
	_, err = session.writeJSONRequestToServer(session.stratumAuthorizeRequest)

	if err != nil {
		glog.Warning("Write Authorize Request Failed: ", err)
		session.Stop()
		return
	}

	// TODO：添加币种后缀，检查mining.authorize是否成功，若不成功，则去掉币种后缀重试

	// 此后转入纯代理模式
	session.proxyStratum()
}

func (session *StratumSession) proxyStratum() {
	// 从服务器到客户端
	go func() {
		for true {
			data, err := session.serverReader.ReadBytes('\n')

			if err != nil {
				glog.Warning("Read From Server Failed: ", err)
				session.Stop()
				break
			}

			_, err = session.clientWriter.Write(data)

			if err != nil {
				glog.Warning("Read From Server Failed: ", err)
				session.Stop()
				break
			}

			session.clientWriter.Flush()
		}
	}()

	// 从客户端到服务器
	go func() {
		for true {
			data, err := session.clientReader.ReadBytes('\n')

			if err != nil {
				glog.Warning("Read From Client Failed: ", err)
				session.Stop()
				break
			}

			_, err = session.serverWriter.Write(data)

			if err != nil {
				glog.Warning("Read From Client Failed: ", err)
				session.Stop()
				break
			}

			session.serverWriter.Flush()
		}
	}()
}

func (session *StratumSession) agentFindWorkerName() {
	glog.Error("proxy of BTC Agent Protocol is not implement now!")
	session.Stop()
}

func peekWithTimeout(reader *bufio.Reader, len int, timeout time.Duration) ([]byte, error) {
	d := make(chan []byte)
	e := make(chan error)

	go func() {
		data, err := reader.Peek(len)
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

func (session *StratumSession) peekFromClientWithTimeout(len int, timeout time.Duration) ([]byte, error) {
	return peekWithTimeout(session.clientReader, len, timeout)
}

func (session *StratumSession) peekFromServerWithTimeout(len int, timeout time.Duration) ([]byte, error) {
	return peekWithTimeout(session.serverReader, len, timeout)
}

func readLineWithTimeout(reader *bufio.Reader, timeout time.Duration) ([]byte, error) {
	d := make(chan []byte)
	e := make(chan error)

	go func() {
		data, err := reader.ReadBytes('\n')
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

func (session *StratumSession) readLineFromClientWithTimeout(timeout time.Duration) ([]byte, error) {
	return readLineWithTimeout(session.clientReader, timeout)
}

func (session *StratumSession) readLineFromServerWithTimeout(timeout time.Duration) ([]byte, error) {
	return readLineWithTimeout(session.serverReader, timeout)
}

func (session *StratumSession) writeJSONResponseToClient(jsonData *JSONRPCResponse) (int, error) {
	bytes, err := jsonData.ToJSONBytes()

	if err != nil {
		return 0, err
	}

	defer session.clientWriter.Flush()
	defer session.clientWriter.WriteByte('\n')
	return session.clientWriter.Write(bytes)
}

func (session *StratumSession) writeJSONRequestToServer(jsonData *JSONRPCRequest) (int, error) {
	bytes, err := jsonData.ToJSONBytes()

	if err != nil {
		return 0, err
	}

	defer session.serverWriter.Flush()
	defer session.serverWriter.WriteByte('\n')
	return session.serverWriter.Write(bytes)
}
