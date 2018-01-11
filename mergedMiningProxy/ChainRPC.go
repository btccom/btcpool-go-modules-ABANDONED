package main

import (
	"encoding/hex"
	"encoding/json"
	"errors"

	"merkle-tree-and-bitcoin/hash"
)

// AuxBlockInfo 合并挖矿的区块信息
type AuxBlockInfo struct {
	Hash          hash.Byte32
	ChainID       uint32
	Bits          string
	Target        hash.Byte32
	Height        uint32
	PrevBlockHash hash.Byte32
	CoinbaseValue uint64
}

// RPCCallCreateAuxBlock 调用CreateAuxBlock方法
func RPCCallCreateAuxBlock(rpcInfo ChainRPCInfo) (auxBlockInfo AuxBlockInfo, err error) {
	responseJSON, err := RPCCall(rpcInfo.RPCServer, rpcInfo.CreateAuxBlock.Method, rpcInfo.CreateAuxBlock.Params)
	if err != nil {
		return
	}

	response, err := ParseRPCResponse(responseJSON)
	if response.Error != nil {
		errBytes, _ := json.Marshal(response.Error)
		err = errors.New(string(errBytes))
	}

	rpcRawResult, ok := response.Result.(map[string]interface{})
	if !ok {
		err = errors.New("RPC result is not a JSON object")
		return
	}

	// ------------ Hash ------------

	hashKey := rpcInfo.CreateAuxBlock.ResponseKeys.Hash
	hash, ok := rpcRawResult[hashKey]
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
	chainIDKey := rpcInfo.CreateAuxBlock.ResponseKeys.ChainID
	if len(chainIDKey) < 1 {
		auxBlockInfo.ChainID = rpcInfo.ChainID
	} else {
		chainID, ok := rpcRawResult[chainIDKey]
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
	}

	// ------------ Bits ------------

	bitsKey := rpcInfo.CreateAuxBlock.ResponseKeys.Bits
	bits, ok := rpcRawResult[bitsKey]
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

	targetKey := rpcInfo.CreateAuxBlock.ResponseKeys.Target
	target, ok := rpcRawResult[targetKey]
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

	heightKey := rpcInfo.CreateAuxBlock.ResponseKeys.Height
	if len(heightKey) >= 1 {
		height, ok := rpcRawResult[heightKey]
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

	prevBlockHashKey := rpcInfo.CreateAuxBlock.ResponseKeys.PrevBlockHash
	if len(prevBlockHashKey) >= 1 {
		prevBlockHash, ok := rpcRawResult[prevBlockHashKey]
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
	coinbaseValueKey := rpcInfo.CreateAuxBlock.ResponseKeys.CoinbaseValue
	if len(coinbaseValueKey) >= 1 {

		coinbaseValue, ok := rpcRawResult[coinbaseValueKey]
		if !ok {
			err = errors.New("rpc result: missing " + coinbaseValueKey)
			return
		}

		coinbaseValueFloat, ok := coinbaseValue.(float64)
		if !ok {
			err = errors.New("rpc result: " + coinbaseValueKey + " is not a number")
			return
		}

		auxBlockInfo.CoinbaseValue = uint64(coinbaseValueFloat)
	}

	// ------------ Finished ------------
	return
}
