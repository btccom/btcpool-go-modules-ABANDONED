package main

import (
	"flag"
	"net"

	"github.com/golang/glog"
)

func main() {
	flag.Set("alsologtostderr", "true")
	flag.Parse()

	glog.Info("Listen TCP 0.0.0.0:18080")
	ln, err := net.Listen("tcp", "0.0.0.0:18080")

	if err != nil {
		glog.Fatal("listen failed: ", err)
		return
	}

	for {
		conn, err := ln.Accept()

		if err != nil {
			continue
		}

		session := NewStratumSession(conn)
		go session.Run()
	}
}
