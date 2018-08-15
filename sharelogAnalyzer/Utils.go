package main

import "time"

// GetCurrentShareLogFile 获取当前的ShareLog文件名
func GetCurrentShareLogFile(dir string, pattern string) string {
	return dir + "/" + time.Now().UTC().Format(pattern)
}
