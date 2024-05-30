#!/bin/bash -x
./build.sh || exit $?
DIR="50M"
TANK=tank0
test ! -z "$1" && DIR="$1"
test -e "$DIR" && echo "dir not found $DIR" && exit 1
rm -v hashdb history
ln -sv /$TANK/nntp-history/hashdb/100M/hashdb .
ln -sv /$TANK/nntp-history/history/100M/history .
rm -v *.out
