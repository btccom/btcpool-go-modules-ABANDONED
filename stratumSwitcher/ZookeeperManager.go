package main

import (
	"errors"
	"strings"
	"sync"
	"time"

	"github.com/golang/glog"
	"github.com/samuel/go-zookeeper/zk"
)

// zookeeper连接超时时间
const zookeeperConnectingTimeoutSeconds = 60

// Zookeeper连接失活超时时间
const zookeeperConnAliveTimeout = 5

// NodeWatcherChannels 节点监控者的channel
type NodeWatcherChannels map[uint32]chan zk.Event

// NodeWatcher 节点监控器
type NodeWatcher struct {
	// Zookeeper管理器
	zookeeperManager *ZookeeperManager
	// 被监控节点的路径
	nodePath string
	// 被监控节点的当前值
	nodeValue []byte
	// 被监控的Zookeeper事件
	zkWatchEvent <-chan zk.Event
	// 节点监控者的channel
	watcherChannels NodeWatcherChannels
}

// NewNodeWatcher 新建节点监控器
func NewNodeWatcher(zookeeperManager *ZookeeperManager) *NodeWatcher {
	watcher := new(NodeWatcher)
	watcher.zookeeperManager = zookeeperManager
	watcher.watcherChannels = make(NodeWatcherChannels)
	return watcher
}

// Run 开始监控
func (watcher *NodeWatcher) Run() {
	go func() {
		event := <-watcher.zkWatchEvent

		watcher.zookeeperManager.lock.Lock()
		defer watcher.zookeeperManager.lock.Unlock()

		for _, eventChan := range watcher.watcherChannels {
			eventChan <- event
			close(eventChan)
		}

		watcher.zookeeperManager.removeNodeWatcher(watcher)
	}()
}

// NodeWatcherMap Zookeeper监控器Map
type NodeWatcherMap map[string]*NodeWatcher

// ZookeeperManager Zookeeper管理器
type ZookeeperManager struct {
	// 修改 watcherMap 时加的锁
	lock sync.Mutex
	// 监控器Map
	watcherMap NodeWatcherMap
	// Zookeeper连接
	zookeeperConn *zk.Conn
}

// NewZookeeperManager 新建Zookeeper管理器
func NewZookeeperManager(brokers []string) (manager *ZookeeperManager, err error) {
	manager = new(ZookeeperManager)
	manager.watcherMap = make(NodeWatcherMap)

	// 建立到Zookeeper集群的连接
	var event <-chan zk.Event
	manager.zookeeperConn, event, err = zk.Connect(brokers, time.Duration(zookeeperConnAliveTimeout)*time.Second)
	if err != nil {
		return
	}

	zkConnected := make(chan bool, 1)

	go func() {
		glog.Info("Zookeeper: waiting for connecting to ", brokers, "...")
		for {
			e := <-event
			glog.Info("Zookeeper: ", e)

			if e.State == zk.StateConnected {
				zkConnected <- true
				return
			}
		}
	}()

	select {
	case <-zkConnected:
		break
	case <-time.After(zookeeperConnectingTimeoutSeconds * time.Second):
		err = errors.New("Zookeeper: connecting timeout")
		break
	}

	return
}

// removeNodeWatcher 移除监控节点
func (manager *ZookeeperManager) removeNodeWatcher(watcher *NodeWatcher) {
	delete(manager.watcherMap, watcher.nodePath)
	if glog.V(3) {
		glog.Info("Zookeeper: release NodeWatcher: ", watcher.nodePath)
	}
}

// GetW 获取Zookeeper节点的值并设置监控
func (manager *ZookeeperManager) GetW(path string, sessionID uint32) (value []byte, event <-chan zk.Event, err error) {
	manager.lock.Lock()
	defer manager.lock.Unlock()

	watcher, exists := manager.watcherMap[path]

	if !exists {
		watcher = NewNodeWatcher(manager)
		watcher.nodePath = path
		watcher.nodeValue, _, watcher.zkWatchEvent, err = manager.zookeeperConn.GetW(path)

		if err != nil {
			return
		}

		manager.watcherMap[path] = watcher
		if glog.V(3) {
			glog.Info("Zookeeper: add NodeWatcher: ", path)
		}

		defer watcher.Run()
	}

	eventChan := make(chan zk.Event, 1)
	watcher.watcherChannels[sessionID] = eventChan
	if glog.V(3) {
		glog.Info("Zookeeper: add WatcherChannel: ", path, "; ", Uint32ToHex(sessionID))
	}

	value = watcher.nodeValue
	event = eventChan
	return
}

// Create 创建Zookeeper节点
func (manager *ZookeeperManager) Create(path string, data []byte) (err error) {
	_, err = manager.zookeeperConn.Create(path, data, 0, zk.WorldACL(zk.PermAll))
	return
}

// ReleaseW 释放监控
func (manager *ZookeeperManager) ReleaseW(path string, sessionID uint32) {
	manager.lock.Lock()
	defer manager.lock.Unlock()

	watcher, exists := manager.watcherMap[path]

	if !exists {
		return
	}

	eventChan, exists := watcher.watcherChannels[sessionID]

	if !exists {
		return
	}

	close(eventChan)
	delete(watcher.watcherChannels, sessionID)
	if glog.V(3) {
		glog.Info("Zookeeper: release WatcherChannel: ", path, "; ", Uint32ToHex(sessionID))
	}

	// go-zookeeper 的代码显示，它的watcher只会在接收到事件后关闭并释放，
	// 因此，在此处移除 NodaWatcher 并不能使 go-zookeeper 中的 watcher 释放，
	// 并且，反复打开新 watcher 反而会导致 go-zookeeper 处生成大量 watcher 而内存泄露。
	// 因此，此处不再自动释放 NodeWatcher。NodeWatcher 只在接收到 zookeeper 事件后释放。
	/*
		if len(watcher.watcherChannels) == 0 {
			manager.removeNodeWatcher(watcher)
		}
	*/
}

// 递归创建Zookeeper Node
func (manager *ZookeeperManager) createZookeeperPath(path string) error {
	pathTrimmed := strings.Trim(path, "/")
	dirs := strings.Split(pathTrimmed, "/")

	currPath := ""

	for _, dir := range dirs {
		currPath += "/" + dir

		// 看看键是否存在
		exists, _, err := manager.zookeeperConn.Exists(currPath)

		if err != nil {
			return err
		}

		// 已存在，不需要创建
		if exists {
			continue
		}

		// 不存在，创建
		_, err = manager.zookeeperConn.Create(currPath, []byte{}, 0, zk.WorldACL(zk.PermAll))

		if err != nil {
			return err
		}

		glog.Info("Created zookeeper path: ", currPath)
	}

	return nil
}
