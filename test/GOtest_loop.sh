#!/bin/bash
step=1000000 # 1M
ends=100000000 # 100M
if [ ! -z "$2" ]; then
 step="$1"
 ends="$2"
else
 echo "usage: $0 step=$step ends=$ends"
fi

todo=$step
while [ $todo -le $ends ]; do
 ./nntp-history-test -todo $todo -p 4 -keyindex=2 -pprofmem=true -numcpu=4 -DBG_ABS1=true -RootBUCKETSperDB=256 -pprof localhost:1234 -NoReplayHisDat=false
 let todo="todo + $step"
done
