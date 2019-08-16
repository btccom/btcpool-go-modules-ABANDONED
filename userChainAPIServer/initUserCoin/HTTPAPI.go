package initusercoin

// #cgo CXXFLAGS: -std=c++11
// #include "UserListJSON.h"
import "C"

import (
	"net/http"
	"strconv"

	"github.com/golang/glog"
)

// HTTPRequestHandle HTTP请求处理函数
type HTTPRequestHandle func(http.ResponseWriter, *http.Request)

// 启动 API Server
func runAPIServer() {
	defer waitGroup.Done()

	// HTTP监听
	glog.Info("Listen HTTP ", configData.ListenAddr)

	http.HandleFunc("/", getUserIDList)

	err := http.ListenAndServe(configData.ListenAddr, nil)

	if err != nil {
		glog.Fatal("HTTP Listen Failed: ", err)
		return
	}
}

// getUserIDList 获取子账户列表
func getUserIDList(w http.ResponseWriter, req *http.Request) {
	lastIDStr := req.FormValue("last_id")
	lastID, _ := strconv.Atoi(lastIDStr)

	json := C.GoString(C.getUserListJson(C.int(lastID)))
	w.Write([]byte(json))
}
