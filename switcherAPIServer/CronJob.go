package main

import (
	"encoding/json"
	"io/ioutil"
	"net/http"
	"time"

	"github.com/golang/glog"
)

// UserCoinMapResponse 用户币种列表接口的返回结果
type UserCoinMapResponse struct {
	ErrNo  int               `json:"err_no"`
	ErrMsg string            `json:"err_msg"`
	Data   map[string]string `json:"data"`
}

// RunCronJob 运行定时检测任务
func RunCronJob() {
	defer waitGroup.Done()

	for true {
		// 执行操作
		// 定义在函数中，这样失败时可以简单的return并进入休眠
		func() {
			response, err := http.Get(configData.UserCoinMapURL)

			if err != nil {
				glog.Error("HTTP Request Failed: ", err)
				return
			}

			body, err := ioutil.ReadAll(response.Body)

			if err != nil {
				glog.Error("HTTP Fetch Body Failed: ", err)
				return
			}

			userCoinMapData := new(UserCoinMapResponse)

			err = json.Unmarshal(body, userCoinMapData)

			if err != nil {
				glog.Error("Parse Result Failed: ", err, "; ", string(body))
				return
			}

			if userCoinMapData.ErrNo != 0 {
				glog.Error("API Returned a Error: ", string(body))
				return
			}

			for puname, coin := range userCoinMapData.Data {
				oldCoin, err := changeMiningCoin(puname, coin)

				if err != nil {
					// 以较低优先级记录币种未改变日志
					if err == APIErrCoinNoChange {
						glog.V(3).Info(err.ErrMsg, ": ", puname, ": ", oldCoin, " -> ", coin)
					} else {
						glog.Info(err.ErrMsg, ": ", puname, ": ", oldCoin, " -> ", coin)
					}
				} else {
					glog.Info("success: ", puname, ": ", oldCoin, " -> ", coin)
				}
			}
		}()

		// 休眠
		time.Sleep(time.Duration(configData.CronIntervalSeconds) * time.Second)
	}
}
