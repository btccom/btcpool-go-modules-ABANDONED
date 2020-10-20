package main

import (
	"strings"
	"time"

	"github.com/golang/glog"
	"github.com/samuel/go-zookeeper/zk"
)

// Zookeeper连接超时时间
const zookeeperConnTimeout = 5

// Zookeeper 实现Zookeeper操作
type Zookeeper struct {
	*zk.Conn
}

// NewZookeeper 创建Zookeeper对象
func NewZookeeper(configData *ConfigData) (zookeeper *Zookeeper, err error) {
	zookeeper = new(Zookeeper)

	// 建立到Zookeeper集群的连接
	zookeeper.Conn, _, err = zk.Connect(configData.ZKBroker, time.Duration(zookeeperConnTimeout)*time.Second)
	if err != nil {
		return
	}

	// 检查并创建会用到的Zookeeper路径
	err = zookeeper.CreatePath(configData.ZKSwitcherWatchDir)
	if err != nil {
		return
	}

	if configData.EnableUserAutoReg {
		err = zookeeper.CreatePath(configData.ZKAutoRegWatchDir)
		if err != nil {
			return
		}
	}

	if len(configData.ZKSubPoolUpdateBaseDir) > 0 {
		err = zookeeper.CreatePath(configData.ZKSubPoolUpdateBaseDir)
		if err != nil {
			return
		}
	}

	// 无错返回
	return
}

// CreatePath 递归创建Zookeeper节点
func (zookeeper *Zookeeper) CreatePath(path string) error {
	pathTrimmed := strings.Trim(path, "/")
	dirs := strings.Split(pathTrimmed, "/")

	currPath := ""

	for _, dir := range dirs {
		currPath += "/" + dir

		// 看看键是否存在
		exists, _, err := zookeeper.Exists(currPath)

		if err != nil {
			return err
		}

		// 已存在，不需要创建
		if exists {
			continue
		}

		// 不存在，创建
		_, err = zookeeper.Create(currPath, []byte{}, 0, zk.WorldACL(zk.PermAll))

		if err != nil {
			// 再看看键是否存在（键可能已被其他线程创建）
			exists, _, _ = zookeeper.Exists(currPath)
			if exists {
				continue
			}
			// 键不存在，返回错误
			return err
		}

		glog.Info("Created zookeeper path: ", currPath)
	}

	return nil
}
