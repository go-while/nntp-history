#!/bin/bash
PATH="$PATH:/usr/local/go/bin"
export GOPATH=$(pwd)
export GO111MODULE=auto
export GOEXPERIMENT=arenas
clear
go run -race nntp-history-test.go
