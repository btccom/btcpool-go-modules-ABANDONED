package main

import (
	"flag"
	"net"

	"github.com/golang/glog"
)

func main() {
	var serverID uint8 = 1
	var listenAddr = "0.0.0.0:18080"

	flag.Set("alsologtostderr", "true")
	flag.Parse()

	glog.Info("Listen TCP ", listenAddr)
	ln, err := net.Listen("tcp", listenAddr)

	if err != nil {
		glog.Fatal("listen failed: ", err)
		return
	}

	StratumSessionGlobalInit(serverID)

	for {
		conn, err := ln.Accept()

		if err != nil {
			continue
		}

		session, err := NewStratumSession(conn)

		if err != nil {
			conn.Close()
			glog.Error("NewStratumSession failed: ", err)
		}

		go session.Run()
	}
}
