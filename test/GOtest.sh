# testing bbolt batch debug
./nntp-history-test -numcpu=4 -BoltDB_PageSize=4 -BoltDB_MaxBatchSize=3 -BoltDB_MaxBatchDelay=100 -BatchFlushEvery=100 -RootBUCKETSperDB=256 -todo 100000000 -p 4 -keyindex=2 -pprofcpu=true -pprofmem=true -pprof localhost:1234 -NoReplayHisDat=false # -DBG_ABS1=true -DBG_ABS2=true

# 100k+ tx/sec test
# ./nntp-history-test -numcpu=4 -BoltDB_MaxBatchSize=1024 -BoltDB_MaxBatchDelay=100 -BatchFlushEvery=5000 -RootBUCKETSperDB=256 -todo 100000000 -p 4 -keyindex=2 -pprofcpu=true -pprofmem=true -pprof localhost:1234 -NoReplayHisDat=false # -DBG_ABS1=true -DBG_ABS2=true


# other tests
# ./nntp-history-test -todo 100000000 -p 4 -pprofcpu=true -numcpu=4 #-DBG_ABS1=true
# ./nntp-history-test -todo 50000000 -keyalgo=11 -keylen=8 -p 4 # -pprofcpu=true -pprofmem=true
