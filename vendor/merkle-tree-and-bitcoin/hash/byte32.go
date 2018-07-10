package hash

import (
	"encoding/hex"
	"fmt"
)

// Byte32 is a type that provides a cute way of expressing this trivial
// fixed size array type, and is useful because the type is often used in
// slices, and it prevents a forest of square brackets when that is done.
type Byte32 [32]byte

// MakeByte32FromHex 从hex字符串初始化
func MakeByte32FromHex(hexStr string) (data Byte32, err error) {
	bytes, err := hex.DecodeString(hexStr)
	data.Assign(bytes)
	return
}

// Assign 赋值
func (data *Byte32) Assign(value []byte) {
	copy(data[:], value)
}

// Reverse 反转，返回前后顺序颠倒的字节
func (data *Byte32) Reverse() Byte32 {
	var reversed Byte32

	length := len(data)
	x := 0
	y := length - 1

	for x < length {
		reversed[x] = data[y]
		x++
		y--
	}

	return reversed
}

// Hex is just syntax sugar to avoid having to write things like
// fmt.Sprintf("0x", ... all over the place.
func (data *Byte32) Hex() string {
	return fmt.Sprintf("%0x", *data)
}

// HexReverse is just likes Hex() but with a reverse byte order.
func (data *Byte32) HexReverse() string {
	return fmt.Sprintf("%0x", data.Reverse())
}
