package main

import (
	"strings"

	"github.com/golang/glog"
	"github.com/samuel/go-zookeeper/zk"
)

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
