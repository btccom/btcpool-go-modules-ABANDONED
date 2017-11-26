package main

import (
	"net"

	"github.com/golang/glog"
)

// Zero Downtime upgrades of TCP servers in Go
// <http://blog.nella.org/?p=879>

// These are here because there is no API in syscall for turning OFF
// close-on-exec (yet).

// from syscall/zsyscall_linux_386.go, but it seems like it might work
// for other platforms too.
// copied from <https://github.com/jeffallen/jra-go/blob/master/debugreader/debugreader.go>.
func fcntl(fd int, cmd int, arg int) (val int, err error) {
	glog.Fatal("Function fcntl has not implement in Windows.")
	return
}

func setCloseOnExec(fd uintptr) {
	glog.Fatal("Function setCloseOnExec has not implement in Windows.")
	return
}

func setNoCloseOnExec(fd uintptr) {
	glog.Fatal("Function setNoCloseOnExec has not implement in Windows.")
	return
}

func execNewBin(binPath string, args []string) (err error) {
	glog.Fatal("Function execNewBin has not implement in Windows.")
	return
}

func getConnFd(conn net.Conn) (fd uintptr, err error) {
	glog.Fatal("Function getConnFd has not implement in Windows.")
	return
}

func getListenerFd(listener net.Listener) (fd uintptr, err error) {
	glog.Fatal("Function getListenerFd has not implement in Windows.")
	return
}

func newConnFromFd(fd uintptr) (conn net.Conn, err error) {
	glog.Fatal("Function newConnFromFd has not implement in Windows.")
	return
}

func newListenerFromFd(fd uintptr) (listener net.Listener, err error) {
	glog.Fatal("Function newListenerFromFd has not implement in Windows.")
	return
}

func signalUSR2Listener(callback func()) {
	glog.Info("Function signalUSR2Listener has not implement in Windows.")
	return
}
