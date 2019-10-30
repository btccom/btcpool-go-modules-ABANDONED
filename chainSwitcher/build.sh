#!/bin/sh
cd $(dirname "$0")

go get -v github.com/segmentio/kafka-go
go get -v github.com/golang/snappy
go get -v github.com/go-sql-driver/mysql
go get -v github.com/golang/glog

go build -v
