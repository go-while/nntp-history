# testing bbolt batch debug: test1
./nntp-history-test -numcpu=4 -BoltDB_PageSize=16 -BoltDB_MaxBatchSize=3 -BoltDB_MaxBatchDelay=100 -BatchFlushEvery=100 -RootBUCKETSperDB=256 -todo 100000000 -p 4 -keyindex=2 -pprofcpu=true -pprofmem=true -pprof localhost:1234 -NoReplayHisDat=false # -DBG_ABS1=true -DBG_ABS2=true

# testing bbolt batch debug: test2
#./nntp-history-test -numcpu=4 -BoltDB_PageSize=16 -BoltDB_MaxBatchSize=3 -BoltDB_MaxBatchDelay=1000 -BatchFlushEvery=1000 -RootBUCKETSperDB=256 -todo 100000000 -p 4 -keyindex=2 -pprofcpu=true -pprofmem=true -pprof localhost:1234 -NoReplayHisDat=false # -DBG_ABS1=true -DBG_ABS2=true

# testing bbolt batch debug: test3
#./nntp-history-test -numcpu=4 -BoltDB_PageSize=16 -BoltDB_MaxBatchSize=3 -BoltDB_MaxBatchDelay=10 -BatchFlushEvery=2500 -RootBUCKETSperDB=256 -todo 100000000 -p 4 -keyindex=2 -pprofcpu=true -pprofmem=true -pprof localhost:1234 -NoReplayHisDat=false # -DBG_ABS1=true -DBG_ABS2=true



# 100k+ tx/sec test
# ./nntp-history-test -numcpu=4 -BoltDB_MaxBatchSize=1024 -BoltDB_MaxBatchDelay=100 -BatchFlushEvery=5000 -RootBUCKETSperDB=256 -todo 100000000 -p 4 -keyindex=2 -pprofcpu=true -pprofmem=true -pprof localhost:1234 -NoReplayHisDat=false # -DBG_ABS1=true -DBG_ABS2=true

# testing 4K root buckets * 16 dbs = 65k go routines as batchQueues
#./nntp-history-test -numcpu=4 -BoltDB_PageSize=64 -BoltDB_MaxBatchSize=10 -BoltDB_MaxBatchDelay=100 -BatchFlushEvery=15000 -RootBUCKETSperDB=4096 -todo 100000000 -p 4 -keyindex=2 -pprofcpu=true -pprofmem=true -pprof localhost:1234 -NoReplayHisDat=false # -DBG_ABS1=true -DBG_ABS2=true

# testing 4k root buckets + 60s batchflushevery (better have 16g+ of ram)
#./nntp-history-test -numcpu=4 -BoltDB_PageSize=16 -BoltDB_MaxBatchSize=1000 -BoltDB_MaxBatchDelay=1000 -BatchFlushEvery=60000 -RootBUCKETSperDB=4096 -todo 100000000 -p 4 -keyindex=2 -pprofcpu=true -pprofmem=true -pprof localhost:1234 -NoReplayHisDat=false # -DBG_ABS1=true -DBG_ABS2=true


# other tests
# ./nntp-history-test -todo 100000000 -p 4 -pprofcpu=true -numcpu=4 #-DBG_ABS1=true
# ./nntp-history-test -todo 50000000 -keyalgo=11 -keylen=8 -p 4 # -pprofcpu=true -pprofmem=true
