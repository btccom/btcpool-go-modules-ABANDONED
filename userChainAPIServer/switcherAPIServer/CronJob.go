package main

import (
	"encoding/json"
	"io/ioutil"
	"net/http"
	"strconv"
	"time"

	"github.com/golang/glog"
)

// UserCoinMapData 用户币种列表接口响应的data字段数据结构
type UserCoinMapData struct {
	UserCoin map[string]string `json:"user_coin"`
	NowDate  int64             `json:"now_date"`
}

// UserCoinMapResponse 用户币种列表接口响应的数据结构
type UserCoinMapResponse struct {
	ErrNo  int             `json:"err_no"`
	ErrMsg string          `json:"err_msg"`
	Data   UserCoinMapData `json:"data"`
}

// RunCronJob 运行定时检测任务
func RunCronJob() {
	defer waitGroup.Done()

	// 上次请求接口的时间
	var lastRequestDate int64

	for true {
		// 执行操作
		// 定义在函数中，这样失败时可以简单的return并进入休眠
		func() {

			url := configData.UserCoinMapURL
			// 若上次请求过接口，则附加上次请求的时间到url
			if lastRequestDate > 0 {
				url += "?last_date=" + strconv.FormatInt(lastRequestDate, 10)
			}
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

			userCoinMapResponse := new(UserCoinMapResponse)

			err = json.Unmarshal(body, userCoinMapResponse)

			if err != nil {
				glog.Error("Parse Result Failed: ", err, "; ", string(body))
				return
			}

			if userCoinMapResponse.ErrNo != 0 {
				glog.Error("API Returned a Error: ", string(body))
				return
			}

			// 记录本次请求的时间
			lastRequestDate = userCoinMapResponse.Data.NowDate

			glog.Info("HTTP GET Success. TimeStamp: ", userCoinMapResponse.Data.NowDate, "; UserCoin Num: ", len(userCoinMapResponse.Data.UserCoin))

			// 遍历用户币种列表
			for puname, coin := range userCoinMapResponse.Data.UserCoin {
				oldCoin, err := changeMiningCoin(puname, coin)

				if err != nil {
					glog.Info(err.ErrMsg, ": ", puname, ": ", oldCoin, " -> ", coin)
				} else {
					glog.Info("success: ", puname, ": ", oldCoin, " -> ", coin)
				}
			}
		}()

		// 休眠
		time.Sleep(time.Duration(configData.CronIntervalSeconds) * time.Second)
	}
}
