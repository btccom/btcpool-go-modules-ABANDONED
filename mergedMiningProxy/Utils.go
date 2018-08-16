package main

import (
	"encoding/hex"
	"fmt"
	"math/big"
	"strconv"
)

// CompactToBig 将target的压缩形式（bits）转换为大整数
// 拷贝自 <https://github.com/elastos/Elastos.ELA/blob/28be25a43f3befaa8bc7d7b77f8dfa8b48d43970/blockchain/blockchain.go#L380>
func CompactToBig(compact uint32) *big.Int {
	// Extract the mantissa, sign bit, and exponent.
	mantissa := compact & 0x007fffff
	isNegative := compact&0x00800000 != 0
	exponent := uint(compact >> 24)

	// Since the base for the exponent is 256, the exponent can be treated
	// as the number of bytes to represent the full 256-bit number.  So,
	// treat the exponent as the number of bytes and shift the mantissa
	// right or left accordingly.  This is equivalent to:
	// N = mantissa * 256^(exponent-3)
	var bn *big.Int
	if exponent <= 3 {
		mantissa >>= 8 * (3 - exponent)
		bn = big.NewInt(int64(mantissa))
	} else {
		bn = big.NewInt(int64(mantissa))
		bn.Lsh(bn, 8*(exponent-3))
	}

	// Make it negative if the sign bit is set.
	if isNegative {
		bn = bn.Neg(bn)
	}

	return bn
}

// BigToCompact 将大整数转换为target的压缩形式（bits）
// 拷贝自 <https://github.com/elastos/Elastos.ELA/blob/28be25a43f3befaa8bc7d7b77f8dfa8b48d43970/blockchain/difficulty.go#L83>
func BigToCompact(n *big.Int) uint32 {
	// No need to do any work if it's zero.
	if n.Sign() == 0 {
		return 0
	}

	// Since the base for the exponent is 256, the exponent can be treated
	// as the number of bytes.  So, shift the number right or left
	// accordingly.  This is equivalent to:
	// mantissa = mantissa / 256^(exponent-3)
	var mantissa uint32
	exponent := uint(len(n.Bytes()))
	if exponent <= 3 {
		mantissa = uint32(n.Bits()[0])
		mantissa <<= 8 * (3 - exponent)
	} else {
		// Use a copy to avoid modifying the caller's original number.
		tn := new(big.Int).Set(n)
		mantissa = uint32(tn.Rsh(tn, 8*(exponent-3)).Bits()[0])
	}

	// When the mantissa already has the sign bit set, the number is too
	// large to fit into the available 23-bits, so divide the number by 256
	// and increment the exponent accordingly.
	if mantissa&0x00800000 != 0 {
		mantissa >>= 8
		exponent++
	}

	// Pack the exponent, sign bit, and mantissa into an unsigned 32-bit
	// int and return it.
	compact := uint32(exponent<<24) | mantissa
	if n.Sign() < 0 {
		compact |= 0x00800000
	}
	return compact
}

// BitsToTarget 将target的压缩形式（bits）转换为完整形式
func BitsToTarget(bits string) (target string, err error) {
	var bitsNum uint64
	bitsNum, err = strconv.ParseUint(bits, 16, 32)
	if err != nil {
		return
	}

	targetBig := CompactToBig(uint32(bitsNum))
	target = fmt.Sprintf("%064s", targetBig.Text(16))
	return
}

// TargetToBits 将target的完整形式转换为压缩形式（bits）
func TargetToBits(target string) (bits string, err error) {
	targetBytes, err := hex.DecodeString(target)
	if err != nil {
		return
	}
	targetNum := new(big.Int).SetBytes(targetBytes[:])
	bitsNum := BigToCompact(targetNum)
	bits = fmt.Sprintf("%08x", bitsNum)
	return
}

// DeepCopy 深度拷贝map和切片
// 来自：https://studygolang.com/articles/8036
func DeepCopy(value interface{}) interface{} {
	if valueMap, ok := value.(map[string]interface{}); ok {
		newMap := make(map[string]interface{})
		for k, v := range valueMap {
			newMap[k] = DeepCopy(v)
		}
		return newMap

	} else if valueSlice, ok := value.([]interface{}); ok {
		newSlice := make([]interface{}, len(valueSlice))
		for k, v := range valueSlice {
			newSlice[k] = DeepCopy(v)
		}
		return newSlice

	}

	return value
}
