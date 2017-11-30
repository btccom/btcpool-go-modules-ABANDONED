package main

import (
	"errors"
	"net"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"

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
	r0, _, e1 := syscall.Syscall(syscall.SYS_FCNTL, uintptr(fd), uintptr(cmd), uintptr(arg))
	val = int(r0)
	if e1 != 0 {
		err = e1
	}
	return
}

func setCloseOnExec(fd uintptr) (err error) {
	_, err = fcntl(int(fd), syscall.F_SETFD, syscall.FD_CLOEXEC)
	return
}

func setNoCloseOnExec(fd uintptr) (err error) {
	_, err = fcntl(int(fd), syscall.F_SETFD, ^syscall.FD_CLOEXEC)
	return
}

func execNewBin(binPath string, args []string) (err error) {
	realPath, err := filepath.Abs(binPath)
	if err != nil {
		realPath = binPath
	}

	argv := append([]string{binPath}, args...)
	envv := os.Environ()

	glog.Info("Exec: ", argv)

	return syscall.Exec(realPath, argv, envv)
}

func getConnFd(conn net.Conn) (fd uintptr, err error) {
	tc, ok := conn.(*net.TCPConn)
	if !ok {
		return 0, errors.New("getConnFd: conn is not a TCPConn")
	}

	fc, err := tc.File()
	if err != nil {
		return
	}

	fd = fc.Fd()
	return
}

func getListenerFd(listener net.Listener) (fd uintptr, err error) {
	tl, ok := listener.(*net.TCPListener)
	if !ok {
		return 0, errors.New("getListenerFd: listener is not a TCPListener")
	}

	fl, err := tl.File()
	if err != nil {
		return
	}

	fd = fl.Fd()
	return
}

func newConnFromFd(fd uintptr) (conn net.Conn, err error) {
	// 防止文件描述符泄露
	err = setCloseOnExec(fd)
	if err != nil {
		return
	}

	f := os.NewFile(fd, "tcp conn")
	conn, err = net.FileConn(f)
	return
}

func newListenerFromFd(fd uintptr) (listener net.Listener, err error) {
	// 防止文件描述符泄露
	err = setCloseOnExec(fd)
	if err != nil {
		return
	}

	f := os.NewFile(fd, "tcp listener")
	listener, err = net.FileListener(f)
	return
}

func signalUSR2Listener(callback func()) {
	c := make(chan os.Signal)
	signal.Notify(c, syscall.SIGUSR2)
	for {
		<-c
		callback()
	}
}
