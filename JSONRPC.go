package main

import (
	"encoding/json"
)

// JSONRPCData 是 Stratum 协议里用到的 JSON RPC 的数据结构
type JSONRPCData struct {
	ID     interface{}   `json:"id"`
	Method string        `json:"method"`
	Params []interface{} `json:"params"`
}

// NewJSONRPCData 解析 JSON RPC 字符串并创建 JSONRPCData 对象
func NewJSONRPCData(rpcJSON []byte) (JSONRPCData, error) {
	var rpcData JSONRPCData

	err := json.Unmarshal(rpcJSON, &rpcData)

	return rpcData, err
}

// AddParam 向 JSONRPCData 对象添加一个或多个参数
func (rpcData JSONRPCData) AddParam(param ...interface{}) {
	rpcData.Params = append(rpcData.Params, param...)
}

// SetParam 设置 JSONRPCData 对象的参数
// 传递给 SetParam 的参数列表将按顺序复制到 JSONRPCData.Params 中
func (rpcData JSONRPCData) SetParam(param ...interface{}) {
	rpcData.Params = param
}

// ToJSONBytes 将 JSONRPCData 对象转换为 JSON 字节序列
func (rpcData JSONRPCData) ToJSONBytes() ([]byte, error) {
	return json.Marshal(rpcData)
}
