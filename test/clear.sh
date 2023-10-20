#!/bin/bash
./build.sh || exit $?

echo "$0: $(pwd)"

TANK=tank0

rm -v hashdb
rm -v history

ln -sv /$TANK/nntp-history/hashdb . || exit 1
ln -sv /$TANK/nntp-history/history . || exit 1

# clears files
rm -v hashdb/history.dat.hash.*
rm -v history/history.dat

rm -v *.out
