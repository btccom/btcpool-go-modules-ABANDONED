package main

import (
	"sync"

	"github.com/willf/bitset"
)

//////////////////////////////// SessionIDManager //////////////////////////////

// SessionIDMask 会话ID掩码，用于分离serverID和sessionID
// 也是sessionID部分可以达到的最大数值
const SessionIDMask uint32 = 0x00FFFFFF // 16777215

// MaxValidSessionID 最大的合法sessionID
const MaxValidSessionID uint32 = SessionIDMask - 1 // 16777214

// SessionIDManager 线程安全的会话ID管理器
type SessionIDManager struct {
	//
	//  SESSION ID: UINT32
	//
	//   xxxxxxxx     xxxxxxxx xxxxxxxx xxxxxxxx
	//  ----------    --------------------------
	//  server ID          session id
	//   [1, 255]        range: [0, MaxValidSessionID]
	//
	serverID   uint8
	sessionIDs *bitset.BitSet

	count    uint32 // how many ids are used now
	allocIDx uint32
	lock     sync.Mutex
}

// NewSessionIDManager 创建一个会话ID管理器实例
func NewSessionIDManager(serverID uint8) *SessionIDManager {
	manager := new(SessionIDManager)

	manager.serverID = serverID
	manager.sessionIDs = bitset.New(uint(SessionIDMask))
	manager.count = 0
	manager.allocIDx = 0

	manager.sessionIDs.ClearAll()

	return manager
}

// isFull 判断会话ID是否已满（内部使用，不加锁）
func (manager *SessionIDManager) isFullWithoutLock() bool {
	return (manager.count >= SessionIDMask)
}

// IsFull 判断会话ID是否已满
func (manager *SessionIDManager) IsFull() bool {
	defer manager.lock.Unlock()
	manager.lock.Lock()

	return manager.isFullWithoutLock()
}

// AllocSessionID 为调用者分配一个会话ID
func (manager *SessionIDManager) AllocSessionID() (sessionID uint32, err error) {
	defer manager.lock.Unlock()
	manager.lock.Lock()

	if manager.isFullWithoutLock() {
		sessionID = SessionIDMask
		err = ErrSessionIDFull
		return
	}

	// find an empty bit
	for manager.sessionIDs.Test(uint(manager.allocIDx)) {
		manager.allocIDx++
		if manager.allocIDx > MaxValidSessionID {
			manager.allocIDx = 0
		}
	}

	// set to true
	manager.sessionIDs.Set(uint(manager.allocIDx))
	manager.count++

	sessionID = (uint32(manager.serverID) << 24) | manager.allocIDx
	err = nil
	return
}

// ResumeSessionID 恢复之前的会话ID
func (manager *SessionIDManager) ResumeSessionID(sessionID uint32) (err error) {
	defer manager.lock.Unlock()
	manager.lock.Lock()

	idx := sessionID & SessionIDMask

	// test if the bit be empty
	if manager.sessionIDs.Test(uint(idx)) {
		err = ErrSessionIDOccupied
		return
	}

	// set to true
	manager.sessionIDs.Set(uint(idx))
	manager.count++

	if manager.allocIDx <= idx {
		manager.allocIDx = idx + 1
	}

	err = nil
	return
}

// FreeSessionID 释放调用者持有的会话ID
func (manager *SessionIDManager) FreeSessionID(sessionID uint32) {
	defer manager.lock.Unlock()
	manager.lock.Lock()

	idx := sessionID & SessionIDMask

	if !manager.sessionIDs.Test(uint(idx)) {
		// ID未分配，无需释放
		return
	}

	manager.sessionIDs.Clear(uint(idx))
	manager.count--
}
