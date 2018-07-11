package main

import (
	"encoding/json"
)

// JSONRPCRequest JSON RPC 请求的数据结构
type JSONRPCRequest struct {
	ID     interface{}   `json:"id"`
	Method string        `json:"method"`
	Params []interface{} `json:"params"`

	// Worker: ETHProxy from ethminer may contains this field
	Worker string `json:"worker,omitempty"`
}

// JSONRPCResponse JSON RPC 响应的数据结构
type JSONRPCResponse struct {
	ID     interface{} `json:"id"`
	Result interface{} `json:"result"`
	Error  interface{} `json:"error"`
}

// JSONRPC2Response response message of json-rpc 2.0
type JSONRPC2Response struct {
	JSONRPC string `json:"jsonrpc,omitempty"`
	JSONRPCResponse
}

// JSONRPCArray JSON RPC 数组
type JSONRPCArray []interface{}

// jsonRPCVersion version of JSON-RPC response
var jsonRPCVersion uint8 = 1

// SetJSONRPCVersion set the value of "jsonrpc" field in response.
// Pass empty string if want to omit it.
func SetJSONRPCVersion(version uint8) {
	jsonRPCVersion = version
}

// NewJSONRPCRequest 解析 JSON RPC 请求字符串并创建 JSONRPCRequest 对象
func NewJSONRPCRequest(rpcJSON []byte) (*JSONRPCRequest, error) {
	rpcData := new(JSONRPCRequest)

	err := json.Unmarshal(rpcJSON, &rpcData)

	return rpcData, err
}

// AddParam 向 JSONRPCRequest 对象添加一个或多个参数
func (rpcData *JSONRPCRequest) AddParam(param ...interface{}) {
	rpcData.Params = append(rpcData.Params, param...)
}

// SetParam 设置 JSONRPCRequest 对象的参数
// 传递给 SetParam 的参数列表将按顺序复制到 JSONRPCRequest.Params 中
func (rpcData *JSONRPCRequest) SetParam(param ...interface{}) {
	rpcData.Params = param
}

// ToJSONBytes 将 JSONRPCRequest 对象转换为 JSON 字节序列
func (rpcData *JSONRPCRequest) ToJSONBytes() ([]byte, error) {
	return json.Marshal(rpcData)
}

// NewJSONRPCResponse 解析 JSON RPC 响应字符串并创建 JSONRPCResponse 对象
func NewJSONRPCResponse(rpcJSON []byte) (*JSONRPCResponse, error) {
	rpcData := new(JSONRPCResponse)

	err := json.Unmarshal(rpcJSON, &rpcData)

	return rpcData, err
}

// SetResult 设置 JSONRPCResponse 对象的返回结果
func (rpcData *JSONRPCResponse) SetResult(result interface{}) {
	rpcData.Result = result
}

// ToJSONBytes 将 JSONRPCResponse 对象转换为 JSON 字节序列
func (rpcData *JSONRPCResponse) ToJSONBytes() ([]byte, error) {
	if jsonRPCVersion == 1 {
		return json.Marshal(rpcData)
	}

	rpc2Data := JSONRPC2Response{"2.0", *rpcData}
	return json.Marshal(rpc2Data)
}
