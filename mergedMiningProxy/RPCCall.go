package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
)

// RPCRequest JSON-RPC 1.0 请求（数组参数）
type RPCRequest struct {
	ID     interface{}   `json:"id"`
	Method string        `json:"method"`
	Params []interface{} `json:"params"`
}

// RPC2Request JSON-RPC 2.0 请求（数组或对象参数）
type RPC2Request struct {
	ID     interface{} `json:"id"`
	Method string      `json:"method"`
	Params interface{} `json:"params"`
}

// RPCResponse RPC响应
type RPCResponse struct {
	ID     interface{} `json:"id"`
	Result interface{} `json:"result"`
	Error  interface{} `json:"error"`
}

// RPCError RPC错误
type RPCError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

// RPCCall 调用RPC方法
func RPCCall(server ChainRPCServer, method string, params interface{}) (response []byte, err error) {
	rpcRequest := RPC2Request{nil, method, params}

	// encode request to buffer
	bufSend := &bytes.Buffer{}
	enc := json.NewEncoder(bufSend)
	err = enc.Encode(rpcRequest)
	if err != nil {
		err = fmt.Errorf("Error when encoding json: %s", err)
		return
	}

	// create http request
	req, err := http.NewRequest("POST", server.URL, bufSend)
	if err != nil {
		err = fmt.Errorf("Error when creating new http request: %s", err)
		return
	}
	req.Header.Set("Content-Type", "application/json")
	if len(server.User) > 0 {
		req.SetBasicAuth(server.User, server.Passwd)
	}

	// do request
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		err = fmt.Errorf("Error when performing http request: %s", err)
		return
	}
	defer resp.Body.Close()

	// get response
	response, err = ioutil.ReadAll(resp.Body)
	if err != nil {
		err = fmt.Errorf("Error when reading http response: %s", err)
		return
	}

	// no error, finished
	return
}

// ParseRPCResponse 将RPC响应的JSON字节数组转换为RPCResponse结构体
func ParseRPCResponse(respJSON []byte) (response RPCResponse, err error) {
	err = json.Unmarshal(respJSON, &response)
	if err != nil {
		err = fmt.Errorf("Parse json %s failed: %s", string(respJSON), err)

	}
	return
}
