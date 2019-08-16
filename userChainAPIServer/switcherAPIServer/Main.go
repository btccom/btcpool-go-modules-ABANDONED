package switcherAPIServer

import (
	"encoding/json"
	"io/ioutil"
	"sync"
	"time"

	"github.com/golang/glog"
	"github.com/samuel/go-zookeeper/zk"
)

// Zookeeper连接超时时间
const zookeeperConnTimeout = 5

// ConfigData 配置数据
type ConfigData struct {
	// 是否启用 API Server
	EnableAPIServer bool
	// API 用户名
	APIUser string
	// API 密码
	APIPassword string
	// API Server 的监听IP:端口
	ListenAddr string

	// AvailableCoins 可用币种，形如 {"btc", "bcc", ...}
	AvailableCoins []string

	// Zookeeper集群的IP:端口列表
	ZKBroker []string
	// ZKSwitcherWatchDir Switcher监控的Zookeeper路径，以斜杠结尾
	ZKSwitcherWatchDir string

	// 是否启用定时检测任务
	EnableCronJob bool
	// 定时检测间隔时间
	CronIntervalSeconds int
	// 用户:币种对应表的URL
	UserCoinMapURL string
	// 挖矿服务器对子账户名大小写不敏感，此时将总是写入小写的子账户名
	StratumServerCaseInsensitive bool
}

// zookeeperConn Zookeeper连接对象
var zookeeperConn *zk.Conn

// 配置数据
var configData *ConfigData

// 用于等待goroutine结束
var waitGroup sync.WaitGroup

// Main function
func Main(configFilePath string) {
	// 读取配置文件
	configJSON, err := ioutil.ReadFile(configFilePath)

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
	conn, _, err := zk.Connect(configData.ZKBroker, time.Duration(zookeeperConnTimeout)*time.Second)

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

	if configData.EnableAPIServer {
		waitGroup.Add(1)
		go runAPIServer()
	}

	if configData.EnableCronJob {
		waitGroup.Add(1)
		go RunCronJob()
	}

	waitGroup.Wait()
}
