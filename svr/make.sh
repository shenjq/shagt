#!/bin/sh
version="v0.1"
flags="-X 'main.Version=$version' -X 'main.BuildTime=`date +"%Y-%m-%d %H:%m:%S"`' "
go build -ldflags "$flags" shagt_svr.go
mv shagt_svr ../bin
