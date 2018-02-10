package main

import (
	"bytes"
	"encoding/binary"
	"encoding/hex"
	"net"
	"strconv"
)

// IP2Long IP转整数
// 来自 <https://www.socketloop.com/tutorials/golang-convert-ip-address-string-to-long-unsigned-32-bit-integer>
func IP2Long(ip string) uint32 {
	var long uint32
	binary.Read(bytes.NewBuffer(net.ParseIP(ip).To4()), binary.BigEndian, &long)
	return long
}

// Long2IP 整数转IP
// 来自 <https://www.socketloop.com/tutorials/golang-convert-ip-address-string-to-long-unsigned-32-bit-integer>
func Long2IP(ipLong uint32) string {
	ipInt := int64(ipLong)

	// need to do two bit shifting and “0xff” masking
	b0 := strconv.FormatInt((ipInt>>24)&0xff, 10)
	b1 := strconv.FormatInt((ipInt>>16)&0xff, 10)
	b2 := strconv.FormatInt((ipInt>>8)&0xff, 10)
	b3 := strconv.FormatInt((ipInt & 0xff), 10)
	return b0 + "." + b1 + "." + b2 + "." + b3
}

// Uint32ToHex unit32 转 hex
func Uint32ToHex(num uint32) string {
	bytesBuffer := bytes.NewBuffer([]byte{})
	binary.Write(bytesBuffer, binary.BigEndian, num)
	return hex.EncodeToString(bytesBuffer.Bytes())
}

// SubString 字符串截取
// <http://outofmemory.cn/code-snippet/1365/Go-language-jiequ-string-function>
func SubString(str string, start, length int) string {
	rs := []rune(str)
	rl := len(rs)
	end := 0

	if start < 0 {
		start = rl - 1 + start
	}
	end = start + length

	if start > end {
		start, end = end, start
	}

	if start < 0 {
		start = 0
	}
	if start > rl {
		start = rl
	}
	if end < 0 {
		end = 0
	}
	if end > rl {
		end = rl
	}

	return string(rs[start:end])
}
