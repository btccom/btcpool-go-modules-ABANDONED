package main

import (
	"encoding/hex"
	"errors"

	"./hash"

	"github.com/GeertJohan/go.httpjsonrpc"
	"github.com/golang/glog"
)

// AuxBlockInfo 合并挖矿的区块信息
type AuxBlockInfo struct {
	Hash          hash.Byte32
	ChainID       uint32
	Bits          string
	Target        hash.Byte32
	Height        uint32
	PrevBlockHash hash.Byte32
	CoinbaseValue uint32

	// RPCRawResult RPC返回的原始结果
	RPCRawResult map[string]interface{}
}

// RPCCall 调用RPC方法
func RPCCall(serverInfo CoinRPCInfo, methodInfo RPCMethodInfo) (result interface{}, err error) {
	client := httpjsonrpc.NewClient(serverInfo.RPCUrl, nil)
	client.SetBasicAuth(serverInfo.RPCUser, serverInfo.RPCPasswd)

	_, err = client.Call(methodInfo.Method, methodInfo.Params, &result)
	return
}

// RPCCallCreateAuxBlock 调用CreateAuxBlock方法
func RPCCallCreateAuxBlock(rpcInfo CoinRPCInfo) (auxBlockInfo AuxBlockInfo, err error) {
	result, err := RPCCall(rpcInfo, rpcInfo.CreateAuxBlock)
	if err != nil {
		return
	}

	var ok bool
	auxBlockInfo.RPCRawResult, ok = result.(map[string]interface{})
	if !ok {
		err = errors.New("RPC result is not a JSON object")
		return
	}

	// ------------ Hash ------------

	hashKey, ok := rpcInfo.CreateAuxBlock.ResponseKeys["Hash"]
	if !ok {
		err = errors.New("config: missing Chains[n].CreateAuxBlock.ResponseKeys.Hash")
		return
	}

	hash, ok := auxBlockInfo.RPCRawResult[hashKey]
	if !ok {
		err = errors.New("rpc result: missing " + hashKey)
		return
	}

	hashStr, ok := hash.(string)
	if !ok {
		err = errors.New("rpc result: " + hashKey + " is not a string")
		return
	}

	hashByte, err := hex.DecodeString(hashStr)
	if err != nil {
		err = errors.New("rpc result: " + hashKey + " decode failed: " + err.Error())
		return
	}

	if len(hashByte) != 32 {
		err = errors.New("rpc result: " + hashKey + " is not a hex of 32 bytes")
		return
	}

	auxBlockInfo.Hash.Assign(hashByte)
	auxBlockInfo.Hash.Reverse()

	// ------------ ChainID ------------
	chainIDKey, ok := rpcInfo.CreateAuxBlock.ResponseKeys["ChainID"]
	if !ok {
		err = errors.New("config: missing Chains[n].CreateAuxBlock.ResponseKeys.ChainID")
		return
	}

	chainID, ok := auxBlockInfo.RPCRawResult[chainIDKey]
	if !ok {
		err = errors.New("rpc result: missing " + chainIDKey)
		return
	}

	chainIDFloat, ok := chainID.(float64)
	if !ok {
		err = errors.New("rpc result: " + chainIDKey + " is not a number")
		return
	}

	auxBlockInfo.ChainID = uint32(chainIDFloat)

	// ------------ Bits ------------

	bitsKey, ok := rpcInfo.CreateAuxBlock.ResponseKeys["Bits"]
	if !ok {
		err = errors.New("config: missing Chains[n].CreateAuxBlock.ResponseKeys.Bits")
		return
	}

	bits, ok := auxBlockInfo.RPCRawResult[bitsKey]
	if !ok {
		err = errors.New("rpc result: missing " + bitsKey)
		return
	}

	auxBlockInfo.Bits, ok = bits.(string)
	if !ok {
		err = errors.New("rpc result: " + bitsKey + " is not a string")
		return
	}

	// ------------ Target ------------

	targetKey, ok := rpcInfo.CreateAuxBlock.ResponseKeys["Target"]
	if !ok {
		err = errors.New("config: missing Chains[n].CreateAuxBlock.ResponseKeys.Target")
		return
	}

	target, ok := auxBlockInfo.RPCRawResult[targetKey]
	if !ok {
		err = errors.New("rpc result: missing " + targetKey)
		return
	}

	targetStr, ok := target.(string)
	if !ok {
		err = errors.New("rpc result: " + targetKey + " is not a string")
		return
	}

	targetByte, err := hex.DecodeString(targetStr)
	if err != nil {
		err = errors.New("rpc result: " + targetKey + " decode failed: " + err.Error())
		return
	}

	if len(targetByte) != 32 {
		err = errors.New("rpc result: " + targetKey + " is not a hex of 32 bytes")
		return
	}

	auxBlockInfo.Target.Assign(targetByte)
	auxBlockInfo.Target.Reverse()

	// ------------ Height ------------

	heightKey, ok := rpcInfo.CreateAuxBlock.ResponseKeys["Height"]
	if !ok {
		glog.Info("config: missing (optional) Chains[n].CreateAuxBlock.ResponseKeys.Bits, skip")
	} else {
		height, ok := auxBlockInfo.RPCRawResult[heightKey]
		if !ok {
			err = errors.New("rpc result: missing " + heightKey)
			return
		}

		heightFloat, ok := height.(float64)
		if !ok {
			err = errors.New("rpc result: " + heightKey + " is not a number")
			return
		}
		auxBlockInfo.Height = uint32(heightFloat)
	}

	// ------------ PrevBlockHash ------------

	prevBlockHashKey, ok := rpcInfo.CreateAuxBlock.ResponseKeys["PrevBlockHash"]
	if !ok {
		glog.Info("config: missing Chains[n].CreateAuxBlock.ResponseKeys.PrevBlockHash, skip")
	} else {
		prevBlockHash, ok := auxBlockInfo.RPCRawResult[prevBlockHashKey]
		if !ok {
			err = errors.New("rpc result: missing " + prevBlockHashKey)
			return
		}

		prevBlockHashStr, ok := prevBlockHash.(string)
		if !ok {
			err = errors.New("rpc result: " + prevBlockHashKey + " is not a string")
			return
		}

		var prevBlockHashByte []byte
		prevBlockHashByte, err = hex.DecodeString(prevBlockHashStr)
		if err != nil {
			err = errors.New("rpc result: " + prevBlockHashKey + " decode failed: " + err.Error())
			return
		}

		if len(prevBlockHashByte) != 32 {
			err = errors.New("rpc result: " + prevBlockHashKey + " is not a hex of 32 bytes")
			return
		}

		auxBlockInfo.PrevBlockHash.Assign(prevBlockHashByte)
	}

	// ------------ CoinbaseValue ------------
	coinbaseValueKey, ok := rpcInfo.CreateAuxBlock.ResponseKeys["CoinbaseValue"]
	if !ok {
		err = errors.New("config: missing Chains[n].CreateAuxBlock.ResponseKeys.CoinbaseValue")
		return
	}

	coinbaseValue, ok := auxBlockInfo.RPCRawResult[coinbaseValueKey]
	if !ok {
		err = errors.New("rpc result: missing " + coinbaseValueKey)
		return
	}

	coinbaseValueFloat, ok := coinbaseValue.(float64)
	if !ok {
		err = errors.New("rpc result: " + coinbaseValueKey + " is not a number")
		return
	}

	auxBlockInfo.CoinbaseValue = uint32(coinbaseValueFloat)

	// ------------ Finished ------------
	return
}
