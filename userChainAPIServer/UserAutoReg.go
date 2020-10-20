package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/golang/glog"
)

// RunUserAutoReg 运行自动注册任务
func (manager *UserChainManager) RunUserAutoReg(waitGroup *sync.WaitGroup) {
	defer waitGroup.Done()

	config := manager.configData

	zkWatchDir := config.ZKAutoRegWatchDir[0 : len(config.ZKAutoRegWatchDir)-1] // 移除结尾的"/"
	glog.Info("UserAutoReg watch in zk: ", zkWatchDir)

	for {
		users, _, eventPool, err := manager.zookeeper.ChildrenW(zkWatchDir)

		if err != nil {
			glog.Error("zookeeper ChildrenW failed: ", err)
			time.Sleep(config.UserAutoRegAPI.IntervalSeconds * time.Second)
			continue
		}

		if len(users) > 0 {
			for _, user := range users {
				manager.regUser(user, config)
			}
		} else {
			<-eventPool
		}
	}
}

func (manager *UserChainManager) regUser(user string, config *ConfigData) {
	path := config.ZKAutoRegWatchDir + user

	// 返回时将通过删除zk节点来唤醒发起自动注册的switcher
	defer manager.zookeeper.Delete(path, 0)

	info, _, _ := manager.zookeeper.Get(path)
	glog.Info("reg user: ", user, ", info: ", string(info))

	// 构建要提交的内容
	postData := make(map[string]string)
	for key, value := range config.UserAutoRegAPI.PostData {
		postData[key] = strings.Replace(value, "{sub_name}", user, -1)
	}

	responseBytes, err := HTTPPost(config.UserAutoRegAPI, postData)
	if err != nil {
		glog.Warning("reg user failed. user: ", user, ", errmsg: ", err)
		return
	}

	type apiData struct {
		PUID int32 `json:"puid"`
	}

	type apiResponse struct {
		Data    apiData `json:"data"`
		Status  string  `json:"status"`
		Message string  `json:"message"`
	}

	var response apiResponse

	err = json.Unmarshal(responseBytes, &response)
	if err != nil {
		glog.Warning("reg user failed. user: ", user, ", errmsg: ", err, ", response: ", string(responseBytes))
		return
	}

	if response.Data.PUID <= 0 {
		glog.Warning("reg user failed. user: ", user, ", puid: ", response.Data.PUID,
			", coin: ", config.UserAutoRegAPI.DefaultCoin,
			", status: ", response.Status, ", message: ", response.Message)
		return
	}

	glog.Info("reg user success. user: ", user, ", puid: ", response.Data.PUID,
		", coin: ", config.UserAutoRegAPI.DefaultCoin,
		", status: ", response.Status, ", message: ", response.Message)

	// 注册成功，更新用户记录
	user = manager.RegularUserName(user)
	manager.SetPUID(user, config.UserAutoRegAPI.DefaultCoin, response.Data.PUID)
	err = manager.WriteToZK(user)

	if err != nil {
		glog.Error("WriteToZK(", user, ") failed: ", err)
	}
}

// HTTPPost 调用HTTP Post方法
func HTTPPost(api AutoRegAPIConfig, data interface{}) (response []byte, err error) {

	// encode request to buffer
	bufSend := &bytes.Buffer{}
	enc := json.NewEncoder(bufSend)
	err = enc.Encode(data)
	if err != nil {
		err = fmt.Errorf("Error when encoding json: %s", err)
		return
	}

	// create http request
	req, err := http.NewRequest("POST", api.URL, bufSend)
	if err != nil {
		err = fmt.Errorf("Error when creating new http request: %s", err)
		return
	}
	req.Header.Set("Content-Type", "application/json")
	if len(api.User) > 0 {
		req.SetBasicAuth(api.User, api.Password)
	}

	// do request
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		err = fmt.Errorf("Error when performing http request: %s", err)
		return
	}
	defer resp.Body.Close()

	// get response
	response, err = ioutil.ReadAll(resp.Body)
	if err != nil {
		err = fmt.Errorf("Error when reading http response: %s", err)
		return
	}

	// no error, finished
	return
}
