package main

import (
	"errors"
	"strconv"
	"sync"

	"github.com/willf/bitset"
)

//////////////////////////////// SessionIDManager //////////////////////////////

// SessionIDManager 线程安全的会话ID管理器
type SessionIDManager struct {
	//
	//  SESSION ID: UINT32
	//
	//   xxxxxxxx     xxxxxxxx xxxxxxxx xxxxxxxx
	//  ----------    --------------------------
	//  server ID         session index id
	//   [1, 255]        range: [0, MaxValidSessionID]
	//
	serverID   uint8
	sessionIDs *bitset.BitSet

	count    uint32 // how many ids are used now
	allocIDx uint32
	lock     sync.Mutex

	indexBits uint8 // bits of session index id
	// SessionIDMask 会话ID掩码，用于分离serverID和sessionID
	// 也是sessionID部分可以达到的最大数值
	sessionIDMask uint32

	// MaxValidSessionID 最大的合法sessionID
	// should less than sessionIDMask
	maxValidSessionID uint32
}

// NewSessionIDManager 创建一个会话ID管理器实例
func NewSessionIDManager(serverID uint8, indexBits uint8) (manager *SessionIDManager, err error) {
	if indexBits > 24 {
		err = errors.New("indexBits should not > 24, but it = " + strconv.Itoa(int(indexBits)))
		return
	}
	if serverID == 0 {
		err = errors.New("serverID not set (serverID = 0)")
		return
	}

	manager = new(SessionIDManager)

	manager.sessionIDMask = (1 << indexBits) - 1
	manager.maxValidSessionID = manager.sessionIDMask - 1

	manager.serverID = serverID
	manager.sessionIDs = bitset.New(uint(manager.sessionIDMask))
	manager.count = 0
	manager.allocIDx = 0

	manager.sessionIDs.ClearAll()
	return
}

// isFull 判断会话ID是否已满（内部使用，不加锁）
func (manager *SessionIDManager) isFullWithoutLock() bool {
	return (manager.count >= manager.sessionIDMask)
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
		sessionID = manager.sessionIDMask
		err = ErrSessionIDFull
		return
	}

	// find an empty bit
	for manager.sessionIDs.Test(uint(manager.allocIDx)) {
		manager.allocIDx++
		if manager.allocIDx > manager.maxValidSessionID {
			manager.allocIDx = 0
		}
	}

	// set to true
	manager.sessionIDs.Set(uint(manager.allocIDx))
	manager.count++

	sessionID = (uint32(manager.serverID) << manager.indexBits) | manager.allocIDx
	err = nil
	return
}

// ResumeSessionID 恢复之前的会话ID
func (manager *SessionIDManager) ResumeSessionID(sessionID uint32) (err error) {
	defer manager.lock.Unlock()
	manager.lock.Lock()

	idx := sessionID & manager.sessionIDMask

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

	idx := sessionID & manager.sessionIDMask

	if !manager.sessionIDs.Test(uint(idx)) {
		// ID未分配，无需释放
		return
	}

	manager.sessionIDs.Clear(uint(idx))
	manager.count--
}
