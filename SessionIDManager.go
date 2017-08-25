package main

import (
	"sync"

	"github.com/willf/bitset"
)

//////////////////////////////// SessionIDManager //////////////////////////////

// MaxServerSessionIndex 单个服务器的会话ID最大编号（不要改动）
const MaxServerSessionIndex uint32 = 0x00FFFFFE // 16777214

// SessionIDManager 线程安全的会话ID管理器
type SessionIDManager struct {
	//
	//  SESSION ID: UINT32_T
	//
	//   xxxxxxxx     xxxxxxxx xxxxxxxx xxxxxxxx
	//  ----------    --------------------------
	//  server ID          session id
	//   [1, 255]        range: [0, MaxServerSessionIndex]
	//
	serverID   uint8
	sessionIds *bitset.BitSet
	//std::bitset<MAX_SESSION_INDEX_SERVER + 1> sessionIds

	count    int32 // how many ids are used now
	allocIdx uint32
	lock     sync.Mutex
}

// NewSessionIDManager 创建一个会话ID管理器实例
/*func NewSessionIDManager() SessionIDManager {
	sessionIDManager SessionIDManager

}*/
