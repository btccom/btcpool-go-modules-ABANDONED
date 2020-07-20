package initusercoin

// #cgo CXXFLAGS: -std=c++11
// #include "UserListJSON.h"
import "C"

import (
	"net/http"
	"strconv"
	"unsafe"

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
	coin := req.FormValue("coin")
	lastIDStr := req.FormValue("last_id")
	lastID, _ := strconv.Atoi(lastIDStr)

	coinC := C.CString(coin)
	json := C.GoString(C.getUserListJson(C.int(lastID), coinC))
	C.free(unsafe.Pointer(coinC))
	w.Write([]byte(json))
}

// GetUserUpdateTime 获取用户的更新时间（即进入列表的时间）
func GetUserUpdateTime(puname string, coin string) int64 {
	punameC := C.CString(puname)
	coinC := C.CString(coin)
	defer C.free(unsafe.Pointer(punameC))
	defer C.free(unsafe.Pointer(coinC))
	return int64(C.getUserUpdateTime(punameC, coinC))
}

// GetSafetyPeriod 获取用户更新的安全期（在安全期内，子账户可能尚未进入sserver的缓存）
func GetSafetyPeriod() int64 {
	return int64(configData.IntervalSeconds * 15 / 10)
}
