package main

import (
	"encoding/json"
	"flag"
	"io/ioutil"
	"net/http"
	"strings"
	"time"

	"github.com/golang/glog"
	"github.com/samuel/go-zookeeper/zk"
)

// ConfigData 配置数据
type ConfigData struct {
	ListenAddr string
	// AvailableCoins 可用币种，形如 {"btc", "bcc", ...}
	AvailableCoins []string
	ZKBroker       []string
	// ZKSwitcherWatchDir Switcher监控的Zookeeper路径，以斜杠结尾
	ZKSwitcherWatchDir string
}

// zookeeperConn Zookeeper连接对象
var zookeeperConn *zk.Conn

// 配置数据
var configData *ConfigData

func main() {
	// 解析命令行参数
	configFilePath := flag.String("config", "./config.json", "Path of config file")
	flag.Parse()

	// 读取配置文件
	configJSON, err := ioutil.ReadFile(*configFilePath)

	if err != nil {
		glog.Fatal("read config failed: ", err)
		return
	}

	configData = new(ConfigData)
	err = json.Unmarshal(configJSON, configData)

	if err != nil {
		glog.Fatal("parse config failed: ", err)
		return
	}

	// 若zookeeper路径不以“/”结尾，则添加
	if configData.ZKSwitcherWatchDir[len(configData.ZKSwitcherWatchDir)-1] != '/' {
		configData.ZKSwitcherWatchDir += "/"
	}

	// 建立到Zookeeper集群的连接
	conn, _, err := zk.Connect(configData.ZKBroker, time.Second)

	if err != nil {
		glog.Fatal("Connect Zookeeper Failed: ", err)
		return
	}

	zookeeperConn = conn

	// 检查并创建StratumSwitcher使用的Zookeeper路径
	err = createZookeeperPath(configData.ZKSwitcherWatchDir)

	if err != nil {
		glog.Fatal("Create Zookeeper Path Failed: ", err)
		return
	}

	// HTTP监听
	glog.Info("Listen TCP ", configData.ListenAddr)

	http.HandleFunc("/switch", switchHandle)
	err = http.ListenAndServe(configData.ListenAddr, nil)

	if err != nil {
		glog.Fatal("HTTP Listen Failed: ", err)
		return
	}
}

// 递归创建Zookeeper Node
func createZookeeperPath(path string) error {
	pathTrimmed := strings.Trim(path, "/")
	dirs := strings.Split(pathTrimmed, "/")

	currPath := ""

	for _, dir := range dirs {
		currPath += "/" + dir

		// 看看键是否存在
		exists, _, err := zookeeperConn.Exists(currPath)

		if err != nil {
			return err
		}

		// 已存在，不需要创建
		if exists {
			continue
		}

		// 不存在，创建
		_, err = zookeeperConn.Create(currPath, []byte{}, 0, zk.WorldACL(zk.PermAll))

		if err != nil {
			return err
		}

		glog.Info("Created zookeeper path: ", currPath)
	}

	return nil
}

// APIResponse API响应数据结构
type APIResponse struct {
	ErrNo   int    `json:"err_no"`
	ErrMsg  string `json:"err_msg"`
	Success bool   `json:"success"`
}

// switchHandle 处理币种切换请求
func switchHandle(w http.ResponseWriter, req *http.Request) {
	params := req.URL.Query()

	puname := params.Get("puname")
	coin := params.Get("coin")
	oldCoin := ""

	if len(puname) < 1 {
		glog.Info("puname is empty: ", req.RequestURI)
		writeError(w, 101, "puname is empty")
		return
	}

	if strings.Contains(puname, "/") {
		glog.Info("puname invalid: ", req.RequestURI)
		writeError(w, 102, "puname invalid")
		return
	}

	if len(coin) < 1 {
		glog.Info("coin is empty: ", req.RequestURI)
		writeError(w, 103, "coin is empty")
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
		glog.Info("coin is inexistent: ", req.RequestURI)
		writeError(w, 104, "coin is inexistent")
		return
	}

	// stratumSwitcher 监控的键
	zkPath := configData.ZKSwitcherWatchDir + puname

	// 看看键是否存在
	exists, _, err := zookeeperConn.Exists(zkPath)

	if err != nil {
		glog.Info("read zookeeper failed: ", req.RequestURI, "; ", err)
		writeError(w, 105, "read record failed")
		return
	}

	if exists {
		// 读取zookeeper看看原来的值是多少
		oldCoinData, _, err := zookeeperConn.Get(zkPath)

		if err != nil {
			glog.Info("read zookeeper failed: ", req.RequestURI, "; ", err)
			writeError(w, 106, "read record failed")
			return
		}

		oldCoin = string(oldCoinData)

		// 没有改变
		if oldCoin == coin {
			glog.Info("no change: ", req.RequestURI)
			writeError(w, 107, "no change")
			return
		}

		// 写入新值
		_, err = zookeeperConn.Set(zkPath, []byte(coin), -1)

		if err != nil {
			glog.Info("write zookeeper node failed: ", req.RequestURI, "; ", err)
			writeError(w, 107, "write record failed")
			return
		}

	} else {
		// 不存在，直接创建
		_, err = zookeeperConn.Create(zkPath, []byte(coin), 0, zk.WorldACL(zk.PermAll))

		if err != nil {
			glog.Info("create zookeeper node failed: ", req.RequestURI, "; ", err)
			writeError(w, 107, "write record failed")
			return
		}
	}

	glog.Info("success: ", puname, ": ", oldCoin, " -> ", coin)
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
