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
	"github.com/samuel/go-zookeeper/zk"
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
	// 监控的Zookeeper路径
	zkWatchPath string
	// 监控的Zookeeper事件
	zkWatchEvent <-chan zk.Event
}

// StratumServerInfo Stratum服务器的信息
type StratumServerInfo struct {
	URL string
}

// StratumServerInfoMap Stratum服务器的信息散列表
type StratumServerInfoMap map[string]StratumServerInfo

// sessionIDManager 会话ID管理器实例
var sessionIDManager *SessionIDManager

// StratumServerInfoMap Stratum服务器信息列表
var stratumServerInfoMap StratumServerInfoMap

// zookeeperConn Zookeeper连接对象
var zookeeperConn *zk.Conn

// zookeeperSwitcherWatchDir 切换服务监控的zookeeper目录路径
// 具体监控的路径为 zookeeperSwitcherWatchDir/子账户名
var zookeeperSwitcherWatchDir string

// StratumSessionGlobalInit StratumSession功能的全局初始化
// 需要在使用StratumSession功能之前调用且仅调用一次
func StratumSessionGlobalInit(serverID uint8, serverMap StratumServerInfoMap, zkBrokers []string, zkSwitcherWatchDir string) error {
	sessionIDManager = NewSessionIDManager(serverID)
	stratumServerInfoMap = serverMap
	zookeeperSwitcherWatchDir = zkSwitcherWatchDir

	// 建立到Zookeeper集群的连接
	conn, _, err := zk.Connect(zkBrokers, time.Second)

	if err != nil {
		return errors.New("Connect Zookeeper Failed: " + err.Error())
	}

	zookeeperConn = conn
	return nil
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

	glog.Info("Session ID: ", session.sessionIDString)

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

					// 为请求添加sessionID
					request.SetParam(session.sessionIDString)
					data, _ := request.ToJSONBytes()
					glog.Info(string(data))
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
	// 从zookeeper读取用户想挖的币种

	session.zkWatchPath = zookeeperSwitcherWatchDir + session.subaccountName
	data, _, event, err := zookeeperConn.GetW(session.zkWatchPath)

	if err != nil {
		glog.Error("FindMiningCoin Failed: " + session.zkWatchPath + "; " + err.Error())

		response := JSONRPCResponse{nil, nil, JSONRPCArray{201, "Cannot Found Minning Coin Type", nil}}
		session.writeJSONResponseToClient(&response)

		session.Stop()
		return
	}

	session.miningCoin = string(data)
	session.zkWatchEvent = event

	session.connectStratumServer()
}

func (session *StratumSession) connectStratumServer() {
	serverInfo, ok := stratumServerInfoMap[session.miningCoin]

	if !ok {
		glog.Error("Stratum Server Not Found: ", session.miningCoin)

		response := JSONRPCResponse{nil, nil, JSONRPCArray{301, "Stratum Server Not Found: " + session.miningCoin, nil}}
		session.writeJSONResponseToClient(&response)
		session.Stop()
		return
	}

	serverConn, err := net.Dial("tcp", serverInfo.URL)

	if err != nil {
		glog.Error("Connect Stratum Server Failed: ", session.miningCoin, "; ", serverInfo.URL, "; ", err)

		response := JSONRPCResponse{nil, nil, JSONRPCArray{301, "Connect Stratum Server Failed: " + session.miningCoin, nil}}
		session.writeJSONResponseToClient(&response)
		session.Stop()
		return
	}

	glog.Info("Connect Stratum Server Success: ", session.miningCoin, "; ", serverInfo.URL)

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
	var running = true

	// 从服务器到客户端
	go func() {
		for running {
			data, err := session.serverReader.ReadBytes('\n')

			if err != nil {
				// 判断是否进行了服务器切换
				if !running {
					// 不断开连接，直接退出函数
					glog.Info("Downstream: exited by switch coin")
					break
				}

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

		glog.Info("Downstream: exited")
	}()

	// 从客户端到服务器
	go func() {
		for running {
			data, err := session.clientReader.ReadBytes('\n')

			if err != nil {
				glog.Warning("Read From Client Failed: ", err)
				session.Stop()
				break
			}

			_, err = session.serverWriter.Write(data)

			if err != nil {
				// 判断是否进行了服务器切换
				if !running {
					// 不断开连接，直接退出函数
					glog.Info("Upstream: exited by switch coin")
					break
				}

				glog.Warning("Read From Client Failed: ", err)
				session.Stop()
				break
			}

			session.serverWriter.Flush()
		}

		glog.Info("Upstream: exited")
	}()

	// 监控来自zookeeper的切换指令并进行Stratum切换
	go func() {
		for session.IsRunning() {
			<-session.zkWatchEvent

			if !session.IsRunning() {
				break
			}

			data, _, event, err := zookeeperConn.GetW(session.zkWatchPath)
			session.zkWatchEvent = event

			if err != nil {
				glog.Error("Read From Zookeeper Failed: ", session.zkWatchPath, "; ", err)

				// 忽略GetW的错误并尝试继续监控
				_, _, existEvent, err := zookeeperConn.ExistsW(session.zkWatchPath)

				// 还是失败，放弃监控并结束会话
				if err != nil {
					glog.Error("Watch From Zookeeper Failed: ", session.zkWatchPath, "; ", err)
					session.Stop()
					break
				}

				session.zkWatchEvent = existEvent
				continue
			}

			newMiningCoin := string(data)

			// 若币种未改变，则继续监控
			if newMiningCoin == session.miningCoin {
				continue
			}

			// 若币种对应的Stratum服务器不存在，则忽略事件并继续监控
			_, exists := stratumServerInfoMap[newMiningCoin]
			if !exists {
				glog.Error("Stratum Server Not Found for New Mining Coin: ", newMiningCoin)
				continue
			}

			// 开始切换币种
			glog.Info("Mining Coin Changed: ", session.fullWorkerName, ": ", session.miningCoin, " -> ", newMiningCoin)
			session.miningCoin = newMiningCoin

			// 设置运行标志
			running = false

			// 断开旧连接
			session.serverConn.Close()

			// 建立新连接
			go session.connectStratumServer()

			// 退出
			glog.Info("CoinWatcher: exited by switch coin")
			break
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
