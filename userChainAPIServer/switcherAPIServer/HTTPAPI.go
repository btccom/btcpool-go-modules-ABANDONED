package switcherAPIServer

import (
	"crypto/subtle"
	"encoding/json"
	"io/ioutil"
	"net/http"
	"strings"

	"github.com/golang/glog"
	"github.com/samuel/go-zookeeper/zk"
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

// HTTPRequestHandle HTTP请求处理函数
type HTTPRequestHandle func(http.ResponseWriter, *http.Request)

// 启动 API Server
func runAPIServer() {
	defer waitGroup.Done()

	// HTTP监听
	glog.Info("Listen HTTP ", configData.ListenAddr)

	http.HandleFunc("/switch", basicAuth(switchHandle))
	http.HandleFunc("/switch-multi-user", basicAuth(switchMultiUserHandle))

	err := http.ListenAndServe(configData.ListenAddr, nil)

	if err != nil {
		glog.Fatal("HTTP Listen Failed: ", err)
		return
	}
}

// basicAuth 执行Basic认证
func basicAuth(f HTTPRequestHandle) HTTPRequestHandle {
	return func(w http.ResponseWriter, r *http.Request) {
		apiUser := []byte(configData.APIUser)
		apiPasswd := []byte(configData.APIPassword)

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

// switchHandle 处理币种切换请求
func switchHandle(w http.ResponseWriter, req *http.Request) {
	puname := req.FormValue("puname")
	coin := req.FormValue("coin")

	oldCoin, err := changeMiningCoin(puname, coin)

	if err != nil {
		glog.Info(err, ": ", req.RequestURI)
		writeError(w, err.ErrNo, err.ErrMsg)
		return
	}

	glog.Info("[single-switch] ", puname, ": ", oldCoin, " -> ", coin)
	writeSuccess(w)
}

// switchMultiUserHandle 处理多用户币种切换请求
func switchMultiUserHandle(w http.ResponseWriter, req *http.Request) {
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

	if len(reqData.UserCoins) == 0 {
		glog.Info(APIErrUserCoinsEmpty.ErrMsg, ": ", req.RequestURI)
		writeError(w, APIErrUserCoinsEmpty.ErrNo, APIErrUserCoinsEmpty.ErrMsg)
		return
	}

	for _, usercoin := range reqData.UserCoins {
		coin := usercoin.Coin

		for _, puname := range usercoin.PUNames {
			oldCoin, err := changeMiningCoin(puname, coin)

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

func changeMiningCoin(puname string, coin string) (oldCoin string, apiErr *APIError) {
	oldCoin = ""

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

	for _, availableCoin := range configData.AvailableCoins {
		if availableCoin == coin {
			exists = true
			break
		}
	}

	if !exists {
		apiErr = APIErrCoinIsInexistent
		return
	}

	if configData.StratumServerCaseInsensitive {
		// stratum server对子账户名大小写不敏感
		// 简单的将子账户名转换为小写即可
		puname = strings.ToLower(puname)
	}

	// stratumSwitcher 监控的键
	zkPath := configData.ZKSwitcherWatchDir + puname

	// 看看键是否存在
	exists, _, err := zookeeperConn.Exists(zkPath)

	if err != nil {
		glog.Error("zk.Exists(", zkPath, ") Failed: ", err)
		apiErr = APIErrReadRecordFailed
		return
	}

	if exists {
		// 读取zookeeper看看原来的值是多少
		oldCoinData, _, err := zookeeperConn.Get(zkPath)

		if err != nil {
			glog.Error("zk.Get(", zkPath, ") Failed: ", err)
			apiErr = APIErrReadRecordFailed
			return
		}

		oldCoin = string(oldCoinData)

		// 没有改变
		// 没有改变不再返回错误，这样一来，如果stratumSwitcher错过了前一个切换消息，可以再收到一次切换消息以完成切换
		// 在stratumSwitcher那里，如果币种确实没有发生改变，切换就不会发生
		/*if oldCoin == coin {
			apiErr = APIErrCoinNoChange
			return
		}*/

		// 写入新值
		_, err = zookeeperConn.Set(zkPath, []byte(coin), -1)

		if err != nil {
			glog.Error("zk.Set(", zkPath, ",", coin, ") Failed: ", err)
			apiErr = APIErrWriteRecordFailed
			return
		}

	} else {
		// 不存在，直接创建
		_, err = zookeeperConn.Create(zkPath, []byte(coin), 0, zk.WorldACL(zk.PermAll))

		if err != nil {
			glog.Error("zk.Create(", zkPath, ",", coin, ") Failed: ", err)
			apiErr = APIErrWriteRecordFailed
			return
		}
	}

	apiErr = nil
	return
}
