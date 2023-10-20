#!/bin/bash
PATH="$PATH:/usr/local/go/bin"
export GOPATH=$(pwd)
export GO111MODULE=auto
export GOEXPERIMENT=arenas
export BOLTDB_DEBUG=1 
go run nntp-history-test.go
