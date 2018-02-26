package main

import (
	"fmt"
	"math/big"
	"strconv"
)

// CompactToBig 将target的压缩形式（bits）转换为大整数
// 拷贝自 <https://github.com/elastos/Elastos.ELA/blob/master/core/ledger/difficulty.go#L135>
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
