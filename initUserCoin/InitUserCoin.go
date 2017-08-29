package main

import (
	"encoding/json"
	"io/ioutil"
	"net/http"
	"strings"

	"github.com/golang/glog"
	"github.com/samuel/go-zookeeper/zk"
)

// UserIDMapResponse 用户id列表接口响应的数据结构
type UserIDMapResponse struct {
	ErrNo  int            `json:"err_no"`
	ErrMsg string         `json:"err_msg"`
	Data   map[string]int `json:"data"`
}

// InitUserCoin 拉取用户id列表来初始化用户币种记录
func InitUserCoin(coin string, url string) {
	defer waitGroup.Done()

	glog.Info("HTTP GET ", url)
	response, err := http.Get(url)

	if err != nil {
		glog.Error("HTTP Request Failed: ", err)
		return
	}

	body, err := ioutil.ReadAll(response.Body)

	if err != nil {
		glog.Error("HTTP Fetch Body Failed: ", err)
		return
	}

	userIDMapResponse := new(UserIDMapResponse)

	err = json.Unmarshal(body, userIDMapResponse)

	if err != nil {
		glog.Error("Parse Result Failed: ", err, "; ", string(body))
		return
	}

	if userIDMapResponse.ErrNo != 0 {
		glog.Error("API Returned a Error: ", string(body))
		return
	}

	glog.Info("HTTP GET Success. User Num: ", len(userIDMapResponse.Data))

	// 遍历用户币种列表
	for puname := range userIDMapResponse.Data {
		err := setMiningCoin(puname, coin)

		if err != nil {
			glog.Info(err.ErrMsg, ": ", puname, ": ", coin)
		} else {
			glog.Info("success: ", puname, ": ", coin)
		}
	}

	glog.Info("Finish: ", coin, "; ", url)
}

func setMiningCoin(puname string, coin string) (apiErr *APIError) {

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

	for availableCoin := range configData.UserListAPI {
		if availableCoin == coin {
			exists = true
			break
		}
	}

	if !exists {
		apiErr = APIErrCoinIsInexistent
		return
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
