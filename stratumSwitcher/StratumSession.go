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

// 纯代理模式下接收消息的超时时间
// 若长时间接收不到消息，就无法及时处理对端已断开事件，
// 因此设置接收超时时间，每隔一定时间就放弃接收，检查状态，并重新开始接收
const receiveMessageTimeoutSeconds = 15

// Zookeeper连接超时时间
const zookeeperConnTimeout = 5

// ProtocolType 代理的协议类型
type ProtocolType int

const (
	// ProtocolStratum Stratum协议
	ProtocolStratum = iota
	// ProtocolBTCAgent BTCAgent协议
	ProtocolBTCAgent = iota
	// ProtocolUnknown 未知协议（无法处理）
	ProtocolUnknown = iota
)

var (
	// ErrReadLineTimeout 从bufio.Reader中读取一行超时
	ErrReadLineTimeout = errors.New("Readline Timeout")
	// ErrSessionIDFull SessionID已满（所有可用值均已分配）
	ErrSessionIDFull = errors.New("Session ID is Full")
)

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
	minerName      string

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
	conn, _, err := zk.Connect(zkBrokers, time.Duration(zookeeperConnTimeout)*time.Second)

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
		return session, ErrSessionIDFull
	}

	session.sessionID = sessionID

	// uint32 to string
	bytesBuffer := bytes.NewBuffer([]byte{})
	binary.Write(bytesBuffer, binary.BigEndian, sessionID)
	session.sessionIDString = hex.EncodeToString(bytesBuffer.Bytes())

	glog.V(3).Info("Session ID: ", session.sessionIDString)

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

	protocolType, ok := session.protocolDetect()

	if !ok || protocolType == ProtocolUnknown {
		session.Stop()
		return
	}

	switch protocolType {
	case ProtocolStratum:
		session.runProxyStratum()
	case ProtocolBTCAgent:
		session.runProxyAgent()
	}
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

	glog.V(2).Info("Session Stoped: ", session.fullWorkerName)
}

func (session *StratumSession) protocolDetect() (ProtocolType, bool) {
	magicNumber, err := session.peekFromClientWithTimeout(1, protocolDetectTimeoutSeconds*time.Second)

	if err != nil {
		glog.Warning("read failed: ", err)
		session.Stop()
		return ProtocolUnknown, false
	}

	if magicNumber[0] == 0x7F {
		glog.V(3).Info("Found BTC Agent Protocol")
		return ProtocolBTCAgent, true

	} else if magicNumber[0] == '{' {
		glog.V(3).Info("Found Stratum Protocol")
		return ProtocolStratum, true

	} else {
		glog.Warning("Unknown Protocol")
		return ProtocolUnknown, true
	}
}

func (session *StratumSession) runProxyStratum() {
	var ok bool

	ok = session.stratumFindWorkerName()

	if !ok {
		session.Stop()
		return
	}

	ok = session.findMiningCoin()

	if !ok {
		session.Stop()
		return
	}

	ok = session.connectStratumServer()

	if !ok {
		session.Stop()
		return
	}

	// 此后转入纯代理模式
	session.proxyStratum()
}

func (session *StratumSession) stratumFindWorkerName() bool {
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
								// 截取“.”之前的做为子账户名，之后的做矿机名
								pos := strings.Index(session.fullWorkerName, ".")
								session.subaccountName = session.fullWorkerName[:pos]
								session.minerName = session.fullWorkerName[pos+1:]
							} else {
								session.subaccountName = session.fullWorkerName
								session.minerName = ""
							}

							if len(session.subaccountName) >= 1 {

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
			return false
		}

		glog.V(2).Info("FindWorkerName Success: ", session.fullWorkerName)
		return true

	case <-time.After(findWorkerNameTimeoutSeconds * time.Second):
		glog.Warning("FindWorkerName Timeout")
		return false
	}
}

func (session *StratumSession) findMiningCoin() bool {
	// 从zookeeper读取用户想挖的币种

	session.zkWatchPath = zookeeperSwitcherWatchDir + session.subaccountName
	data, _, event, err := zookeeperConn.GetW(session.zkWatchPath)

	if err != nil {
		glog.Error("FindMiningCoin Failed: " + session.zkWatchPath + "; " + err.Error())

		response := JSONRPCResponse{nil, nil, JSONRPCArray{201, "Cannot Found Minning Coin Type", nil}}
		session.writeJSONResponseToClient(&response)

		return false
	}

	session.miningCoin = string(data)
	session.zkWatchEvent = event

	return true
}

func (session *StratumSession) connectStratumServer() bool {
	// 寻找币种对应的服务器
	serverInfo, ok := stratumServerInfoMap[session.miningCoin]

	// 对应的服务器不存在
	if !ok {
		glog.Error("Stratum Server Not Found: ", session.miningCoin)

		response := JSONRPCResponse{nil, nil, JSONRPCArray{301, "Stratum Server Not Found: " + session.miningCoin, nil}}
		session.writeJSONResponseToClient(&response)
		return false
	}

	// 连接服务器
	serverConn, err := net.Dial("tcp", serverInfo.URL)

	if err != nil {
		glog.Error("Connect Stratum Server Failed: ", session.miningCoin, "; ", serverInfo.URL, "; ", err)

		response := JSONRPCResponse{nil, nil, JSONRPCArray{301, "Connect Stratum Server Failed: " + session.miningCoin, nil}}
		session.writeJSONResponseToClient(&response)
		return false
	}

	glog.V(3).Info("Connect Stratum Server Success: ", session.miningCoin, "; ", serverInfo.URL)

	session.serverConn = serverConn
	session.serverReader = bufio.NewReader(serverConn)
	session.serverWriter = bufio.NewWriter(serverConn)

	// 为请求添加sessionID
	// API格式：mining.subscribe("user agent/version", "extranonce1")
	// <https://en.bitcoin.it/wiki/Stratum_mining_protocol>

	// 获取原始的参数1（user agent）
	userAgent := "stratumSwitcher"
	if len(session.stratumSubscribeRequest.Params) >= 1 {
		userAgent, ok = session.stratumSubscribeRequest.Params[0].(string)
	}
	glog.V(3).Info("UserAgent: ", userAgent)

	session.stratumSubscribeRequest.SetParam(userAgent, session.sessionIDString)

	// 发送mining.subscribe请求给服务器
	// sessionID已包含在其中，一并发送给服务器
	_, err = session.writeJSONRequestToServer(session.stratumSubscribeRequest)

	if err != nil {
		glog.Warning("Write Subscribe Request Failed: ", err)
		return false
	}

	responseJSON, err := session.readLineFromServerWithTimeout(readSubscribeResponseTimeoutSeconds * time.Second)

	if err != nil {
		glog.Warning("Read Subscribe Response Failed: ", err)
		return false
	}

	// 检查服务器返回的 sessionID 与当前保存的是否一致
	response, err := NewJSONRPCResponse(responseJSON)

	if err != nil {
		glog.Warning("Parse Subscribe Response Failed: ", err)
		return false
	}

	result, ok := response.Result.([]interface{})

	if !ok {
		glog.Warning("Parse Subscribe Response Failed")
		return false
	}

	if len(result) < 2 {
		glog.Warning("Field too Few of Subscribe Response Result: ", err)
		return false
	}

	sessionID, ok := result[1].(string)

	if !ok {
		glog.Warning("Parse Subscribe Response Failed")
		return false
	}

	// 服务器返回的 sessionID 与当前保存的不一致，此时挖到的所有share都会是无效的，断开连接
	if sessionID != session.sessionIDString {
		glog.Warning("Session ID Inconformity:  ", sessionID, " != ", session.sessionIDString)
		return false
	}

	glog.V(3).Info("Subscribe Success: ", string(responseJSON))

	// 认证响应的JSON数据
	var authorizeResponseJSON []byte
	// 添加了币种后缀的矿机名
	fullWorkerNameWithCoinPostfix := session.subaccountName + "_" + session.miningCoin + "." + session.minerName

	// 认证状态
	var authSuccess = false
	// 最后一次尝试的矿机名
	var authWorkerName string

	// 在15秒内多次尝试认证
	// 之所以要多次认证，是因为第一次创建子账户的时候，Stratum Server不能及时的收到消息。
	// 新创建的子账户需要约10秒才能在Stratum Server可用。
	for i := 0; i < 5; i++ {
		// 首次认证尝试
		authWorkerName = fullWorkerNameWithCoinPostfix
		glog.V(3).Info("Authorize: ", authWorkerName)
		authSuccess, authorizeResponseJSON = session.miningAuthorize(authWorkerName)

		if authSuccess {
			break
		}

		// 认证没有成功，去掉币种后缀再试
		// 目前来说只有开启切换功能时新创建的子账户有币种后缀，之前的子账户并没有
		// 并且，即使重命名了子账户，没有重启过的stratum server也不会感知到子账户名已经改变
		glog.V(3).Info("Authorize failed with ", authWorkerName, ", try ", session.fullWorkerName)
		authWorkerName = session.fullWorkerName
		authSuccess, authorizeResponseJSON = session.miningAuthorize(authWorkerName)

		if authSuccess {
			break
		}

		// 还是没有成功，sleep 3秒
		time.Sleep(time.Duration(3) * time.Second)
	}

	// 若认证响应不为空，就转发给矿机，无论认证是否成功
	if authorizeResponseJSON != nil {
		_, err := session.clientWriter.Write(authorizeResponseJSON)

		if err != nil {
			glog.Warning("Write Authorize Response to Client Failed: ", err)
			return false
		}

		session.clientWriter.Flush()
	}

	// 返回认证的结果（若认证失败，则认为连接失败）

	if !authSuccess {
		glog.Warning("Authorize Failed: ", authWorkerName, "; ", session.miningCoin)
		return false
	}

	glog.Info("Authorize Success: ", authWorkerName, "; ", session.miningCoin)
	return true
}

// miningAuthorize 矿机认证
func (session *StratumSession) miningAuthorize(fullWorkerName string) (bool, []byte) {
	session.stratumAuthorizeRequest.Params[0] = fullWorkerName

	// 发送mining.authorize请求给服务器
	_, err := session.writeJSONRequestToServer(session.stratumAuthorizeRequest)

	if err != nil {
		glog.Warning("Write Authorize Request Failed: ", err)
		return false, nil
	}

	responseJSON, err := session.readLineFromServerWithTimeout(readSubscribeResponseTimeoutSeconds * time.Second)

	if err != nil {
		glog.Warning("Read Authorize Response Failed: ", err)
		return false, nil
	}

	response, err := NewJSONRPCResponse(responseJSON)

	if err != nil {
		glog.Warning("Parse Authorize Response Failed: ", err)
		return false, nil
	}

	success, ok := response.Result.(bool)

	if !ok || !success {
		return false, responseJSON
	}

	return true, responseJSON
}

func (session *StratumSession) proxyStratum() {
	var running = true

	// 从服务器到客户端
	go func() {
		for running {
			data, err := session.readLineFromServerWithTimeout(receiveMessageTimeoutSeconds * time.Second)

			if err == ErrReadLineTimeout {
				continue
			}

			if err != nil {
				// 判断是否进行了服务器切换
				if !running {
					// 不断开连接，直接退出函数
					glog.V(3).Info("Downstream: exited by switch coin")
					break
				}

				glog.V(3).Info("Read From Server Failed: ", err)
				session.Stop()
				break
			}

			_, err = session.clientWriter.Write(data)

			if err != nil {
				glog.V(3).Info("Write To Client Failed: ", err)
				session.Stop()
				break
			}

			session.clientWriter.Flush()
		}

		glog.V(3).Info("Downstream: exited")
	}()

	// 从客户端到服务器
	go func() {
		for running {
			data, err := session.readLineFromClientWithTimeout(receiveMessageTimeoutSeconds * time.Second)

			if err == ErrReadLineTimeout {
				continue
			}

			if err != nil {
				glog.V(3).Info("Read From Client Failed: ", err)
				session.Stop()
				break
			}

			_, err = session.serverWriter.Write(data)

			if err != nil {
				// 判断是否进行了服务器切换
				if !running {
					// 不断开连接，直接退出函数
					glog.V(3).Info("Upstream: exited by switch coin")
					break
				}

				glog.V(3).Info("Write To Server Failed: ", err)
				session.Stop()
				break
			}

			session.serverWriter.Flush()
		}

		glog.V(3).Info("Upstream: exited")
	}()

	// 监控来自zookeeper的切换指令并进行Stratum切换
	go func() {
		for session.IsRunning() {

			select {
			case <-session.zkWatchEvent:
				// 接收到数据，继续后续流程

			case <-time.After(receiveMessageTimeoutSeconds * time.Second):
				// 超时，继续下一轮循环（若session已停止，将在下一轮中退出）
				continue
			}

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
			glog.V(2).Info("Mining Coin Changed: ", session.fullWorkerName, ": ", session.miningCoin, " -> ", newMiningCoin)
			session.miningCoin = newMiningCoin

			// 设置运行标志
			running = false

			// 断开旧连接
			session.serverConn.Close()

			// 建立新连接
			ok := session.connectStratumServer()

			if !ok {
				session.Stop()
				break
			}

			// 转入代理模式
			session.proxyStratum()

			// 退出
			glog.V(3).Info("CoinWatcher: exited by switch coin")
			break
		}

		glog.V(3).Info("CoinWatcher: exited")
	}()
}

func (session *StratumSession) runProxyAgent() {
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
		return nil, ErrReadLineTimeout
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
