package main

import (
	"encoding/binary"
	"io"
)

// Share 通用的Share接口
type Share interface {
	Load(r io.Reader) error
	Save(w io.Writer) error
	GetUserID() int32
	GetWorkerID() int64
	GetIPv4() uint32
	GetTime() uint64
	GetShareDiff() uint64
}

// ShareBitcoinV1 第一版比特币Share结构
type ShareBitcoinV1 struct {
	JobID     uint64
	WorkerID  int64
	IPv4      uint32
	UserID    int32
	ShareDiff uint64
	TimeStamp uint32
	BlockBits uint32
	Result    int32
	Padding   int32
}

// Load 从文件读入Share
func (share *ShareBitcoinV1) Load(r io.Reader) error {
	return binary.Read(r, binary.LittleEndian, share)
}

// Save 写入Share到文件
func (share *ShareBitcoinV1) Save(w io.Writer) error {
	return binary.Write(w, binary.LittleEndian, share)
}

// GetUserID 获得用户id
func (share *ShareBitcoinV1) GetUserID() int32 {
	return share.UserID
}

// GetWorkerID 获取矿工ID
func (share *ShareBitcoinV1) GetWorkerID() int64 {
	return share.WorkerID
}

// GetIPv4 获取矿工的IPv4地址
func (share *ShareBitcoinV1) GetIPv4() uint32 {
	return share.IPv4
}

// GetTime 获取Share的提交时间
func (share *ShareBitcoinV1) GetTime() uint64 {
	return uint64(share.TimeStamp)
}

// GetTime 获取Share的难度
func (share *ShareBitcoinV1) GetShareDiff() uint64 {
	return share.ShareDiff
}
