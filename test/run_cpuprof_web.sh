#!/bin/bash
PATH="$PATH:/usr/local/go/bin"
export GOPATH=$(pwd)
export GO111MODULE=auto
export GOEXPERIMENT=arenas
#go test -cpuprofile=cpu.out nntp-history-test.go || exit $?
#go run -cpuprofile=cpu.out nntp-history-test.go || exit $?
#go tool pprof cpu.pprof.webgrab http://127.0.0.1:1234/debug/pprof/profile
go tool pprof -http=:8000 cpu.pprof.out