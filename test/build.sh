#!/bin/bash
echo "$0"
PATH="$PATH:/usr/local/go/bin"
#export GOPATH=$(pwd)
export GO111MODULE=auto
#export GOEXPERIMENT=arenas
go build nntp-history-test.go
RET=$?
test $RET -eq 0 && cp -v nntp-history-test test2/
echo $(date)
test $RET -gt 0 && echo "BUILD FAILED! RET=$RET" || echo "BUILD OK!"
rsync -va clear.sh nntp-history-test GOtest.sh root@spool1:/home/nntp-history/
#mkdir -p hashdb history
#rm -f hashdb/* history/*
exit $RET
