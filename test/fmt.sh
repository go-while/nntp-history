#!/bin/bash
PATH="$PATH:/usr/local/go/bin"
echo "FMT"
go fmt *.go
cd ../ || exit 1
go fmt *.go
RET=$?
test $RET -gt 0 && echo "go fmt failed!" && exit $RET
exit $RET
