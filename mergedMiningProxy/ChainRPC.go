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
		err = errors.New("RPC result is not a JSON object: " + string(responseJSON))
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
	auxBlockInfo.Hash = auxBlockInfo.Hash.Reverse()

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

	// ------------ Bits or Target ------------
	bitsKey := rpcInfo.CreateAuxBlock.ResponseKeys.Bits
	targetKey := rpcInfo.CreateAuxBlock.ResponseKeys.Target
	if len(bitsKey) >= 1 {
		// Use bits first if exists
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

		// Convert bits to target
		targetStr, errmsg := BitsToTarget(auxBlockInfo.Bits)
		if errmsg != nil {
			err = errors.New("rpc result: cannot convert bits (" + auxBlockInfo.Bits + ") to target: " + errmsg.Error())
		}
		targetByte, errmsg := hex.DecodeString(targetStr)
		if err != nil {
			err = errors.New("rpc result: targetStr (" + targetStr + ") decode failed: " + errmsg.Error())
			return
		}
		if len(targetByte) != 32 {
			err = errors.New("rpc result: targetStr (" + targetStr + ") is not a hex of 32 bytes")
			return
		}
		auxBlockInfo.Target.Assign(targetByte)

	} else if len(targetKey) >= 1 {
		// Bits not exist, use target
		target, ok := rpcRawResult[targetKey]
		if !ok {
			err = errors.New("rpc result: missing " + targetKey)
			return
		}
		targetStr, ok := target.(string)
		if !ok {
			err = errors.New("rpc result: target is not a string")
			return
		}
		targetByte, errmsg := hex.DecodeString(targetStr)
		if err != nil {
			err = errors.New("rpc result: targetStr (" + targetStr + ") decode failed: " + errmsg.Error())
			return
		}
		if len(targetByte) != 32 {
			err = errors.New("rpc result: targetStr (" + targetStr + ") is not a hex of 32 bytes")
			return
		}
		auxBlockInfo.Target.Assign(targetByte)
		// The target string in getauxblock is little endian.
		auxBlockInfo.Target = auxBlockInfo.Target.Reverse()
		targetStr = auxBlockInfo.Target.Hex()

		// Convert target to bits
		auxBlockInfo.Bits, err = TargetToBits(targetStr)
		if err != nil {
			err = errors.New("rpc result: cannot convert Target ( " + targetStr + ") to bits: " + err.Error())
			return
		}

	} else {
		err = errors.New("wrong configure: the Bits and Target fields cannot be omitted at the same time")
		return
	}

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
