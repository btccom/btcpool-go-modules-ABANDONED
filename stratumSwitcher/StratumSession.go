package main

import (
	"bufio"
	"errors"
	"net"
	"strings"
	"sync"
	"time"

	"github.com/golang/glog"
	"github.com/samuel/go-zookeeper/zk"
)

// BTCAgent的客户端类型前缀
const btcAgentClientTypePrefix = "btccom-agent/"

// NiceHash Ethereum Stratum Protocol 的协议类型前缀
const ethereumStratumNiceHashPrefix = "ethereumstratum/"

// 响应中使用的 NiceHash Ethereum Stratum Protocol 的版本
const ethereumStratumNiceHashVersion = "EthereumStratum/1.0.0"

// 发送给sserver的ETHProxy协议版本字符串
const ethproxyVersion = "ETHProxy/1.0.0"

// BTCAgent的ex-message的magic number
const btcAgentExMessageMagicNumber = 0x7F

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

// 服务器断开连接时的重试次数
const retryTimeWhenServerDown = 10

// 创建的 bufio Reader 的 buffer 大小
const bufioReaderBufSize = 128

// ProtocolType 代理的协议类型
type ProtocolType uint8

const (
	// ProtocolBitcoinStratum 比特币Stratum协议
	ProtocolBitcoinStratum ProtocolType = iota
	// ProtocolEthereumStratum 以太坊普通Stratum协议
	ProtocolEthereumStratum
	// ProtocolEthereumStratumNiceHash NiceHash建议的以太坊Stratum协议
	ProtocolEthereumStratumNiceHash
	// ProtocolEthereumProxy EthProxy软件实现的以太坊Stratum协议
	ProtocolEthereumProxy
	// ProtocolUnknown 未知协议（无法处理）
	ProtocolUnknown
)

// RunningStat 运行状态
type RunningStat uint8

const (
	// StatRunning 正在运行
	StatRunning RunningStat = iota
	// StatStoped 已停止
	StatStoped RunningStat = iota
	// StatReconnecting 正在重连服务器
	StatReconnecting RunningStat = iota
)

// AuthorizeStat 认证状态
type AuthorizeStat uint8

const (
	// StatConnected 已连接（默认状态）
	StatConnected AuthorizeStat = iota
	// StatSubScribed 已订阅
	StatSubScribed
	// StatAuthorized 已认证
	StatAuthorized
)

// StratumSession 是一个 Stratum 会话，包含了到客户端和到服务端的连接及状态信息
type StratumSession struct {
	// 会话管理器
	manager *StratumSessionManager

	// Stratum协议类型
	protocolType ProtocolType
	// 是否为BTCAgent
	isBTCAgent bool

	// 是否在运行
	runningStat RunningStat
	// 服务器重连计数器
	reconnectCounter uint32
	// 改变runningStat和switchCoinCount时要加的锁
	lock sync.Mutex

	clientConn   net.Conn
	clientReader *bufio.Reader

	// 客户端IP地址及端口
	clientIPPort string

	serverConn   net.Conn
	serverReader *bufio.Reader

	// sessionID 会话ID，也做为矿机挖矿时的 Extranonce1
	sessionID       uint32
	sessionIDString string

	fullWorkerName   string // 完整的矿工名
	subaccountName   string // 子账户名部分
	minerNameWithDot string // 矿机名部分（包含前导“.”）

	stratumSubscribeRequest *JSONRPCRequest
	stratumAuthorizeRequest *JSONRPCRequest

	// 用户所挖的币种
	miningCoin string
	// 监控的Zookeeper路径
	zkWatchPath string
	// 监控的Zookeeper事件
	zkWatchEvent <-chan zk.Event
}

// NewStratumSession 创建一个新的 Stratum 会话
func NewStratumSession(manager *StratumSessionManager, clientConn net.Conn, sessionID uint32) (session *StratumSession) {
	session = new(StratumSession)

	session.runningStat = StatStoped
	session.manager = manager
	session.sessionID = sessionID

	session.clientConn = clientConn
	session.clientReader = bufio.NewReaderSize(clientConn, bufioReaderBufSize)

	session.clientIPPort = clientConn.RemoteAddr().String()
	session.sessionIDString = Uint32ToHex(session.sessionID)

	if manager.chainType == ChainTypeEthereum {
		// Ethereum uses 24 bit session id
		session.sessionIDString = session.sessionIDString[2:8]
	}

	if glog.V(3) {
		glog.Info("IP: ", session.clientIPPort, ", Session ID: ", session.sessionIDString)
	}
	return
}

// IsRunning 检查会话是否在运行（线程安全）
func (session *StratumSession) IsRunning() bool {
	session.lock.Lock()
	defer session.lock.Unlock()

	return session.runningStat != StatStoped
}

// setStat 设置会话状态（线程安全）
func (session *StratumSession) setStat(stat RunningStat) {
	session.lock.Lock()
	session.runningStat = stat
	session.lock.Unlock()
}

// setStatNonLock 设置会话状态（
func (session *StratumSession) setStatNonLock(stat RunningStat) {
	session.runningStat = stat
}

// getStat 获取会话状态（线程安全）
func (session *StratumSession) getStat() RunningStat {
	session.lock.Lock()
	defer session.lock.Unlock()

	return session.runningStat
}

// getStatNonLock 获取会话状态（无锁，非线程安全，用于在已加锁函数内部调用）
func (session *StratumSession) getStatNonLock() RunningStat {
	return session.runningStat
}

// getReconnectCounter 获取币种切换计数（线程安全）
func (session *StratumSession) getReconnectCounter() uint32 {
	session.lock.Lock()
	defer session.lock.Unlock()

	return session.reconnectCounter
}

// Run 启动一个 Stratum 会话
func (session *StratumSession) Run() {
	session.lock.Lock()

	if session.runningStat != StatStoped {
		session.lock.Unlock()
		return
	}

	session.runningStat = StatRunning
	session.lock.Unlock()

	session.protocolType = session.protocolDetect()

	// 其实目前只有一种协议，即Stratum协议
	// BTCAgent在认证完成之前走的也是Stratum协议
	if session.protocolType == ProtocolUnknown {
		session.Stop()
		return
	}

	session.runProxyStratum()
}

// Resume 恢复一个Stratum会话
func (session *StratumSession) Resume(sessionData StratumSessionData, serverConn net.Conn) {
	session.lock.Lock()

	if session.runningStat != StatStoped {
		session.lock.Unlock()
		return
	}

	session.runningStat = StatRunning
	session.lock.Unlock()

	// 设置默认协议
	session.protocolType = session.getDefaultStratumProtocol()

	// 恢复服务器连接
	session.serverConn = serverConn
	session.serverReader = bufio.NewReaderSize(serverConn, bufioReaderBufSize)

	stat := StatConnected

	if sessionData.StratumSubscribeRequest != nil {
		_, stratumErr := session.stratumHandleRequest(sessionData.StratumSubscribeRequest, &stat)
		if stratumErr != nil {
			glog.Error("Resume session ", session.clientIPPort, " failed: ", stratumErr)
			session.Stop()
			return
		}
	}

	if sessionData.StratumAuthorizeRequest != nil {
		_, stratumErr := session.stratumHandleRequest(sessionData.StratumAuthorizeRequest, &stat)
		if stratumErr != nil {
			glog.Error("Resume session ", session.clientIPPort, " failed: ", stratumErr)
			session.Stop()
			return
		}
	}

	if stat != StatAuthorized {
		glog.Error("Resume session ", session.clientIPPort, " failed: stat should be StatAuthorized, but is ", stat)
		session.Stop()
		return
	}

	err := session.findMiningCoin()
	if err != nil {
		glog.Error("Resume session ", session.clientIPPort, " failed: ", err)
		session.Stop()
		return
	}

	if session.miningCoin != sessionData.MiningCoin {
		glog.Error("Resume session ", session.clientIPPort, " failed: mining coin changed: ",
			sessionData.MiningCoin, " -> ", session.miningCoin)
		session.Stop()
		return
	}

	glog.Info("Resume Session Success: ", session.clientIPPort, "; ", session.fullWorkerName, "; ", session.miningCoin)

	// 此后转入纯代理模式
	session.proxyStratum()
}

// Stop 停止一个 Stratum 会话
func (session *StratumSession) Stop() {
	session.lock.Lock()

	if session.runningStat == StatStoped {
		session.lock.Unlock()
		return
	}

	session.runningStat = StatStoped
	session.lock.Unlock()

	if session.serverConn != nil {
		session.serverConn.Close()
	}

	if session.clientConn != nil {
		session.clientConn.Close()
	}

	session.manager.ReleaseStratumSession(session)
	session.manager = nil

	if glog.V(2) {
		glog.Info("Session Stoped: ", session.clientIPPort, "; ", session.fullWorkerName, "; ", session.miningCoin)
	}
}

func (session *StratumSession) protocolDetect() ProtocolType {
	magicNumber, err := session.peekFromClientWithTimeout(1, protocolDetectTimeoutSeconds*time.Second)

	if err != nil {
		glog.Warning("read failed: ", err)
		return ProtocolUnknown
	}

	// 从客户端收到的第一个报文一定是Stratum协议的JSON字符串。
	// BTC Agent在subscribe和authorize阶段发送的是标准Stratum协议JSON字符串，
	// 只有在authorize完成之后才可能出现ex-message。
	//
	// 这也就是说，一方面，BTC Agent可以和普通矿机共享连接和认证流程，
	// 另一方面，我们无法在最开始就检测出客户端是BTC Agent，我们要随时做好收到ex-message的准备。
	if magicNumber[0] != '{' {
		glog.Warning("Unknown Protocol")
		return ProtocolUnknown
	}

	if glog.V(3) {
		glog.Info("Found Stratum Protocol")
	}

	return session.getDefaultStratumProtocol()
}

func (session *StratumSession) getDefaultStratumProtocol() ProtocolType {
	if session.manager.chainType == ChainTypeBitcoin {
		return ProtocolBitcoinStratum
	}

	if session.manager.chainType == ChainTypeEthereum {
		// This is the default protocol. The protocol may change after further detection.
		// The difference between ProtocolEthereumProxy and the other two Ethereum protocols is that
		// ProtocolEthereumProxy is no "mining.subscribe" phase, so it is set as default to simplify the detection.
		return ProtocolEthereumProxy
	}

	return ProtocolUnknown
}

func (session *StratumSession) runProxyStratum() {
	var err error

	err = session.stratumFindWorkerName()

	if err != nil {
		session.Stop()
		return
	}

	err = session.findMiningCoin()

	if err != nil {
		session.Stop()
		return
	}

	err = session.connectStratumServer()

	if err != nil {
		session.Stop()
		return
	}

	// 此后转入纯代理模式
	session.proxyStratum()
}

func (session *StratumSession) parseSubscribeRequest(request *JSONRPCRequest) (result interface{}, err *StratumError) {
	// 保存原始订阅请求以便转发给Stratum服务器
	session.stratumSubscribeRequest = request

	// 生成响应
	switch session.manager.chainType {
	case ChainTypeBitcoin:
		if len(request.Params) >= 1 {
			userAgent, ok := session.stratumSubscribeRequest.Params[0].(string)
			// 判断是否为BTCAgent
			if ok && strings.HasPrefix(strings.ToLower(userAgent), btcAgentClientTypePrefix) {
				session.isBTCAgent = true
			}
		}

		result = JSONRPCArray{JSONRPCArray{JSONRPCArray{"mining.set_difficulty", session.sessionIDString}, JSONRPCArray{"mining.notify", session.sessionIDString}}, session.sessionIDString, 8}
		return

	case ChainTypeEthereum:
		// only ProtocolEthereumStratum and ProtocolEthereumStratumNiceHash has the "mining.subscribe" phase
		session.protocolType = ProtocolEthereumStratum
		result = true
		if len(request.Params) >= 2 {
			// message example: {"id":1,"method":"mining.subscribe","params":["ethminer 0.15.0rc1","EthereumStratum/1.0.0"]}
			protocol, ok := session.stratumSubscribeRequest.Params[1].(string)

			// 判断是否为"EthereumStratum/xxx"
			if ok && strings.HasPrefix(strings.ToLower(protocol), ethereumStratumNiceHashPrefix) {
				session.protocolType = ProtocolEthereumStratumNiceHash

				// message example: {"id":1,"jsonrpc":"2.0","result":[["mining.notify","01003f","EthereumStratum/1.0.0"],"01003f"],"error":null}
				result = JSONRPCArray{JSONRPCArray{"mining.notify", session.sessionIDString, ethereumStratumNiceHashVersion}, session.sessionIDString}
			}
		}
		return

	default:
		glog.Fatal("Unknown Chain Type: ", session.manager.chainType)
		err = StratumErrUnknownChainType
		return
	}
}

func (session *StratumSession) makeSubscribeMessageForEthProxy() {
	// 为ETHProxy协议生成一个订阅请求
	// 该订阅请求是为了向sserver发送session id、矿机IP等需要而创建的
	session.stratumSubscribeRequest = new(JSONRPCRequest)
	session.stratumSubscribeRequest.Method = "mining.subscribe"
	session.stratumSubscribeRequest.SetParam("ETHProxy", ethproxyVersion)
}

func (session *StratumSession) parseAuthorizeRequest(request *JSONRPCRequest) (result interface{}, err *StratumError) {
	// 保存原始请求以便转发给Stratum服务器
	session.stratumAuthorizeRequest = request

	// STRATUM / NICEHASH_STRATUM:        {"id":3, "method":"mining.authorize", "params":["test.aaa", "x"]}
	// ETH_PROXY (Claymore):              {"worker": "eth1.0", "jsonrpc": "2.0", "params": ["0x00d8c82Eb65124Ea3452CaC59B64aCC230AA3482.test.aaa", "x"], "id": 2, "method": "eth_submitLogin"}
	// ETH_PROXY (EthMiner, situation 1): {"id":1, "method":"eth_submitLogin", "params":["0x00d8c82Eb65124Ea3452CaC59B64aCC230AA3482"], "worker":"test.aaa"}
	// ETH_PROXY (EthMiner, situation 2): {"id":1, "method":"eth_submitLogin", "params":["test"], "worker":"aaa"}

	if len(request.Params) < 1 {
		err = StratumErrTooFewParams
		return
	}

	fullWorkerName, ok := request.Params[0].(string)

	if !ok {
		err = StratumErrWorkerNameMustBeString
		return
	}

	// 矿工名
	session.fullWorkerName = strings.TrimSpace(fullWorkerName)

	// 以太坊矿工名中可能包含钱包地址，且矿工名本身可能位于附加的worker字段
	if session.protocolType != ProtocolBitcoinStratum {
		if request.Worker != "" {
			session.fullWorkerName += "." + strings.TrimSpace(request.Worker)
		}
		session.fullWorkerName = StripEthAddrFromFullName(session.fullWorkerName)
	}

	if strings.Contains(session.fullWorkerName, ".") {
		// 截取“.”之前的做为子账户名，“.”及之后的做矿机名
		pos := strings.Index(session.fullWorkerName, ".")
		session.subaccountName = session.fullWorkerName[:pos]
		session.minerNameWithDot = session.fullWorkerName[pos:]
	} else {
		session.subaccountName = session.fullWorkerName
		session.minerNameWithDot = ""
	}

	if len(session.subaccountName) < 1 {
		err = StratumErrWorkerNameStartWrong
		return
	}

	// 获取矿机名成功，但此处不需要返回内容给矿机
	// 连接服务器后会将服务器发送的响应返回给矿机
	result = nil
	err = nil
	return
}

func (session *StratumSession) stratumHandleRequest(request *JSONRPCRequest, stat *AuthorizeStat) (result interface{}, err *StratumError) {
	switch request.Method {
	case "mining.subscribe":
		if *stat != StatConnected {
			err = StratumErrDuplicateSubscribed
			return
		}
		result, err = session.parseSubscribeRequest(request)
		if err == nil {
			*stat = StatSubScribed
		}
		return

	case "eth_submitLogin":
		if session.protocolType == ProtocolEthereumProxy {
			session.makeSubscribeMessageForEthProxy()
			*stat = StatSubScribed
		}
		fallthrough
	case "mining.authorize":
		if *stat != StatSubScribed {
			err = StratumErrNeedSubscribed
			return
		}
		result, err = session.parseAuthorizeRequest(request)
		if err == nil {
			*stat = StatAuthorized
		}
		return

	default:
		// ignore unimplemented methods
		return
	}
}

func (session *StratumSession) stratumFindWorkerName() error {
	e := make(chan error, 1)

	go func() {
		defer close(e)
		response := new(JSONRPCResponse)

		stat := StatConnected

		// 循环结束说明认证成功
		for stat != StatAuthorized {
			requestJSON, err := session.clientReader.ReadBytes('\n')

			if err != nil {
				e <- errors.New("read line failed: " + err.Error())
				return
			}

			request, err := NewJSONRPCRequest(requestJSON)

			// ignore the json decode error
			if err != nil {
				if glog.V(3) {
					glog.Info("JSON decode failed: ", err.Error(), string(requestJSON))
				}
				continue
			}

			// stat will be changed in stratumHandleRequest
			result, stratumErr := session.stratumHandleRequest(request, &stat)

			// 两个均为空说明没有想要返回的响应
			if result != nil || stratumErr != nil {
				response.ID = request.ID
				response.Result = result
				response.Error = stratumErr.ToJSONRPCArray(session.manager.serverID)

				_, err = session.writeJSONResponseToClient(response)

				if err != nil {
					e <- errors.New("Write JSON Response Failed: " + err.Error())
					return
				}
			}
		} // for

		// 发送一个空错误表示成功
		e <- nil
		return
	}()

	select {
	case err := <-e:
		if err != nil {
			glog.Warning(err)
			return err
		}

		if glog.V(2) {
			glog.Info("FindWorkerName Success: ", session.fullWorkerName)
		}
		return nil

	case <-time.After(findWorkerNameTimeoutSeconds * time.Second):
		glog.Warning("FindWorkerName Timeout")
		return errors.New("FindWorkerName Timeout")
	}
}

func (session *StratumSession) findMiningCoin() error {
	// 从zookeeper读取用户想挖的币种

	session.zkWatchPath = session.manager.zookeeperSwitcherWatchDir + session.subaccountName
	data, event, err := session.manager.zookeeperManager.GetW(session.zkWatchPath, session.sessionID)

	if err != nil {
		if glog.V(3) {
			glog.Info("FindMiningCoin Failed: " + session.zkWatchPath + "; " + err.Error())
		}

		var response JSONRPCResponse
		response.Error = NewStratumError(201, "Cannot Found Minning Coin Type").ToJSONRPCArray(session.manager.serverID)
		if session.stratumAuthorizeRequest != nil {
			response.ID = session.stratumAuthorizeRequest.ID
		}

		session.writeJSONResponseToClient(&response)
		return err
	}

	session.miningCoin = string(data)
	session.zkWatchEvent = event

	return nil
}

func (session *StratumSession) connectStratumServer() error {
	// 获取当前运行状态
	runningStat := session.getStatNonLock()
	// 寻找币种对应的服务器
	serverInfo, ok := session.manager.stratumServerInfoMap[session.miningCoin]

	var rpcID interface{}
	if session.stratumAuthorizeRequest != nil {
		rpcID = session.stratumAuthorizeRequest.ID
	}

	// 对应的服务器不存在
	if !ok {
		glog.Error("Stratum Server Not Found: ", session.miningCoin)
		if runningStat != StatReconnecting {
			response := JSONRPCResponse{rpcID, nil, StratumErrStratumServerNotFound.ToJSONRPCArray(session.manager.serverID)}
			session.writeJSONResponseToClient(&response)
		}
		return StratumErrStratumServerNotFound
	}

	// 连接服务器
	serverConn, err := net.Dial("tcp", serverInfo.URL)

	if err != nil {
		glog.Error("Connect Stratum Server Failed: ", session.miningCoin, "; ", serverInfo.URL, "; ", err)
		if runningStat != StatReconnecting {
			response := JSONRPCResponse{rpcID, nil, StratumErrConnectStratumServerFailed.ToJSONRPCArray(session.manager.serverID)}
			session.writeJSONResponseToClient(&response)
		}
		return StratumErrConnectStratumServerFailed
	}

	if glog.V(3) {
		glog.Info("Connect Stratum Server Success: ", session.miningCoin, "; ", serverInfo.URL)
	}

	session.serverConn = serverConn
	session.serverReader = bufio.NewReaderSize(serverConn, bufioReaderBufSize)

	userAgent := "stratumSwitcher"
	protocol := "Stratum"

	// 发送订阅消息给服务器
	if session.stratumSubscribeRequest != nil {
		switch session.protocolType {
		case ProtocolBitcoinStratum:
			// 为请求添加sessionID
			// API格式：mining.subscribe("user agent/version", "extranonce1")
			// <https://en.bitcoin.it/wiki/Stratum_mining_protocol>

			// 获取原始的参数1（user agent）
			if len(session.stratumSubscribeRequest.Params) >= 1 {
				userAgent, _ = session.stratumSubscribeRequest.Params[0].(string)
			}
			if glog.V(3) {
				glog.Info("UserAgent: ", userAgent)
			}

			// 为了保证Web侧“最近提交IP”显示正确，将矿机的IP做为第三个参数传递给Stratum Server
			clientIP := session.clientIPPort[:strings.LastIndex(session.clientIPPort, ":")]
			clientIPLong := IP2Long(clientIP)
			session.stratumSubscribeRequest.SetParam(userAgent, session.sessionIDString, clientIPLong)

		case ProtocolEthereumStratum:
			fallthrough
		case ProtocolEthereumStratumNiceHash:
			fallthrough
		case ProtocolEthereumProxy:
			// 获取原始的参数1（user agent）和参数2（protocol，可能存在）
			if len(session.stratumSubscribeRequest.Params) >= 1 {
				userAgent, _ = session.stratumSubscribeRequest.Params[0].(string)
			}
			if len(session.stratumSubscribeRequest.Params) >= 2 {
				protocol, _ = session.stratumSubscribeRequest.Params[1].(string)
			}
			if glog.V(3) {
				glog.Info("UserAgent: ", userAgent, "; Protocol: ", protocol)
			}

			clientIP := session.clientIPPort[:strings.LastIndex(session.clientIPPort, ":")]
			clientIPLong := IP2Long(clientIP)

			// Session ID 做为第三个参数传递
			// 矿机IP做为第四个参数传递
			session.stratumSubscribeRequest.SetParam(userAgent, protocol, session.sessionIDString, clientIPLong)

		default:
			glog.Fatal("Unimplemented Stratum Protocol: ", session.protocolType)
			return ErrParseSubscribeResponseFailed
		}

		// 发送mining.subscribe请求给服务器
		// sessionID已包含在其中，一并发送给服务器
		_, err = session.writeJSONRequestToServer(session.stratumSubscribeRequest)
		if err != nil {
			glog.Warning("Write Subscribe Request Failed: ", err)
			return err
		}

		responseJSON, err := session.readLineFromServerWithTimeout(readSubscribeResponseTimeoutSeconds * time.Second)
		if err != nil {
			glog.Warning("Read Subscribe Response Failed: ", err)
			return err
		}

		response, err := NewJSONRPCResponse(responseJSON)
		if err != nil {
			glog.Warning("Parse Subscribe Response Failed: ", err)
			return err
		}

		// 检查服务器返回的订阅结果
		switch session.protocolType {
		case ProtocolBitcoinStratum:
			fallthrough
		case ProtocolEthereumStratumNiceHash:
			result, ok := response.Result.([]interface{})
			if !ok {
				glog.Warning("Parse Subscribe Response Failed: result is not an array")
				return ErrParseSubscribeResponseFailed
			}
			if len(result) < 2 {
				glog.Warning("Field too Few of Subscribe Response Result: ", result)
				return ErrParseSubscribeResponseFailed
			}

			sessionID, ok := result[1].(string)
			if !ok {
				glog.Warning("Parse Subscribe Response Failed: result[1] is not a string")
				return ErrParseSubscribeResponseFailed
			}

			// 服务器返回的 sessionID 与当前保存的不一致，此时挖到的所有share都会是无效的，断开连接
			if sessionID != session.sessionIDString {
				glog.Warning("Session ID Mismatched:  ", sessionID, " != ", session.sessionIDString)
				return ErrSessionIDInconformity
			}

		case ProtocolEthereumStratum:
			fallthrough
		case ProtocolEthereumProxy:
			result, ok := response.Result.(bool)
			if !ok || !result {
				glog.Warning("Parse Subscribe Response Failed: response is ", string(responseJSON))
				return ErrParseSubscribeResponseFailed
			}

		default:
			glog.Fatal("Unimplemented Stratum Protocol: ", session.protocolType)
			return ErrParseSubscribeResponseFailed
		}

		if glog.V(3) {
			glog.Info("Subscribe Success: ", string(responseJSON))
		}
	}

	// 认证响应的JSON数据
	var authorizeResponseJSON []byte
	// 添加了币种后缀的矿机名
	fullWorkerNameWithCoinPostfix := session.subaccountName + "_" + session.miningCoin + session.minerNameWithDot

	// 认证状态
	var authSuccess = false
	// 最后一次尝试的矿机名
	var authWorkerName string
	// 矿机的密码，仅用于显示
	var authWorkerPasswd string

	if len(session.stratumAuthorizeRequest.Params) >= 2 {
		authWorkerPasswd, _ = session.stratumAuthorizeRequest.Params[1].(string)
	}

	// 在15秒内多次尝试认证
	// 之所以要多次认证，是因为第一次创建子账户的时候，Stratum Server不能及时的收到消息。
	// 新创建的子账户需要约10秒才能在Stratum Server可用。
	for i := 0; i < 5; i++ {
		// 首次认证尝试
		authWorkerName = fullWorkerNameWithCoinPostfix
		if glog.V(3) {
			glog.Info("Authorize: ", authWorkerName)
		}
		authSuccess, authorizeResponseJSON = session.miningAuthorize(authWorkerName)

		if authSuccess {
			break
		}

		// 认证没有成功，去掉币种后缀再试
		// 目前来说只有开启切换功能时新创建的子账户有币种后缀，之前的子账户并没有
		// 并且，即使重命名了子账户，没有重启过的stratum server也不会感知到子账户名已经改变
		if glog.V(3) {
			glog.Info("Authorize failed with ", authWorkerName, ", try ", session.fullWorkerName)
		}
		authWorkerName = session.fullWorkerName
		authSuccess, authorizeResponseJSON = session.miningAuthorize(authWorkerName)

		if authSuccess {
			break
		}

		// 还是没有成功，sleep 3秒
		time.Sleep(time.Duration(3) * time.Second)
	}

	// 若认证响应不为空，就转发给矿机，无论认证是否成功
	// 在重连服务器时不发送
	if authorizeResponseJSON != nil && runningStat != StatReconnecting {
		_, err := session.clientConn.Write(authorizeResponseJSON)

		if err != nil {
			glog.Warning("Write Authorize Response to Client Failed: ", err)
			return err
		}
	}

	// 返回认证的结果（若认证失败，则认为连接失败）
	if !authSuccess {
		glog.Warning("Authorize Failed: ", authWorkerName, "; ", session.miningCoin)
		return ErrAuthorizeFailed
	}

	glog.Info("Authorize Success: ", session.clientIPPort, "; ", session.miningCoin, "; ", authWorkerName, "; ", authWorkerPasswd, "; ", userAgent, "; ", protocol)
	return nil
}

// miningAuthorize 矿机认证
func (session *StratumSession) miningAuthorize(fullWorkerName string) (bool, []byte) {
	var request JSONRPCRequest

	// 深拷贝
	request.ID = session.stratumAuthorizeRequest.ID
	request.Method = session.stratumAuthorizeRequest.Method
	request.Params = make([]interface{}, len(session.stratumAuthorizeRequest.Params))
	copy(request.Params, session.stratumAuthorizeRequest.Params)

	// 设置为（可能）添加了币种后缀的矿工名
	request.Params[0] = fullWorkerName

	// 发送mining.authorize请求给服务器
	_, err := session.writeJSONRequestToServer(&request)

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
	if session.getStat() != StatRunning {
		glog.Info("proxyStratum: session stopped by another goroutine")
		return
	}

	// 注册会话
	session.manager.RegisterStratumSession(session)

	// 从服务器到客户端
	go func() {
		// 记录当前的币种切换计数
		currentReconnectCounter := session.getReconnectCounter()

		bufLen := session.serverReader.Buffered()
		// 将bufio中的剩余内容写入对端
		if bufLen > 0 {
			buf := make([]byte, bufLen)
			session.serverReader.Read(buf)
			session.clientConn.Write(buf)
		}
		// 释放bufio
		session.serverReader = nil
		// 简单的流复制
		buffer := make([]byte, bufioReaderBufSize)
		_, err := IOCopyBuffer(session.clientConn, session.serverConn, buffer)
		// 流复制结束，说明其中一方关闭了连接
		// 不对BTCAgent应用重连
		if err == ErrReadFailed && !session.isBTCAgent {
			// 服务器关闭了连接，尝试重连
			session.tryReconnect(currentReconnectCounter)
		} else {
			// 客户端关闭了连接，结束会话
			session.tryStop(currentReconnectCounter)
		}
		if glog.V(3) {
			glog.Info("DownStream: exited; ", session.clientIPPort, "; ", session.fullWorkerName, "; ", session.miningCoin)
		}
	}()

	// 从客户端到服务器
	go func() {
		// 记录当前的币种切换计数
		currentReconnectCounter := session.getReconnectCounter()

		bufLen := session.clientReader.Buffered()
		// 将bufio中的剩余内容写入对端
		if bufLen > 0 {
			buf := make([]byte, bufLen)
			session.clientReader.Read(buf)
			session.serverConn.Write(buf)
		}
		// 释放bufio
		session.clientReader = nil
		// 简单的流复制
		buffer := make([]byte, bufioReaderBufSize)
		bufferLen, err := IOCopyBuffer(session.serverConn, session.clientConn, buffer)
		// 流复制结束，说明其中一方关闭了连接
		// 不对BTCAgent应用重连
		if err == ErrWriteFailed && !session.isBTCAgent {
			// 服务器关闭了连接，尝试重连
			session.tryReconnect(currentReconnectCounter)
			// 若重连成功，尝试将缓存中的内容转发到新服务器
			// getStat() 会锁定到重连成功或放弃重连为止
			if bufferLen > 0 && session.getStat() == StatRunning {
				session.serverConn.Write(buffer[0:bufferLen])
			}
		} else {
			// 客户端关闭了连接，结束会话
			session.tryStop(currentReconnectCounter)
		}
		if glog.V(3) {
			glog.Info("UpStream: exited; ", session.clientIPPort, "; ", session.fullWorkerName, "; ", session.miningCoin)
		}
	}()

	// 监控来自zookeeper的切换指令并进行Stratum切换
	go func() {
		// 记录当前的币种切换计数
		currentReconnectCounter := session.getReconnectCounter()

		for {
			<-session.zkWatchEvent

			if !session.IsRunning() {
				break
			}

			if currentReconnectCounter != session.getReconnectCounter() {
				break
			}

			data, event, err := session.manager.zookeeperManager.GetW(session.zkWatchPath, session.sessionID)

			if err != nil {
				glog.Error("Read From Zookeeper Failed, sleep ", zookeeperConnAliveTimeout, "s: ", session.zkWatchPath, "; ", err)
				time.Sleep(zookeeperConnAliveTimeout * time.Second)
				continue
			}

			session.zkWatchEvent = event
			newMiningCoin := string(data)

			// 若币种未改变，则继续监控
			if newMiningCoin == session.miningCoin {
				if glog.V(3) {
					glog.Info("Mining Coin Not Changed: ", session.fullWorkerName, ": ", session.miningCoin, " -> ", newMiningCoin)
				}
				continue
			}

			// 若币种对应的Stratum服务器不存在，则忽略事件并继续监控
			_, exists := session.manager.stratumServerInfoMap[newMiningCoin]
			if !exists {
				glog.Error("Stratum Server Not Found for New Mining Coin: ", newMiningCoin)
				continue
			}

			// 币种已改变
			if glog.V(2) {
				glog.Info("Mining Coin Changed: ", session.fullWorkerName, "; ", session.miningCoin, " -> ", newMiningCoin, "; ", currentReconnectCounter)
			}

			// 进行币种切换
			if session.isBTCAgent {
				// 因为BTCAgent会话是有状态的（一个连接里包含多个AgentSession，
				// 对应多台矿机），所以没有办法安全的无缝切换BTCAgent会话，
				// 只能采用断开连接的方法。
				session.tryStop(currentReconnectCounter)
			} else {
				// 普通连接，直接切换币种
				session.switchCoinType(newMiningCoin, currentReconnectCounter)
			}
			break
		}

		if glog.V(3) {
			glog.Info("CoinWatcher: exited; ", session.clientIPPort, "; ", session.fullWorkerName, "; ", session.miningCoin)
		}
	}()
}

// 检查是否发生了重连，若未发生重连，则停止会话
func (session *StratumSession) tryStop(currentReconnectCounter uint32) bool {
	session.lock.Lock()
	defer session.lock.Unlock()

	// 会话未在运行，不停止
	if session.runningStat != StatRunning {
		return false
	}

	// 判断是否已经重连过
	if currentReconnectCounter == session.reconnectCounter {
		//未发生重连，尝试停止
		go session.Stop()
		return true
	}

	// 已重连，则不进行任何操作
	return false
}

// 检查是否发生了重连，若未发生重连，则尝试重连
func (session *StratumSession) tryReconnect(currentReconnectCounter uint32) bool {
	session.lock.Lock()
	defer session.lock.Unlock()

	// 会话未在运行，不重连
	if session.runningStat != StatRunning {
		return false
	}

	// 判断是否已经重连过
	if currentReconnectCounter == session.reconnectCounter {
		//未发生重连，尝试重连
		// 状态设为“正在重连服务器”，重连计数器加一
		session.setStatNonLock(StatReconnecting)
		session.reconnectCounter++

		session.reconnectStratumServer(retryTimeWhenServerDown)
		return true
	}

	// 已重连，则不进行任何操作
	return false
}

func (session *StratumSession) switchCoinType(newMiningCoin string, currentReconnectCounter uint32) {
	// 设置新币种
	session.miningCoin = newMiningCoin

	// 锁定会话，防止会话被其他线程停止
	session.lock.Lock()
	defer session.lock.Unlock()

	// 会话未在运行，放弃操作
	if session.runningStat != StatRunning {
		glog.Warning("SwitchCoinType: session not running")
		return
	}
	// 会话已被其他线程重连，放弃操作
	if currentReconnectCounter != session.reconnectCounter {
		glog.Warning("SwitchCoinType: session reconnected by other goroutine")
		return
	}
	// 会话未被重连，可操作
	// 状态设为“正在重连服务器”，重连计数器加一
	session.setStatNonLock(StatReconnecting)
	session.reconnectCounter++

	// 重连服务器
	session.reconnectStratumServer(retryTimeWhenServerDown)
}

// reconnectStratumServer 重连服务器
func (session *StratumSession) reconnectStratumServer(retryTime int) {
	// 移除会话注册
	session.manager.UnRegisterStratumSession(session)

	// 销毁serverReader
	if session.serverReader != nil {
		bufLen := session.serverReader.Buffered()
		// 将bufio中的剩余内容写入对端
		if bufLen > 0 {
			buf := make([]byte, bufLen)
			session.serverReader.Read(buf)
			session.clientConn.Write(buf)
		}
		session.serverReader = nil
	}

	// 断开原服务器
	session.serverConn.Close()
	session.serverConn = nil

	// 重新创建clientReader
	if session.clientReader == nil {
		session.clientReader = bufio.NewReaderSize(session.clientConn, bufioReaderBufSize)
	}

	// 连接服务器
	var err error
	// 至少要尝试一次，所以从-1开始
	for i := -1; i < retryTime; i++ {
		err = session.connectStratumServer()
		if err == nil {
			break
		} else {
			time.Sleep(1 * time.Second)
		}
	}
	if err != nil {
		go session.Stop()
		return
	}

	// 回到运行状态
	session.setStatNonLock(StatRunning)

	// 转入纯代理模式
	go session.proxyStratum()
}

func peekWithTimeout(reader *bufio.Reader, len int, timeout time.Duration) ([]byte, error) {
	e := make(chan error, 1)
	var buffer []byte

	go func() {
		data, err := reader.Peek(len)
		buffer = data
		e <- err
		close(e)
	}()

	select {
	case err := <-e:
		return buffer, err
	case <-time.After(timeout):
		return nil, ErrBufIOReadTimeout
	}
}

func (session *StratumSession) peekFromClientWithTimeout(len int, timeout time.Duration) ([]byte, error) {
	return peekWithTimeout(session.clientReader, len, timeout)
}

func (session *StratumSession) peekFromServerWithTimeout(len int, timeout time.Duration) ([]byte, error) {
	return peekWithTimeout(session.serverReader, len, timeout)
}

func readByteWithTimeout(reader *bufio.Reader, buffer []byte, timeout time.Duration) (int, error) {
	e := make(chan error, 1)
	var length int

	go func() {
		len, err := reader.Read(buffer)
		length = len
		e <- err
		close(e)
	}()

	select {
	case err := <-e:
		return length, err
	case <-time.After(timeout):
		return 0, ErrBufIOReadTimeout
	}
}

func readLineWithTimeout(reader *bufio.Reader, timeout time.Duration) ([]byte, error) {
	e := make(chan error, 1)
	var buffer []byte

	go func() {
		data, err := reader.ReadBytes('\n')
		buffer = data
		e <- err
		close(e)
	}()

	select {
	case err := <-e:
		return buffer, err
	case <-time.After(timeout):
		return nil, ErrBufIOReadTimeout
	}
}

func (session *StratumSession) readByteFromClientWithTimeout(buffer []byte, timeout time.Duration) (int, error) {
	return readByteWithTimeout(session.clientReader, buffer, timeout)
}

func (session *StratumSession) readByteFromServerWithTimeout(buffer []byte, timeout time.Duration) (int, error) {
	return readByteWithTimeout(session.serverReader, buffer, timeout)
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

	defer session.clientConn.Write([]byte{'\n'})
	return session.clientConn.Write(bytes)
}

func (session *StratumSession) writeJSONRequestToServer(jsonData *JSONRPCRequest) (int, error) {
	bytes, err := jsonData.ToJSONBytes()

	if err != nil {
		return 0, err
	}

	defer session.serverConn.Write([]byte{'\n'})
	return session.serverConn.Write(bytes)
}
