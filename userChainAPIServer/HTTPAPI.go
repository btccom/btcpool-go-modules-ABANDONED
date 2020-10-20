package main

import (
	"crypto/subtle"
	"encoding/json"
	"io/ioutil"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/golang/glog"
)

// SwitchUserCoins 欲切换的用户和币种
type SwitchUserCoins struct {
	Coin    string   `json:"coin"`
	PUNames []string `json:"punames"`
}

// SwitchMultiUserRequest 多用户切换请求数据结构
type SwitchMultiUserRequest struct {
	UserCoins []SwitchUserCoins `json:"usercoins"`
}

// APIResponse API响应数据结构
type APIResponse struct {
	ErrNo   int    `json:"err_no"`
	ErrMsg  string `json:"err_msg"`
	Success bool   `json:"success"`
}

// SubPoolUpdate 子池更新信息
type SubPoolUpdate struct {
	Coin         string `json:"coin"`
	SubPoolName  string `json:"subpool_name"`
	CoinbaseInfo string `json:"coinbase_info"`
	PayoutAddr   string `json:"payout_addr"`
}

// SubPoolCoinbase 子池Coinbase信息
type SubPoolCoinbase struct {
	Success     bool   `json:"success"`
	ErrNo       int    `json:"err_no"`
	ErrMsg      string `json:"err_msg"`
	SubPoolName string `json:"subpool_name"`
	Old         struct {
		CoinbaseInfo string `json:"coinbase_info"`
		PayoutAddr   string `json:"payout_addr"`
	} `json:"old"`
}

// SubPoolUpdateAck 子池更新响应
type SubPoolUpdateAck struct {
	SubPoolCoinbase
	New struct {
		CoinbaseInfo string `json:"coinbase_info"`
		PayoutAddr   string `json:"payout_addr"`
	} `json:"new"`
}

// SubPoolUpdateAckInner 子池更新响应(非公开)
type SubPoolUpdateAckInner struct {
	SubPoolUpdateAck
	Host struct {
		HostName string `json:"hostname"`
	} `json:"host"`
}

// HTTPRequestHandle HTTP请求处理函数
type HTTPRequestHandle func(http.ResponseWriter, *http.Request)

func writeSuccess(w http.ResponseWriter) {
	response := APIResponse{0, "", true}
	responseJSON, _ := json.Marshal(response)

	w.Write(responseJSON)
}

func writeError(w http.ResponseWriter, errNo int, errMsg string) {
	response := APIResponse{errNo, errMsg, false}
	responseJSON, _ := json.Marshal(response)

	w.Write(responseJSON)
}

// 启动 API Server
func (manager *UserChainManager) runAPIServer(waitGroup *sync.WaitGroup) {
	defer waitGroup.Done()

	// HTTP监听
	glog.Info("Listen HTTP ", manager.configData.ListenAddr)

	http.HandleFunc("/switch", manager.basicAuth(manager.switchHandle))

	http.HandleFunc("/switch/multi-user", manager.basicAuth(manager.switchMultiUserHandle))
	http.HandleFunc("/switch-multi-user", manager.basicAuth(manager.switchMultiUserHandle))

	http.HandleFunc("/subpool/get-coinbase", manager.basicAuth(manager.getCoinbaseHandle))
	http.HandleFunc("/subpool-get-coinbase", manager.basicAuth(manager.getCoinbaseHandle))

	http.HandleFunc("/subpool/update-coinbase", manager.basicAuth(manager.updateCoinbaseHandle))
	http.HandleFunc("/subpool-update-coinbase", manager.basicAuth(manager.updateCoinbaseHandle))

	err := http.ListenAndServe(manager.configData.ListenAddr, nil)

	if err != nil {
		glog.Fatal("HTTP Listen Failed: ", err)
		return
	}
}

// basicAuth 执行Basic认证
func (manager *UserChainManager) basicAuth(f HTTPRequestHandle) HTTPRequestHandle {
	return func(w http.ResponseWriter, r *http.Request) {
		apiUser := []byte(manager.configData.APIUser)
		apiPasswd := []byte(manager.configData.APIPassword)

		user, passwd, ok := r.BasicAuth()

		// 检查用户名密码是否正确
		if ok && subtle.ConstantTimeCompare(apiUser, []byte(user)) == 1 && subtle.ConstantTimeCompare(apiPasswd, []byte(passwd)) == 1 {
			// 执行被装饰的函数
			f(w, r)
			return
		}

		// 认证失败，提示 401 Unauthorized
		// Restricted 可以改成其他的值
		w.Header().Set("WWW-Authenticate", `Basic realm="Restricted"`)
		// 401 状态码
		w.WriteHeader(http.StatusUnauthorized)
		// 401 页面
		w.Write([]byte(`<h1>401 - Unauthorized</h1>`))
	}
}

// getCoinbaseHandle 获取子池coinbase信息
func (manager *UserChainManager) getCoinbaseHandle(w http.ResponseWriter, req *http.Request) {
	if len(manager.configData.ZKSubPoolUpdateBaseDir) <= 0 {
		writeError(w, 403, "API disabled")
		return
	}

	requestJSON, err := ioutil.ReadAll(req.Body)

	if err != nil {
		glog.Warning(err, ": ", req.RequestURI)
		writeError(w, 500, err.Error())
		return
	}

	var reqData SubPoolUpdate
	err = json.Unmarshal(requestJSON, &reqData)

	if err != nil {
		glog.Info(err, ": ", req.RequestURI)
		writeError(w, 400, "wrong JSON, "+err.Error())
		return
	}

	if len(reqData.Coin) < 1 {
		writeError(w, 400, "coin cannot be empty")
		return
	}
	if len(reqData.SubPoolName) < 1 {
		writeError(w, 400, "subpool_name cannot be empty")
		return
	}

	glog.Info("[subpool-get] Coin: ", reqData.Coin, ", SubPool: ", reqData.SubPoolName)

	reqNode := manager.configData.ZKSubPoolUpdateBaseDir + reqData.Coin + "/" + reqData.SubPoolName
	ackNode := reqNode + "/ack"

	reqByte, stat, err := manager.zookeeper.Get(reqNode)
	if err != nil {
		glog.Warning("[subpool-get] zk path '", reqNode, "' doesn't exists",
			" Coin: ", reqData.Coin, ", SubPool: ", reqData.SubPoolName)
		writeError(w, 404, "subpool '"+reqData.SubPoolName+"' does not exist")
		return
	}

	exists, _, ack, err := manager.zookeeper.ExistsW(ackNode)
	if err != nil || !exists {
		glog.Warning("[subpool-get] zk path '", ackNode, "' doesn't exists",
			" Coin: ", reqData.Coin, ", SubPool: ", reqData.SubPoolName)
		writeError(w, 503, "jobmaker cannot ACK the request")
		return
	}

	_, err = manager.zookeeper.Set(reqNode, reqByte, stat.Version)
	if err != nil {
		glog.Warning("[subpool-get] data has been updated at query time! ", err.Error(),
			" Coin: ", reqData.Coin, ", SubPool: ", reqData.SubPoolName)
		writeError(w, 500, "data has been updated at query time")
		return
	}

	select {
	case <-ack:
		ackJSON, _, err := manager.zookeeper.Get(ackNode)
		if err != nil {
			glog.Warning("[subpool-get] get ACK failed, ", err.Error(),
				" Coin: ", reqData.Coin, ", SubPool: ", reqData.SubPoolName)
			writeError(w, 500, "cannot get ACK from zookeeper")
			return
		}

		var ackData SubPoolUpdateAckInner
		err = json.Unmarshal(ackJSON, &ackData)
		if err != nil {
			glog.Warning("[subpool-get] parse ACK failed, ", err.Error(),
				" Coin: ", reqData.Coin, ", SubPool: ", reqData.SubPoolName)
			writeError(w, 500, "cannot parse ACK in zookeeper")
			return
		}

		if !ackData.Success && ackData.ErrMsg == "empty request" {
			ackData.Success = true
			ackData.ErrMsg = "success"
		}

		glog.Info("[subpool-get] Response: ", ackData.ErrMsg, ", Host: ", ackData.Host.HostName,
			", Coin: ", reqData.Coin, ", SubPool: ", reqData.SubPoolName,
			", Old: ", ackData.Old)

		ackByte, _ := json.Marshal(ackData.SubPoolCoinbase)
		w.Write(ackByte)
		return

	case <-time.After(time.Duration(manager.configData.ZKSubPoolUpdateAckTimeout) * time.Second):
		glog.Warning("[subpool-get] ", "timeout when waiting ACK!",
			" Coin: ", reqData.Coin, ", SubPool: ", reqData.SubPoolName)
		writeError(w, 504, "timeout when waiting ACK")
		return
	}
}

// updateCoinbaseHandle 更新子池coinbase信息
func (manager *UserChainManager) updateCoinbaseHandle(w http.ResponseWriter, req *http.Request) {
	if len(manager.configData.ZKSubPoolUpdateBaseDir) <= 0 {
		writeError(w, 403, "API disabled")
		return
	}

	requestJSON, err := ioutil.ReadAll(req.Body)

	if err != nil {
		glog.Warning(err, ": ", req.RequestURI)
		writeError(w, 500, err.Error())
		return
	}

	var reqData SubPoolUpdate
	err = json.Unmarshal(requestJSON, &reqData)

	if err != nil {
		glog.Info(err, ": ", req.RequestURI)
		writeError(w, 400, "wrong JSON, "+err.Error())
		return
	}

	if len(reqData.Coin) < 1 {
		writeError(w, 400, "coin cannot be empty")
		return
	}
	if len(reqData.SubPoolName) < 1 {
		writeError(w, 400, "subpool_name cannot be empty")
		return
	}
	if len(reqData.PayoutAddr) < 1 {
		writeError(w, 400, "payout_addr cannot be empty")
		return
	}

	glog.Info("[subpool-update] Coin: ", reqData.Coin, ", SubPool: ", reqData.SubPoolName,
		", CoinbaseInfo: ", reqData.CoinbaseInfo, ", PayoutAddr: ", reqData.PayoutAddr)

	reqNode := manager.configData.ZKSubPoolUpdateBaseDir + reqData.Coin + "/" + reqData.SubPoolName
	ackNode := reqNode + "/ack"

	exists, _, err := manager.zookeeper.Exists(reqNode)
	if err != nil || !exists {
		glog.Warning("[subpool-update] zk path '", reqNode, "' doesn't exists",
			" Coin: ", reqData.Coin, ", SubPool: ", reqData.SubPoolName)
		writeError(w, 404, "subpool '"+reqData.SubPoolName+"' does not exist")
		return
	}

	exists, _, ack, err := manager.zookeeper.ExistsW(ackNode)
	if err != nil || !exists {
		glog.Warning("[subpool-update] zk path '", ackNode, "' doesn't exists",
			" Coin: ", reqData.Coin, ", SubPool: ", reqData.SubPoolName)
		writeError(w, 503, "jobmaker cannot ACK the request")
		return
	}

	reqByte, _ := json.Marshal(reqData)
	_, err = manager.zookeeper.Set(reqNode, reqByte, -1)
	if err != nil {
		glog.Warning("[subpool-update] set zk path '", reqNode, "' failed! ", err.Error(),
			" Coin: ", reqData.Coin, ", SubPool: ", reqData.SubPoolName)
		writeError(w, 500, "write data node failed")
		return
	}

	select {
	case <-ack:
		ackJSON, _, err := manager.zookeeper.Get(ackNode)
		if err != nil {
			glog.Warning("[subpool-update] get ACK failed, ", err.Error(),
				" Coin: ", reqData.Coin, ", SubPool: ", reqData.SubPoolName)
			writeError(w, 500, "cannot get ACK from zookeeper")
			return
		}

		var ackData SubPoolUpdateAckInner
		err = json.Unmarshal(ackJSON, &ackData)
		if err != nil {
			glog.Warning("[subpool-update] parse ACK failed, ", err.Error(),
				" Coin: ", reqData.Coin, ", SubPool: ", reqData.SubPoolName)
			writeError(w, 500, "cannot parse ACK in zookeeper")
			return
		}

		if !ackData.Success && ackData.ErrNo == 0 {
			ackData.ErrNo = 500
		}

		glog.Info("[subpool-update] Response: ", ackData.ErrMsg, ", Host: ", ackData.Host.HostName,
			", Coin: ", reqData.Coin, ", SubPool: ", reqData.SubPoolName,
			", Old: ", ackData.Old, ", New: ", ackData.New)

		ackByte, _ := json.Marshal(ackData.SubPoolUpdateAck)
		w.Write(ackByte)
		return

	case <-time.After(time.Duration(manager.configData.ZKSubPoolUpdateAckTimeout) * time.Second):
		glog.Warning("[subpool-update] ", "timeout when waiting ACK!",
			" Coin: ", reqData.Coin, ", SubPool: ", reqData.SubPoolName)
		writeError(w, 504, "timeout when waiting ACK")
		return
	}
}

// switchHandle 处理币种切换请求
func (manager *UserChainManager) switchHandle(w http.ResponseWriter, req *http.Request) {
	puname := req.FormValue("puname")
	coin := req.FormValue("coin")

	oldCoin, err := manager.changeMiningCoin(puname, coin)

	if err != nil {
		glog.Info(err, ": ", req.RequestURI)
		writeError(w, err.ErrNo, err.ErrMsg)
		return
	}

	glog.Info("[single-switch] ", puname, ": ", oldCoin, " -> ", coin)
	writeSuccess(w)
}

// switchMultiUserHandle 处理多用户币种切换请求
func (manager *UserChainManager) switchMultiUserHandle(w http.ResponseWriter, req *http.Request) {
	var reqData SwitchMultiUserRequest

	requestJSON, err := ioutil.ReadAll(req.Body)

	if err != nil {
		glog.Warning(err, ": ", req.RequestURI)
		writeError(w, 500, err.Error())
		return
	}

	err = json.Unmarshal(requestJSON, &reqData)

	if err != nil {
		glog.Info(err, ": ", req.RequestURI)
		writeError(w, 400, err.Error())
		return
	}

	if len(reqData.UserCoins) <= 0 {
		glog.Info(APIErrUserCoinsEmpty.ErrMsg, ": ", req.RequestURI)
		writeError(w, APIErrUserCoinsEmpty.ErrNo, APIErrUserCoinsEmpty.ErrMsg)
		return
	}

	for _, usercoin := range reqData.UserCoins {
		coin := usercoin.Coin

		for _, puname := range usercoin.PUNames {
			oldCoin, err := manager.changeMiningCoin(puname, coin)

			if err != nil {
				glog.Info(err, ": ", req.RequestURI, " {puname=", puname, ", coin=", coin, "}")
				writeError(w, err.ErrNo, err.ErrMsg)
				return
			}

			glog.Info("[multi-switch] ", puname, ": ", oldCoin, " -> ", coin)
		}
	}

	writeSuccess(w)
}

func (manager *UserChainManager) changeMiningCoin(puname string, coin string) (oldCoin string, apiErr *APIError) {
	if len(puname) < 1 {
		apiErr = APIErrPunameIsEmpty
		return
	}
	if strings.Contains(puname, "/") {
		apiErr = APIErrPunameInvalid
		return
	}
	if len(coin) < 1 {
		apiErr = APIErrCoinIsEmpty
		return
	}

	// 检查币种是否存在
	exists := false
	for _, availableCoin := range manager.configData.AvailableCoins {
		if availableCoin == coin {
			exists = true
			break
		}
	}
	if !exists {
		apiErr = APIErrCoinIsInexistent
		return
	}

	puname = manager.RegularUserName(puname)
	oldCoin = manager.GetChain(puname)
	manager.SetChain(puname, coin)
	err := manager.WriteToZK(puname)
	if err != nil {
		glog.Error("WriteToZK(", puname, ") failed: ", err)
		apiErr = APIErrWriteRecordFailed
		return
	}

	apiErr = nil
	return
}
