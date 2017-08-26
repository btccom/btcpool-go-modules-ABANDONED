package main

import (
	"flag"
	"net"

	"github.com/golang/glog"
)

func main() {
	var serverID uint8 = 1
	var listenAddr = "0.0.0.0:18080"
	var zkBroker = []string{"127.0.0.1:2181"}
	var zkSwitcherWatchDir = "/stratumSwitcher/btcbcc/"
	var serverMap = StratumServerInfoMap{"btc": StratumServerInfo{"cn.ss.btc.com:1800"}, "bcc": StratumServerInfo{"cn.ss.btc.com:443"}}

	flag.Set("alsologtostderr", "true")
	flag.Parse()

	glog.Info("Listen TCP ", listenAddr)
	ln, err := net.Listen("tcp", listenAddr)

	if err != nil {
		glog.Fatal("listen failed: ", err)
		return
	}

	err = StratumSessionGlobalInit(serverID, serverMap, zkBroker, zkSwitcherWatchDir)

	if err != nil {
		glog.Fatal("init failed: ", err)
		return
	}

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
