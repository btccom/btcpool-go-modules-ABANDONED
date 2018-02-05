package main

import (
	"bufio"
	"errors"
	"io"
	"net"
	"strings"
	"sync"
	"time"

	"github.com/golang/glog"
	"github.com/samuel/go-zookeeper/zk"
)

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

// ProtocolType 代理的协议类型
type ProtocolType int

const (
	// ProtocolStratum Stratum协议
	ProtocolStratum = iota
	// ProtocolUnknown 未知协议（无法处理）
	ProtocolUnknown = iota
)

// StratumSession 是一个 Stratum 会话，包含了到客户端和到服务端的连接及状态信息
type StratumSession struct {
	// 会话管理器
	manager *StratumSessionManager

	// 是否在运行
	isRunning bool
	// 改变运行状态时进行加锁
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

	session.isRunning = false
	session.manager = manager
	session.sessionID = sessionID

	session.clientConn = clientConn
	session.clientReader = bufio.NewReader(clientConn)

	session.clientIPPort = clientConn.RemoteAddr().String()
	session.sessionIDString = Uint32ToHex(session.sessionID)

	glog.V(3).Info("IP: ", session.clientIPPort, ", Session ID: ", session.sessionIDString)

	return
}

// IsRunning 检查会话是否在运行（线程安全）
func (session *StratumSession) IsRunning() bool {
	session.lock.Lock()
	defer session.lock.Unlock()

	return session.isRunning
}

// Run 启动一个 Stratum 会话
func (session *StratumSession) Run() {
	session.lock.Lock()

	if session.isRunning {
		session.lock.Unlock()
		return
	}

	session.isRunning = true
	session.lock.Unlock()

	protocolType := session.protocolDetect()

	// 其实目前只有一种协议，即Stratum协议
	// BTCAgent在认证完成之前走的也是Stratum协议
	if protocolType != ProtocolStratum {
		session.Stop()
		return
	}

	session.runProxyStratum()
}

// Resume 恢复一个Stratum会话
func (session *StratumSession) Resume(sessionData StratumSessionData, serverConn net.Conn) {
	session.lock.Lock()

	if session.isRunning {
		session.lock.Unlock()
		return
	}

	session.isRunning = true
	session.lock.Unlock()

	// 恢复服务器连接
	session.serverConn = serverConn
	session.serverReader = bufio.NewReader(serverConn)

	_, stratumErr := session.parseSubscribeRequest(sessionData.StratumSubscribeRequest)
	if stratumErr != nil {
		glog.Error("Resume session ", session.clientIPPort, " failed: ", stratumErr)
		session.Stop()
		return
	}

	stratumErr = session.parseAuthorizeRequest(sessionData.StratumAuthorizeRequest)
	if stratumErr != nil {
		glog.Error("Resume session ", session.clientIPPort, " failed: ", stratumErr)
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

	if !session.isRunning {
		session.lock.Unlock()
		return
	}

	session.isRunning = false
	session.lock.Unlock()

	if session.serverConn != nil {
		session.serverConn.Close()
	}

	if session.clientConn != nil {
		session.clientConn.Close()
	}

	session.manager.ReleaseStratumSession(session)
	session.manager = nil

	glog.V(2).Info("Session Stoped: ", session.clientIPPort, "; ", session.fullWorkerName, "; ", session.miningCoin)
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

	glog.V(3).Info("Found Stratum Protocol")
	return ProtocolStratum
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

func (session *StratumSession) parseSubscribeRequest(request *JSONRPCRequest) (interface{}, *StratumError) {
	if request.Method != "mining.subscribe" {
		return nil, StratumErrNeedSubscribed
	}

	// 保存原始订阅请求以便转发给Stratum服务器
	session.stratumSubscribeRequest = request

	// 生成响应
	result := JSONRPCArray{JSONRPCArray{JSONRPCArray{"mining.set_difficulty", session.sessionIDString}, JSONRPCArray{"mining.notify", session.sessionIDString}}, session.sessionIDString, 8}
	return result, nil
}

func (session *StratumSession) parseAuthorizeRequest(request *JSONRPCRequest) *StratumError {
	if request.Method != "mining.authorize" {
		return StratumErrNeedAuthorize
	}

	if len(request.Params) < 1 {
		return StratumErrTooFewParams
	}

	fullWorkerName, ok := request.Params[0].(string)

	if !ok {
		return StratumErrWorkerNameMustBeString
	}

	// 矿工名
	session.fullWorkerName = strings.TrimSpace(fullWorkerName)

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
		return StratumErrWorkerNameStartWrong
	}

	// 保存原始请求以便转发给Stratum服务器
	session.stratumAuthorizeRequest = request

	return nil
}

func (session *StratumSession) stratumFindWorkerName() error {
	e := make(chan error, 1)

	go func() {
		defer close(e)
		response := new(JSONRPCResponse)

		// 矿机订阅
		for {
			requestJSON, err := session.clientReader.ReadBytes('\n')

			if err != nil {
				e <- errors.New("read line failed: " + err.Error())
				return
			}

			request, err := NewJSONRPCRequest(requestJSON)

			// ignore the json decode error
			if err != nil {
				glog.V(3).Info("JSON decode failed: ", err.Error(), string(requestJSON))
				continue
			}

			result, stratumErr := session.parseSubscribeRequest(request)

			response.ID = request.ID
			response.Result = result
			response.Error = stratumErr.ToJSONRPCArray()

			_, err = session.writeJSONResponseToClient(response)

			if err != nil {
				e <- errors.New("Write JSON Response Failed: " + err.Error())
				return
			}

			// 如果订阅成功则跳出循环
			if stratumErr == nil {
				break
			}
		}

		// 矿机认证
		for {
			requestJSON, err := session.clientReader.ReadBytes('\n')

			if err != nil {
				e <- errors.New("read line failed: " + err.Error())
				return
			}

			request, err := NewJSONRPCRequest(requestJSON)

			// ignore the json decode error
			if err != nil {
				glog.V(3).Info("JSON decode failed: ", err.Error(), string(requestJSON))
				continue
			}

			stratumErr := session.parseAuthorizeRequest(request)

			// 如果认证成功则跳出循环
			// 此时不发送认证成功的响应，因为事实上还没有连接服务器进行认证
			if stratumErr == nil {
				// 发送一个空错误表示成功
				e <- nil
				return
			}

			// 否则，把错误信息发给矿机
			response.ID = request.ID
			response.Result = nil
			response.Error = stratumErr.ToJSONRPCArray()

			_, err = session.writeJSONResponseToClient(response)

			if err != nil {
				e <- errors.New("Write JSON Response Failed: " + err.Error())
				return
			}
		}
	}()

	select {
	case err := <-e:
		if err != nil {
			glog.Warning(err)
			return err
		}

		glog.V(2).Info("FindWorkerName Success: ", session.fullWorkerName)
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
		glog.V(3).Info("FindMiningCoin Failed: " + session.zkWatchPath + "; " + err.Error())

		response := JSONRPCResponse{nil, nil, JSONRPCArray{201, "Cannot Found Minning Coin Type", nil}}
		session.writeJSONResponseToClient(&response)

		return err
	}

	session.miningCoin = string(data)
	session.zkWatchEvent = event

	return nil
}

func (session *StratumSession) connectStratumServer() error {
	// 寻找币种对应的服务器
	serverInfo, ok := session.manager.stratumServerInfoMap[session.miningCoin]

	// 对应的服务器不存在
	if !ok {
		glog.Error("Stratum Server Not Found: ", session.miningCoin)

		response := JSONRPCResponse{nil, nil, StratumErrStratumServerNotFound.ToJSONRPCArray()}
		session.writeJSONResponseToClient(&response)
		return StratumErrStratumServerNotFound
	}

	// 连接服务器
	serverConn, err := net.Dial("tcp", serverInfo.URL)

	if err != nil {
		glog.Error("Connect Stratum Server Failed: ", session.miningCoin, "; ", serverInfo.URL, "; ", err)

		response := JSONRPCResponse{nil, nil, StratumErrConnectStratumServerFailed.ToJSONRPCArray()}
		session.writeJSONResponseToClient(&response)
		return StratumErrConnectStratumServerFailed
	}

	glog.V(3).Info("Connect Stratum Server Success: ", session.miningCoin, "; ", serverInfo.URL)

	session.serverConn = serverConn
	session.serverReader = bufio.NewReader(serverConn)

	// 为请求添加sessionID
	// API格式：mining.subscribe("user agent/version", "extranonce1")
	// <https://en.bitcoin.it/wiki/Stratum_mining_protocol>

	// 获取原始的参数1（user agent）
	userAgent := "stratumSwitcher"
	if len(session.stratumSubscribeRequest.Params) >= 1 {
		userAgent, ok = session.stratumSubscribeRequest.Params[0].(string)
	}
	glog.V(3).Info("UserAgent: ", userAgent)

	// 为了保证Web侧“最近提交IP”显示正确，将矿机的IP做为第三个参数传递给Stratum Server
	clientIP := session.clientIPPort[:strings.LastIndex(session.clientIPPort, ":")]
	clientIPLong := IP2Long(clientIP)
	session.stratumSubscribeRequest.SetParam(userAgent, session.sessionIDString, clientIPLong)

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

	// 检查服务器返回的 sessionID 与当前保存的是否一致
	response, err := NewJSONRPCResponse(responseJSON)

	if err != nil {
		glog.Warning("Parse Subscribe Response Failed: ", err)
		return err
	}

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
		glog.Warning("Session ID Inconformity:  ", sessionID, " != ", session.sessionIDString)
		return ErrSessionIDInconformity
	}

	glog.V(3).Info("Subscribe Success: ", string(responseJSON))

	// 认证响应的JSON数据
	var authorizeResponseJSON []byte
	// 添加了币种后缀的矿机名
	fullWorkerNameWithCoinPostfix := session.subaccountName + "_" + session.miningCoin + session.minerNameWithDot

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

	glog.Info("Authorize Success: ", session.clientIPPort, "; ", session.miningCoin, "; ", authWorkerName, "; ", userAgent)
	return nil
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
	// 注册会话
	session.manager.RegisterStratumSession(session)

	// 从服务器到客户端
	go func() {
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
		io.Copy(session.clientConn, session.serverConn)
		// 流复制结束，说明其中一方关闭了连接
		session.Stop()
		glog.V(3).Info("DownStream: exited;", session.clientIPPort, "; ", session.fullWorkerName, "; ", session.miningCoin)
	}()

	// 从客户端到服务器
	go func() {
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
		io.Copy(session.serverConn, session.clientConn)
		// 流复制结束，说明其中一方关闭了连接
		session.Stop()
		glog.V(3).Info("UpStream: exited;", session.clientIPPort, "; ", session.fullWorkerName, "; ", session.miningCoin)
	}()

	// 监控来自zookeeper的切换指令并进行Stratum切换
	go func() {
		for {
			<-session.zkWatchEvent

			if !session.IsRunning() {
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
				glog.V(3).Info("Mining Coin Not Changed: ", session.fullWorkerName, ": ", session.miningCoin, " -> ", newMiningCoin)
				continue
			}

			// 若币种对应的Stratum服务器不存在，则忽略事件并继续监控
			_, exists := session.manager.stratumServerInfoMap[newMiningCoin]
			if !exists {
				glog.Error("Stratum Server Not Found for New Mining Coin: ", newMiningCoin)
				continue
			}

			// 币种已改变
			glog.V(2).Info("Mining Coin Changed: ", session.fullWorkerName, ": ", session.miningCoin, " -> ", newMiningCoin)

			// 因为BTCAgent会话是有状态的（一个连接里包含多个AgentSessionID，对应多台矿机），
			// 因此没有办法安全的无缝切换BTCAgent会话。
			// 所以，采用断开连接的方法反而更保险。
			session.Stop()
			break
		}

		glog.V(3).Info("CoinWatcher: exited; ", session.clientIPPort, "; ", session.fullWorkerName, "; ", session.miningCoin)
	}()
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
