package main

import (
	"fmt"
	"regexp"
	"strconv"
	"time"
)

// GetCurrentShareLogFile 获取当前的ShareLog文件名
func GetCurrentShareLogFile(dir string, pattern string) string {
	return dir + "/" + time.Now().UTC().Format(pattern)
}

// IP2long IPv4点分十进制转整数
// 拷贝自 <http://outofmemory.cn/code-snippet/599/Go-yuyanban-ip2long-long2ip>
// 修改为小端字节序
func IP2long(ipstr string) (ip uint32) {
	r := `^(\d{1,3})\.(\d{1,3})\.(\d{1,3})\.(\d{1,3})`
	reg, err := regexp.Compile(r)
	if err != nil {
		return
	}
	ips := reg.FindStringSubmatch(ipstr)
	if ips == nil {
		return
	}

	ip1, _ := strconv.Atoi(ips[1])
	ip2, _ := strconv.Atoi(ips[2])
	ip3, _ := strconv.Atoi(ips[3])
	ip4, _ := strconv.Atoi(ips[4])

	if ip1 > 255 || ip2 > 255 || ip3 > 255 || ip4 > 255 {
		return
	}

	ip += uint32(ip1)
	ip += uint32(ip2 * 0x100)
	ip += uint32(ip3 * 0x10000)
	ip += uint32(ip4 * 0x1000000)

	return
}

// Long2IP 整数转IPv4点分十进制
// 拷贝自 <http://outofmemory.cn/code-snippet/599/Go-yuyanban-ip2long-long2ip>
// 修改为小端字节序
func Long2IP(ip uint32) string {
	return fmt.Sprintf("%d.%d.%d.%d", ip<<24>>24, ip<<16>>24, ip<<8>>24, ip>>24)
}
