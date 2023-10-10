# nntp-history

nntp-history is a Go module designed for managing and storing NNTP (Network News Transfer Protocol) history records efficiently.

It provides a way to store and retrieve historical message records in a simple and scalable manner.

This module is suitable for building applications related to Usenet news servers or any system that requires managing message history efficiently.

```sh
go get github.com/go-while/nntp-history
   # bbolt code and test with latest commits from github
   # git glone https://github.com/etcd-io/bbolt in 'src/go.etcd.io/'
```

- Test application: [test/nntp-history-test.go](https://github.com/go-while/nntp-history/blob/main/test/nntp-history-test.go)

## Code Hints

## History_Boot Function

The `History_Boot` function in this Go code is responsible for initializing and booting a history management system.

It provides essential configuration options and prepares the system for historical data storage and retrieval.

## Usage

To use the `History_Boot` function, follow these steps:

1. Call the `History_Boot` function with the desired configuration options.

2. The history management system will be initialized and ready for use.

## history.History.WriterChan

- `history.History.WriterChan` is a Go channel used for sending and processing historical data entries.

- It is primarily responsible for writing data to a historical data storage system, using a HashDB (BoltDB) to avoid duplicate entries.

- To send data for writing, you create a `HistoryObject` and send it through the channel.

- If the `ResponseChan` channel is provided, it receives one of the following (int) values:
```go
  /*
  0: Indicates "not a duplicate."
  1: Indicates "duplicate."
  2: Indicates "retry later."
  */
```

## history.History.IndexChan

- The `history.History.IndexChan` is a Go channel with a dual purpose.

- Its primary function is to facilitate the detection of duplicate message-ID hashes within the history file.

- When the offset is set to -1, the channel performs a check for duplicate hashes but does not add the hash to the BoltDB database.

- When the offset is set to a value greater than zero, the channel functions as a mechanism for adding these message-ID hashes to the BoltDB database.

- Beware: Adding message-ID hashes is then normally done via `history.History.WriterChan` if you want to write the history file too!

- If desired, one could only use the `IndexChan` and avoid writing the history file. Use the full `KeyLen` of hash and provide a uniq up-counter for their Offsets.

- If the `IndexRetChan` channel is provided, it receives one of the following (int) values:
```go
  /*
  0: Indicates "not a duplicate."
  1: Indicates "duplicate."
  2: Indicates "retry later."
  */
```

# Message-ID Hash Splitting with BoltDB

## KeyAlgo (`ShortHash`)

- The standard key algorithm used is `ShortHash`.

## Database Organization

To improve processing speed and optimize data storage, we organize data into multiple BoltDB databases based on the first character of the hash:

- We create 16 separate BoltDB databases, each corresponding to one of the 16 possible hexadecimal characters (0-9 and a-f).

Each of these 16 databases is further divided into buckets using the second character of the hash:

- For each database, we create buckets corresponding to the possible values of the second character (0-9 and a-f).

## Key Structure

The remaining portion of the hash (after the first two characters) is used as the key to store offsets in the database:

- The length of this remaining hash used as a key can be customized based on the `KeyLen` setting.
- A lower `KeyLen` results in more "multioffsets" stored per key, which can be beneficial for reducing storage space.
- However, a lower `KeyLen` also result in more frequent file seeks when accessing data.
- The default `KeyLen` is 6. The minimum recommended `KeyLen` is 4.
- Reasonable values for `KeyLen` typically range from 5 to 8.

## FNV KeyAlgos

- The other available `FNV` KeyAlgos have pre-defined `KeyLen` values and use the full hash to generate keys.

## Example

Let's illustrate this approach with an example:

Suppose you have a Message-ID hash of "1a2b3c4d5e6f0...":

- The first character "1" selects the database "1".
- The next character "a" selects the bucket "a" within the "1" database.
- The remaining hash "2b3c4d5e6f0..." is used as the key in the "a" bucket based on the specified `KeyLen`.

By following this approach, you can efficiently organize and retrieve data based on Message-ID hashes while benefiting from the performance and storage optimizations provided by BoltDB.

Feel free to customize the `KeyLen` setting to meet your specific performance and storage requirements.

Smaller `KeyLen` values save space but may result in more disk access, while larger `KeyLen` values reduce disk access but consume more space.


## File Descriptors: 33 (+ 3 /dev/pts + 1 anon_inode: + 2 pipe: ?) = 39

- Writer History.dat: 1
- HashDB: 16 + 16 Fseeks

```sh
ls -lha /proc/$(pidof nntp-history-test)/fd

# counts all open FDs
ls -l /proc/$(pidof nntp-history-test)/fd | wc -l

# count open history.dat
ls -lha /proc/$(pidof nntp-history-test)/fd|grep history.dat$|wc -l

# count open hashdb
ls -lha /proc/$(pidof nntp-history-test)/fd|grep history.dat.hash|wc -l
```


## BBoltDB Statistics

When you retrieve and examine the statistics (e.g., by using the Stats method) of a BoltDB (bbolt) database in Go, you can gather valuable information about the database's performance, resource usage, and structure. The statistics provide insights into how the database is operating and can be useful for optimizing your application. Here are some of the key metrics you can see from BoltDB database statistics:

Number of Buckets: The total number of buckets in the database. Each bucket is essentially a separate namespace for key-value pairs.

Number of Keys: The total number of keys stored in the database. This metric can help you understand the size of your dataset.

Number of Data Pages: The total number of data pages used by the database. Data pages store key-value pairs.

Number of Leaf Pages: The total number of leaf pages in the database. Leaf pages contain key-value pairs directly.

Number of Branch Pages: The total number of branch pages in the database. Branch pages are used for indexing and navigating to leaf pages.

Page Size: The size of each page in bytes. This information can be helpful for understanding memory usage.

Bucket Page Size: The average size of bucket pages in bytes. This metric can provide insights into how efficiently your buckets are organized.

Leaf Page Size: The average size of leaf pages in bytes. This helps you understand the size of the data itself.

Branch Page Size: The average size of branch pages in bytes. This metric can be useful for optimizing indexing.

Allocated Pages: The total number of pages allocated by the database. This can be helpful for monitoring resource usage.

Freed Pages: The total number of pages that have been freed or released. This metric can indicate how efficiently the database manages space.

Page Rebalance: The number of times pages have been rebalanced between sibling branches. This is relevant for understanding how the database maintains a balanced tree structure.

Transaction Stats: Information about transactions, including the number of started and committed transactions.

Page Cache Stats: Metrics related to the page cache, including the number of hits and misses, and the size of the cache.

Free Page Ns: The number of free pages per page allocation size. This provides insight into the fragmentation of free space.

These statistics can help you monitor and optimize your BoltDB database. For example, you can use them to identify performance bottlenecks, understand resource usage patterns, and assess the efficiency of your data organization. Depending on your application's specific requirements, you may want to focus on certain metrics more than others.


## Contributing

Contributions to this code are welcome.

If you have suggestions for improvements or find issues, please feel free to open an issue or submit a pull request.

## License

This code is provided under the MIT License. See the [LICENSE](LICENSE) file for details.



## Benchmark pure writes (no dupe check via hashdb) to history file with 4K bufio.
```sh
./nntp-history-test -useHashDB=false -useGoCache=true
CPU=4/12 | useHashDB: false | useGoCache: true | jobs=4 | todo=1000000 | total=4000000 | keyalgo=11 | keylen=6 | BatchSize=1024
2023/09/29 15:45:46 History: HF='history/history.dat' DB='hashdb/history.dat.hash' C='&{0xc000104f00}' HT=11 HL=6
...
2023/09/29 15:45:52 history_Writer closed fp='history/history.dat' wbt=108148560 offset=108148622 wroteLines=1060280
2023/09/29 15:45:52 key_add=0 key_app=0 total=0
2023/09/29 15:45:52 done=4000000 took 6 seconds
```

## Inserting 4.000.000 `i` hashes (75% duplicates) to history and hashdb + history.DBG_BS_LOG = true
```sh
./nntp-history-test
ARGS: CPU=4/12 | jobs=4 | todo=1000000 | total=4000000 | keyalgo=11 | keylen=6 | BatchSize=1024
 useHashDB: true | IndexParallel=16
 boltOpts='&bbolt.Options{Timeout:9000000000, NoGrowSync:false, NoFreelistSync:false, PreLoadFreelist:false, FreelistType:"", ReadOnly:false, MmapFlags:0, InitialMmapSize:2147483648, PageSize:65536, NoSync:false, OpenFile:(func(string, int, fs.FileMode) (*os.File, error))(nil), Mlock:false}'
2023/10/08 14:31:30 History: new=true
  HF='history/history.dat' DB='hashdb/history.dat.hash.[0-9a-f]'
  KeyAlgo=11 KeyLen=6 NumQueueWriteChan=16
  HashDBQueues:{NumQueueIndexChan=16 NumQueueIndexChans=4 BatchSize=1024 IndexParallel=16}
2023/10/08 14:31:52 End test p=4 nntp-history added=250339 dupes=0 cLock=423214 addretry=0 retry=0 adddupes=0 cdupes=326447 cretry1=0 sum=1000000/1000000 errors=0 locked=250339
2023/10/08 14:31:52 End test p=3 nntp-history added=249431 dupes=0 cLock=423462 addretry=0 retry=0 adddupes=0 cdupes=327107 cretry1=0 sum=1000000/1000000 errors=0 locked=249431
2023/10/08 14:31:52 End test p=1 nntp-history added=251125 dupes=0 cLock=423355 addretry=0 retry=0 adddupes=0 cdupes=325520 cretry1=0 sum=1000000/1000000 errors=0 locked=251125
2023/10/08 14:31:52 End test p=2 nntp-history added=249105 dupes=0 cLock=423066 addretry=0 retry=0 adddupes=0 cdupes=327829 cretry1=0 sum=1000000/1000000 errors=0 locked=249105
2023/10/08 14:31:52 CLOSE_HISTORY: his.WriterChan <- nil
2023/10/08 14:31:52 WAIT CLOSE_HISTORY: lock1=true=1 lock2=true=1 lock3=true=16 lock4=true=16 lock5=true=256 batchQueued=true=48042 batchLocked=false=0
2023/10/08 14:31:53 WAIT CLOSE_HISTORY: lock1=false=0 lock2=false=0 lock3=true=16 lock4=true=16 lock5=true=256 batchQueued=true=45440 batchLocked=true=16
2023/10/08 14:31:54 WAIT CLOSE_HISTORY: lock1=false=0 lock2=false=0 lock3=true=16 lock4=true=16 lock5=true=256 batchQueued=true=18102 batchLocked=true=16
2023/10/08 14:31:55 Quit HDBZW char=e added=62835 passed=0 dupes=0 processed=62835 searches=62835 retry=0
2023/10/08 14:31:55 Quit HDBZW char=6 added=62455 passed=0 dupes=0 processed=62455 searches=62455 retry=0
2023/10/08 14:31:55 Quit HDBZW char=5 added=62388 passed=0 dupes=0 processed=62388 searches=62388 retry=0
2023/10/08 14:31:55 Quit HDBZW char=8 added=62452 passed=0 dupes=0 processed=62452 searches=62452 retry=0
2023/10/08 14:31:55 Quit HDBZW char=d added=62645 passed=0 dupes=0 processed=62645 searches=62645 retry=0
2023/10/08 14:31:55 Quit HDBZW char=4 added=62322 passed=0 dupes=0 processed=62322 searches=62322 retry=0
2023/10/08 14:31:55 Quit HDBZW char=2 added=62530 passed=0 dupes=0 processed=62530 searches=62530 retry=0
2023/10/08 14:31:55 Quit HDBZW char=f added=62371 passed=0 dupes=0 processed=62371 searches=62371 retry=0
2023/10/08 14:31:55 Quit HDBZW char=a added=62633 passed=0 dupes=0 processed=62633 searches=62633 retry=0
2023/10/08 14:31:55 Quit HDBZW char=c added=62497 passed=0 dupes=0 processed=62497 searches=62497 retry=0
2023/10/08 14:31:55 Quit HDBZW char=9 added=62851 passed=0 dupes=0 processed=62851 searches=62851 retry=0
2023/10/08 14:31:55 Quit HDBZW char=1 added=62477 passed=0 dupes=0 processed=62477 searches=62477 retry=0
2023/10/08 14:31:55 Quit HDBZW char=3 added=62122 passed=0 dupes=0 processed=62122 searches=62122 retry=0
2023/10/08 14:31:55 Quit HDBZW char=7 added=62625 passed=0 dupes=0 processed=62625 searches=62625 retry=0
2023/10/08 14:31:55 Quit HDBZW char=0 added=62326 passed=0 dupes=0 processed=62326 searches=62326 retry=0
2023/10/08 14:31:55 Quit HDBZW char=b added=62471 passed=0 dupes=0 processed=62471 searches=62471 retry=0
2023/10/08 14:31:55 WAIT CLOSE_HISTORY: lock1=false=0 lock2=false=0 lock3=false=0 lock4=false=0 lock5=true=256 batchQueued=true=256 batchLocked=false=0
2023/10/08 14:31:56 WAIT CLOSE_HISTORY: lock1=false=0 lock2=false=0 lock3=false=0 lock4=false=0 lock5=true=137 batchQueued=true=137 batchLocked=false=0
2023/10/08 14:31:57 CLOSE_HISTORY DONE
2023/10/08 14:31:57 key_add=999886 key_app=114 total=1000000 fseeks=0 eof=0 BoltDB_decodedOffsets=0 addoffset=0 appoffset=114 trymultioffsets=0 tryoffset=114 searches=1000000 inserted=1000000
2023/10/08 14:31:57 L1LOCK=1000000 | Get: L2=228 L3=1000114 | wCBBS=~995 conti=4 slept=49
2023/10/08 14:31:57 done=4000000 (took 22 seconds) (closewait 5 seconds)
2023/10/08 14:31:57 CrunchBatchLogs: did=1280 dat=1280
2023/10/08 14:31:57 CrunchLogs: BatchSize=00072:01024 t=00000033780:00001067766 µs
2023/10/08 14:31:57 Percentile 005%: BatchSize=00892:01024 t=00000039920:00000116395 µs
2023/10/08 14:31:57 Percentile 010%: BatchSize=00924:01014 t=00000050615:00000137622 µs
2023/10/08 14:31:57 Percentile 015%: BatchSize=00890:01024 t=00000047628:00000163336 µs
2023/10/08 14:31:57 Percentile 020%: BatchSize=00823:01024 t=00000035025:00000413510 µs
2023/10/08 14:31:57 Percentile 025%: BatchSize=00842:00999 t=00000033780:00000113772 µs
2023/10/08 14:31:57 Percentile 030%: BatchSize=00800:00989 t=00000048332:00000127602 µs
2023/10/08 14:31:57 Percentile 035%: BatchSize=00818:00980 t=00000039240:00000136068 µs
2023/10/08 14:31:57 Percentile 040%: BatchSize=00856:01016 t=00000038223:00000129415 µs
2023/10/08 14:31:57 Percentile 045%: BatchSize=00840:01000 t=00000039680:00000065512 µs
2023/10/08 14:31:57 Percentile 050%: BatchSize=00843:00999 t=00000065365:00001025821 µs
2023/10/08 14:31:57 Percentile 055%: BatchSize=00839:00967 t=00000767486:00001040160 µs
2023/10/08 14:31:57 Percentile 060%: BatchSize=00767:00982 t=00000039710:00001067766 µs
2023/10/08 14:31:57 Percentile 065%: BatchSize=00772:00910 t=00000050663:00000140897 µs
2023/10/08 14:31:57 Percentile 070%: BatchSize=00814:01000 t=00000046258:00000165986 µs
2023/10/08 14:31:57 Percentile 075%: BatchSize=00932:01000 t=00000057387:00000145437 µs
2023/10/08 14:31:57 Percentile 080%: BatchSize=00114:01000 t=00000067834:00000158888 µs
2023/10/08 14:31:57 Percentile 085%: BatchSize=00096:00309 t=00000101673:00000110693 µs
2023/10/08 14:31:57 Percentile 090%: BatchSize=00093:00337 t=00000101597:00000108680 µs
2023/10/08 14:31:57 Percentile 095%: BatchSize=00072:00331 t=00000101639:00000124072 µs
2023/10/08 14:31:57 Percentile 100%: BatchSize=00084:00331 t=00000101301:00000134560 µs
```

## Checking 4.000.000 `i` hashes (75% duplicates) vs hashdb
```sh
./nntp-history-test
ARGS: CPU=4/12 | jobs=4 | todo=1000000 | total=4000000 | keyalgo=11 | keylen=6 | BatchSize=1024
 useHashDB: true | IndexParallel=16
 boltOpts='&bbolt.Options{Timeout:9000000000, NoGrowSync:false, NoFreelistSync:false, PreLoadFreelist:false, FreelistType:"", ReadOnly:false, MmapFlags:0, InitialMmapSize:2147483648, PageSize:65536, NoSync:false, OpenFile:(func(string, int, fs.FileMode) (*os.File, error))(nil), Mlock:false}'
2023/10/08 14:32:32 History: new=false
  HF='history/history.dat' DB='hashdb/history.dat.hash.[0-9a-f]'
  KeyAlgo=11 KeyLen=6 NumQueueWriteChan=16
  HashDBQueues:{NumQueueIndexChan=16 NumQueueIndexChans=4 BatchSize=1024 IndexParallel=16}
2023/10/08 14:32:56 End test p=2 nntp-history added=0 dupes=249482 cLock=547547 addretry=0 retry=0 adddupes=0 cdupes=202971 cretry1=0 sum=1000000/1000000 errors=0 locked=249482
2023/10/08 14:32:56 End test p=3 nntp-history added=0 dupes=250551 cLock=548727 addretry=0 retry=0 adddupes=0 cdupes=200722 cretry1=0 sum=1000000/1000000 errors=0 locked=250551
2023/10/08 14:32:56 End test p=4 nntp-history added=0 dupes=249623 cLock=547928 addretry=0 retry=0 adddupes=0 cdupes=202449 cretry1=0 sum=1000000/1000000 errors=0 locked=249623
2023/10/08 14:32:56 End test p=1 nntp-history added=0 dupes=250344 cLock=548689 addretry=0 retry=0 adddupes=0 cdupes=200967 cretry1=0 sum=1000000/1000000 errors=0 locked=250344
2023/10/08 14:32:56 CLOSE_HISTORY: his.WriterChan <- nil
2023/10/08 14:32:56 WAIT CLOSE_HISTORY: lock1=true=1 lock2=true=1 lock3=true=16 lock4=true=16 lock5=true=256 batchQueued=false=0 batchLocked=false=0
2023/10/08 14:32:56 Quit HDBZW char=f added=0 passed=0 dupes=0 processed=0 searches=62371 retry=0
2023/10/08 14:32:56 Quit HDBZW char=9 added=0 passed=0 dupes=0 processed=0 searches=62851 retry=0
2023/10/08 14:32:56 Quit HDBZW char=7 added=0 passed=0 dupes=0 processed=0 searches=62625 retry=0
2023/10/08 14:32:56 Quit HDBZW char=b added=0 passed=0 dupes=0 processed=0 searches=62471 retry=0
2023/10/08 14:32:56 Quit HDBZW char=8 added=0 passed=0 dupes=0 processed=0 searches=62452 retry=0
2023/10/08 14:32:56 Quit HDBZW char=0 added=0 passed=0 dupes=0 processed=0 searches=62326 retry=0
2023/10/08 14:32:56 Quit HDBZW char=1 added=0 passed=0 dupes=0 processed=0 searches=62477 retry=0
2023/10/08 14:32:56 Quit HDBZW char=a added=0 passed=0 dupes=0 processed=0 searches=62633 retry=0
2023/10/08 14:32:56 Quit HDBZW char=2 added=0 passed=0 dupes=0 processed=0 searches=62530 retry=0
2023/10/08 14:32:56 Quit HDBZW char=3 added=0 passed=0 dupes=0 processed=0 searches=62122 retry=0
2023/10/08 14:32:56 Quit HDBZW char=c added=0 passed=0 dupes=0 processed=0 searches=62497 retry=0
2023/10/08 14:32:56 Quit HDBZW char=d added=0 passed=0 dupes=0 processed=0 searches=62645 retry=0
2023/10/08 14:32:56 Quit HDBZW char=4 added=0 passed=0 dupes=0 processed=0 searches=62322 retry=0
2023/10/08 14:32:56 Quit HDBZW char=e added=0 passed=0 dupes=0 processed=0 searches=62835 retry=0
2023/10/08 14:32:56 Quit HDBZW char=5 added=0 passed=0 dupes=0 processed=0 searches=62388 retry=0
2023/10/08 14:32:56 Quit HDBZW char=6 added=0 passed=0 dupes=0 processed=0 searches=62455 retry=0
2023/10/08 14:32:57 CLOSE_HISTORY DONE
2023/10/08 14:32:57 key_add=0 key_app=0 total=0 fseeks=1000000 eof=0 BoltDB_decodedOffsets=999886 addoffset=0 appoffset=0 trymultioffsets=228 tryoffset=999772 searches=1000000 inserted=0
2023/10/08 14:32:57 L1LOCK=1000000 | Get: L2=114 L3=114 | wCBBS=~1024 conti=0 slept=48
2023/10/08 14:32:57 done=4000000 (took 24 seconds) (closewait 1 seconds)
```

## Inserting 400.000.000 `i` hashes (75% duplicates) to history and hashdb (adaptive batchsize)
```sh
./nntp-history-test -todo 100000000
ARGS: CPU=4/12 | jobs=4 | todo=100000000 | total=400000000 | keyalgo=11 | keylen=6 | BatchSize=1024
 useHashDB: true | IndexParallel=16
 boltOpts='&bbolt.Options{Timeout:9000000000, NoGrowSync:false, NoFreelistSync:false, PreLoadFreelist:false, FreelistType:"", ReadOnly:false, MmapFlags:0, InitialMmapSize:2147483648, PageSize:65536, NoSync:false, OpenFile:(func(string, int, fs.FileMode) (*os.File, error))(nil), Mlock:false}'
2023/10/08 18:01:29 History: new=true
  HF='history/history.dat' DB='hashdb/history.dat.hash.[0-9a-f]'
  KeyAlgo=11 KeyLen=6 NumQueueWriteChan=16
  HashDBQueues:{NumQueueIndexChan=16 NumQueueIndexChans=4 BatchSize=1024 IndexParallel=16}
2023/10/08 18:01:44 BoltSpeed: 44297.19 tx/s ( did=664453 in 15.0 sec ) totalTX=664453
2023/10/08 18:01:59 BoltSpeed: 43363.11 tx/s ( did=650447 in 15.0 sec ) totalTX=1314900
...
2023/10/08 18:06:00 RUN test p=4 nntp-history added=2492416 dupes=0 cLock=4021442 addretry=0 retry=0 adddupes=0 cdupes=3486142 cretry1=0 10000000/100000000
2023/10/08 18:06:00 RUN test p=3 nntp-history added=2502228 dupes=0 cLock=4025283 addretry=0 retry=0 adddupes=0 cdupes=3472489 cretry1=0 10000000/100000000
2023/10/08 18:06:00 RUN test p=1 nntp-history added=2501394 dupes=0 cLock=4021422 addretry=0 retry=0 adddupes=0 cdupes=3477184 cretry1=0 10000000/100000000
2023/10/08 18:06:00 RUN test p=2 nntp-history added=2503962 dupes=0 cLock=4020087 addretry=0 retry=0 adddupes=0 cdupes=3475951 cretry1=0 10000000/100000000
...
2023/10/08 18:08:29 BoltSpeed: 28428.51 tx/s ( did=426429 in 15.0 sec ) totalTX=14539993
2023/10/08 18:08:44 BoltSpeed: 28750.23 tx/s ( did=431270 in 15.0 sec ) totalTX=14971263
...
2023/10/08 18:12:00 RUN test p=3 nntp-history added=5006126 dupes=0 cLock=7599269 addretry=0 retry=0 adddupes=0 cdupes=7394605 cretry1=0 20000000/100000000
2023/10/08 18:12:00 RUN test p=4 nntp-history added=4988108 dupes=0 cLock=7590973 addretry=0 retry=0 adddupes=0 cdupes=7420919 cretry1=0 20000000/100000000
2023/10/08 18:12:00 RUN test p=1 nntp-history added=5007827 dupes=0 cLock=7592780 addretry=0 retry=0 adddupes=0 cdupes=7399393 cretry1=0 20000000/100000000
2023/10/08 18:12:00 RUN test p=2 nntp-history added=4997939 dupes=0 cLock=7589744 addretry=0 retry=0 adddupes=0 cdupes=7412317 cretry1=0 20000000/100000000
...
2023/10/08 18:19:29 BoltSpeed: 18166.40 tx/s ( did=272511 in 15.0 sec ) totalTX=27857110
2023/10/08 18:19:44 BoltSpeed: 13151.54 tx/s ( did=197255 in 15.0 sec ) totalTX=28054365
...
2023/10/08 18:22:12 RUN test p=4 nntp-history added=7475027 dupes=0 cLock=10624599 addretry=0 retry=0 adddupes=0 cdupes=11900374 cretry1=0 30000000/100000000
2023/10/08 18:22:12 RUN test p=2 nntp-history added=7488813 dupes=0 cLock=10623678 addretry=0 retry=0 adddupes=0 cdupes=11887509 cretry1=0 30000000/100000000
2023/10/08 18:22:12 RUN test p=3 nntp-history added=7513602 dupes=0 cLock=10636308 addretry=0 retry=0 adddupes=0 cdupes=11850090 cretry1=0 30000000/100000000
2023/10/08 18:22:12 RUN test p=1 nntp-history added=7522558 dupes=0 cLock=10629385 addretry=0 retry=0 adddupes=0 cdupes=11848057 cretry1=0 30000000/100000000
...
2023/10/08 18:22:44 BoltSpeed: 14160.60 tx/s ( did=212199 in 15.0 sec ) totalTX=30427951
2023/10/08 18:22:59 BoltSpeed: 14050.44 tx/s ( did=210745 in 15.0 sec ) totalTX=30638696
...
2023/10/08 18:29:29 BoltSpeed: 14335.57 tx/s ( did=215021 in 15.0 sec ) totalTX=35436346
2023/10/08 18:29:44 BoltSpeed: 16308.28 tx/s ( did=244628 in 15.0 sec ) totalTX=35680974
...
2023/10/08 18:36:16 RUN test p=1 nntp-history added=9997081 dupes=0 cLock=13404293 addretry=0 retry=0 adddupes=0 cdupes=16598626 cretry1=0 40000000/100000000
2023/10/08 18:36:16 RUN test p=2 nntp-history added=9976454 dupes=0 cLock=13397623 addretry=0 retry=0 adddupes=0 cdupes=16625923 cretry1=0 40000000/100000000
2023/10/08 18:36:16 RUN test p=3 nntp-history added=10041923 dupes=0 cLock=13412518 addretry=0 retry=0 adddupes=0 cdupes=16545559 cretry1=0 40000000/100000000
2023/10/08 18:36:16 RUN test p=4 nntp-history added=9984542 dupes=0 cLock=13399590 addretry=0 retry=0 adddupes=0 cdupes=16615868 cretry1=0 40000000/100000000
...
2023/10/08 18:36:29 BoltSpeed: 15694.29 tx/s ( did=235991 in 15.0 sec ) totalTX=40183347
2023/10/08 18:36:44 BoltSpeed: 10035.14 tx/s ( did=150161 in 15.0 sec ) totalTX=40333508
...
2023/10/08 18:44:29 BoltSpeed: 11611.33 tx/s ( did=174149 in 15.0 sec ) totalTX=45049808
2023/10/08 18:44:59 BoltSpeed: 11397.50 tx/s ( did=170961 in 15.0 sec ) totalTX=45311296
...
2023/10/08 18:54:18 RUN test p=2 nntp-history added=12484775 dupes=0 cLock=15895032 addretry=0 retry=0 adddupes=0 cdupes=21620193 cretry1=0 50000000/100000000
2023/10/08 18:54:18 RUN test p=4 nntp-history added=12472563 dupes=0 cLock=15898842 addretry=0 retry=0 adddupes=0 cdupes=21628595 cretry1=0 50000000/100000000
2023/10/08 18:54:18 RUN test p=3 nntp-history added=12527524 dupes=0 cLock=15913534 addretry=0 retry=0 adddupes=0 cdupes=21558942 cretry1=0 50000000/100000000
2023/10/08 18:54:18 RUN test p=1 nntp-history added=12515138 dupes=0 cLock=15902903 addretry=0 retry=0 adddupes=0 cdupes=21581959 cretry1=0 50000000/100000000
...
2023/10/08 18:54:29 BoltSpeed: 7503.50 tx/s ( did=112085 in 14.9 sec ) totalTX=50063485
2023/10/08 18:54:44 BoltSpeed: 6241.39 tx/s ( did=93632 in 15.0 sec ) totalTX=50157117
...
2023/10/08 18:56:29 BoltSpeed: 5856.39 tx/s ( did=87851 in 15.0 sec ) totalTX=51078718
2023/10/08 18:56:44 BoltSpeed: 11830.49 tx/s ( did=177445 in 15.0 sec ) totalTX=51256163
...
2023/10/08 19:05:14 BoltSpeed: 5877.14 tx/s ( did=88158 in 15.0 sec ) totalTX=55384061
2023/10/08 19:05:29 BoltSpeed: 8024.26 tx/s ( did=120365 in 15.0 sec ) totalTX=55504426
...
2023/10/08 19:08:59 BoltSpeed: 10554.54 tx/s ( did=158213 in 15.0 sec ) totalTX=57268414
2023/10/08 19:09:14 BoltSpeed: 5989.68 tx/s ( did=89846 in 15.0 sec ) totalTX=57358260
...
2023/10/08 19:15:15 RUN test p=2 nntp-history added=14969427 dupes=0 cLock=18341406 addretry=0 retry=0 adddupes=0 cdupes=26689167 cretry1=0 60000000/100000000
2023/10/08 19:15:15 RUN test p=4 nntp-history added=14981366 dupes=0 cLock=18345452 addretry=0 retry=0 adddupes=0 cdupes=26673182 cretry1=0 60000000/100000000
2023/10/08 19:15:15 RUN test p=3 nntp-history added=15034644 dupes=0 cLock=18360978 addretry=0 retry=0 adddupes=0 cdupes=26604378 cretry1=0 60000000/100000000
2023/10/08 19:15:15 RUN test p=1 nntp-history added=15014563 dupes=0 cLock=18347434 addretry=0 retry=0 adddupes=0 cdupes=26638003 cretry1=0 60000000/100000000
...
2023/10/08 19:15:44 BoltSpeed: 6035.35 tx/s ( did=90529 in 15.0 sec ) totalTX=60172183
2023/10/08 19:15:59 BoltSpeed: 9167.37 tx/s ( did=137540 in 15.0 sec ) totalTX=60309723
...
2023/10/08 19:20:44 BoltSpeed: 6798.99 tx/s ( did=101995 in 15.0 sec ) totalTX=62408863
2023/10/08 19:20:59 BoltSpeed: 9815.17 tx/s ( did=147205 in 15.0 sec ) totalTX=62556068
...
2023/10/08 19:36:13 RUN test p=2 nntp-history added=17482701 dupes=0 cLock=20749116 addretry=0 retry=0 adddupes=0 cdupes=31768183 cretry1=0 70000000/100000000
2023/10/08 19:36:13 RUN test p=4 nntp-history added=17480194 dupes=0 cLock=20753972 addretry=0 retry=0 adddupes=0 cdupes=31765834 cretry1=0 70000000/100000000
2023/10/08 19:36:13 RUN test p=1 nntp-history added=17506386 dupes=0 cLock=20754704 addretry=0 retry=0 adddupes=0 cdupes=31738910 cretry1=0 70000000/100000000
2023/10/08 19:36:13 RUN test p=3 nntp-history added=17530719 dupes=0 cLock=20768795 addretry=0 retry=0 adddupes=0 cdupes=31700486 cretry1=0 70000000/100000000
...
2023/10/08 19:36:59 BoltSpeed: 5951.88 tx/s ( did=89281 in 15.0 sec ) totalTX=70469109
2023/10/08 19:37:14 BoltSpeed: 5866.55 tx/s ( did=87995 in 15.0 sec ) totalTX=70557104
...
2023/10/08 19:47:59 BoltSpeed: 6033.13 tx/s ( did=90494 in 15.0 sec ) totalTX=75475427
2023/10/08 19:48:14 BoltSpeed: 10104.82 tx/s ( did=151577 in 15.0 sec ) totalTX=75627004
...
2023/10/08 19:56:14 BoltSpeed: 12523.30 tx/s ( did=187842 in 15.0 sec ) totalTX=79567483
2023/10/08 19:56:29 BoltSpeed: 12160.01 tx/s ( did=182432 in 15.0 sec ) totalTX=79749915
...
2023/10/08 19:57:09 RUN test p=1 nntp-history added=19999689 dupes=0 cLock=23142298 addretry=0 retry=0 adddupes=0 cdupes=36858013 cretry1=0 80000000/100000000
2023/10/08 19:57:09 RUN test p=4 nntp-history added=19974453 dupes=0 cLock=23139840 addretry=0 retry=0 adddupes=0 cdupes=36885707 cretry1=0 80000000/100000000
2023/10/08 19:57:09 RUN test p=2 nntp-history added=19990713 dupes=0 cLock=23136754 addretry=0 retry=0 adddupes=0 cdupes=36872533 cretry1=0 80000000/100000000
2023/10/08 19:57:09 RUN test p=3 nntp-history added=20035145 dupes=0 cLock=23157072 addretry=0 retry=0 adddupes=0 cdupes=36807783 cretry1=0 80000000/100000000
...
2023/10/08 20:10:59 BoltSpeed: 5227.70 tx/s ( did=78454 in 15.0 sec ) totalTX=85651938
2023/10/08 20:11:14 BoltSpeed: 7465.34 tx/s ( did=111926 in 15.0 sec ) totalTX=85763864
...
2023/10/08 20:21:50 RUN test p=2 nntp-history added=22495420 dupes=0 cLock=25537568 addretry=0 retry=0 adddupes=0 cdupes=41967012 cretry1=0 90000000/100000000
2023/10/08 20:21:50 RUN test p=1 nntp-history added=22494903 dupes=0 cLock=25545634 addretry=0 retry=0 adddupes=0 cdupes=41959463 cretry1=0 90000000/100000000
2023/10/08 20:21:50 RUN test p=3 nntp-history added=22538428 dupes=0 cLock=25560415 addretry=0 retry=0 adddupes=0 cdupes=41901157 cretry1=0 90000000/100000000
2023/10/08 20:21:50 RUN test p=4 nntp-history added=22471249 dupes=0 cLock=25541123 addretry=0 retry=0 adddupes=0 cdupes=41987628 cretry1=0 90000000/100000000
...
2023/10/08 20:22:14 BoltSpeed: 8337.85 tx/s ( did=125053 in 15.0 sec ) totalTX=90188001
2023/10/08 20:22:29 BoltSpeed: 11329.34 tx/s ( did=169928 in 15.0 sec ) totalTX=90357929
...
2023/10/08 20:33:59 BoltSpeed: 7646.71 tx/s ( did=114697 in 15.0 sec ) totalTX=95321186
2023/10/08 20:34:14 BoltSpeed: 11713.64 tx/s ( did=175825 in 15.0 sec ) totalTX=95497011
...
2023/10/08 20:33:59 BoltSpeed: 7646.71 tx/s ( did=114697 in 15.0 sec ) totalTX=95321186
2023/10/08 20:34:14 BoltSpeed: 11713.64 tx/s ( did=175825 in 15.0 sec ) totalTX=95497011
...
2023/10/08 20:45:24 End test p=4 nntp-history added=24985526 dupes=0 cLock=27846437 addretry=0 retry=0 adddupes=0 cdupes=47168037 cretry1=0 sum=100000000/100000000 errors=0 locked=24985526
2023/10/08 20:45:24 End test p=3 nntp-history added=25028730 dupes=0 cLock=27866312 addretry=0 retry=0 adddupes=0 cdupes=47104958 cretry1=0 sum=100000000/100000000 errors=0 locked=25028730
2023/10/08 20:45:24 End test p=1 nntp-history added=24995308 dupes=0 cLock=27850749 addretry=0 retry=0 adddupes=0 cdupes=47153943 cretry1=0 sum=100000000/100000000 errors=0 locked=24995308
2023/10/08 20:45:24 End test p=2 nntp-history added=24990436 dupes=0 cLock=27839153 addretry=0 retry=0 adddupes=0 cdupes=47170411 cretry1=0 sum=100000000/100000000 errors=0 locked=24990436
...
2023/10/08 20:45:24 CLOSE_HISTORY: his.WriterChan <- nil
2023/10/08 20:45:24 WAIT CLOSE_HISTORY: lock1=true=1 lock2=true=1 lock3=true=16 lock4=true=16 lock5=true=256 batchQueued=true=32689 batchLocked=true=52
2023/10/08 20:45:25 WAIT CLOSE_HISTORY: lock1=false=0 lock2=false=0 lock3=true=16 lock4=true=16 lock5=true=256 batchQueued=true=20232 batchLocked=true=95
2023/10/08 20:45:26 WAIT CLOSE_HISTORY: lock1=false=0 lock2=false=0 lock3=true=16 lock4=true=16 lock5=true=256 batchQueued=true=11616 batchLocked=true=129
2023/10/08 20:45:27 WAIT CLOSE_HISTORY: lock1=false=0 lock2=false=0 lock3=true=16 lock4=true=16 lock5=true=255 batchQueued=true=5006 batchLocked=true=165
2023/10/08 20:45:28 WAIT CLOSE_HISTORY: lock1=false=0 lock2=false=0 lock3=true=16 lock4=true=16 lock5=true=254 batchQueued=true=2576 batchLocked=true=179
2023/10/08 20:45:29 WAIT CLOSE_HISTORY: lock1=false=0 lock2=false=0 lock3=true=16 lock4=true=16 lock5=true=247 batchQueued=true=1992 batchLocked=true=167
2023/10/08 20:45:29 BoltSpeed: 6857.35 tx/s ( did=102877 in 15.0 sec ) totalTX=100004465
Alloc: 692 MiB, TotalAlloc: 6007858 MiB, Sys: 2953 MiB, NumGC: 9237
2023/10/08 20:45:30 WAIT CLOSE_HISTORY: lock1=false=0 lock2=false=0 lock3=true=16 lock4=true=16 lock5=true=238 batchQueued=true=955 batchLocked=true=174
2023/10/08 20:45:31 WAIT CLOSE_HISTORY: lock1=false=0 lock2=false=0 lock3=true=16 lock4=true=16 lock5=true=226 batchQueued=true=650 batchLocked=true=168
2023/10/08 20:45:32 WAIT CLOSE_HISTORY: lock1=false=0 lock2=false=0 lock3=true=16 lock4=true=16 lock5=true=205 batchQueued=true=394 batchLocked=true=156
2023/10/08 20:45:33 WAIT CLOSE_HISTORY: lock1=false=0 lock2=false=0 lock3=true=16 lock4=true=16 lock5=true=164 batchQueued=true=281 batchLocked=true=128
2023/10/08 20:45:34 WAIT CLOSE_HISTORY: lock1=false=0 lock2=false=0 lock3=true=16 lock4=true=16 lock5=true=126 batchQueued=true=233 batchLocked=true=105
2023/10/08 20:45:35 WAIT CLOSE_HISTORY: lock1=false=0 lock2=false=0 lock3=true=14 lock4=true=14 lock5=true=86 batchQueued=true=191 batchLocked=true=70
2023/10/08 20:45:36 WAIT CLOSE_HISTORY: lock1=false=0 lock2=false=0 lock3=true=8 lock4=true=8 lock5=true=35 batchQueued=true=94 batchLocked=true=27
2023/10/08 20:45:37 WAIT CLOSE_HISTORY: lock1=false=0 lock2=false=0 lock3=false=0 lock4=false=0 lock5=true=1 batchQueued=true=1 batchLocked=false=0
2023/10/08 20:45:38 CLOSE_HISTORY DONE
2023/10/08 20:45:38 key_add=98845007 key_app=1154993 total=100000000 fseeks=1158068 eof=0 BoltDB_decodedOffsets=1148962 addoffset=0 appoffset=1154993 trymultioffsets=9085 tryoffset=1145908 searches=100000000 inserted=100000000
2023/10/08 20:45:38 L1LOCK=100000000 | Get: L2=1170186 L3=100006031 | wCBBS=~246 conti=1350 slept=11699
2023/10/08 20:45:38 done=400000000 (took 9835 seconds) (closewait 14 seconds)
2023/10/08 20:45:38 CrunchBatchLogs: did=345935 dat=345935
2023/10/08 20:45:38 CrunchLogs: inserted=00001:01024 wCBBS=00168:01032 µs=0000004362:0023190244
2023/10/08 20:45:38 Range 005%: inserted=00465:01024 wCBBS=00552:01032 µs=0000004362:0000788742 sum=20757
2023/10/08 20:45:38 Range 010%: inserted=00140:00624 wCBBS=00408:00632 µs=0000022077:0004835957 sum=17296
2023/10/08 20:45:38 Range 015%: inserted=00118:00480 wCBBS=00328:00488 µs=0000025653:0008485567 sum=17297
2023/10/08 20:45:38 Range 020%: inserted=00123:00416 wCBBS=00296:00424 µs=0000032572:0009806887 sum=17297
2023/10/08 20:45:38 Range 025%: inserted=00101:00384 wCBBS=00288:00392 µs=0000032892:0012528983 sum=17297
2023/10/08 20:45:38 Range 030%: inserted=00113:00368 wCBBS=00248:00376 µs=0000036213:0011404832 sum=17296
2023/10/08 20:45:38 Range 035%: inserted=00096:00328 wCBBS=00216:00336 µs=0000031375:0012153490 sum=17297
2023/10/08 20:45:38 Range 040%: inserted=00094:00304 wCBBS=00200:00312 µs=0000033563:0011843502 sum=17297
2023/10/08 20:45:38 Range 045%: inserted=00097:00312 wCBBS=00208:00320 µs=0000032448:0013329855 sum=17297
2023/10/08 20:45:38 Range 050%: inserted=00093:00296 wCBBS=00192:00304 µs=0000036156:0013161688 sum=17296
2023/10/08 20:45:38 Range 055%: inserted=00092:00312 wCBBS=00192:00320 µs=0000037472:0017085266 sum=17297
2023/10/08 20:45:38 Range 060%: inserted=00104:00320 wCBBS=00200:00328 µs=0000024703:0017085998 sum=17297
2023/10/08 20:45:38 Range 065%: inserted=00086:00320 wCBBS=00216:00328 µs=0000037516:0023190244 sum=17297
2023/10/08 20:45:38 Range 070%: inserted=00102:00304 wCBBS=00216:00312 µs=0000040803:0017648534 sum=17296
2023/10/08 20:45:38 Range 075%: inserted=00088:00323 wCBBS=00200:00328 µs=0000034907:0017675761 sum=17297
2023/10/08 20:45:38 Range 080%: inserted=00084:00288 wCBBS=00192:00296 µs=0000038198:0016353364 sum=17297
2023/10/08 20:45:38 Range 085%: inserted=00080:00290 wCBBS=00168:00296 µs=0000034792:0017551815 sum=17297
2023/10/08 20:45:38 Range 090%: inserted=00095:00296 wCBBS=00192:00304 µs=0000039112:0018160352 sum=17296
2023/10/08 20:45:38 Range 095%: inserted=00092:00320 wCBBS=00192:00328 µs=0000035286:0018300794 sum=17297
2023/10/08 20:45:38 Range 100%: inserted=00001:00296 wCBBS=-0001:00304 µs=0000037367:0017850962 sum=13837
```

## Checking 400.000.000 `i` hashes (75% duplicates) vs hashdb
```sh
./nntp-history-test -todo 100000000
ARGS: CPU=4/12 | jobs=4 | todo=100000000 | total=400000000 | keyalgo=11 | keylen=6 | BatchSize=1024
 useHashDB: true | IndexParallel=16
 boltOpts='&bbolt.Options{Timeout:9000000000, NoGrowSync:false, NoFreelistSync:false, PreLoadFreelist:false, FreelistType:"", ReadOnly:false, MmapFlags:0, InitialMmapSize:2147483648, PageSize:65536, NoSync:false, OpenFile:(func(string, int, fs.FileMode) (*os.File, error))(nil), Mlock:false}'
2023/10/09 02:31:15 History: new=false
  HF='history/history.dat' DB='hashdb/history.dat.hash.[0-9a-f]'
  KeyAlgo=11 KeyLen=6 NumQueueWriteChan=16
  HashDBQueues:{NumQueueIndexChan=16 NumQueueIndexChans=4 BatchSize=1024 IndexParallel=16}
...
2023/10/09 02:31:30 BoltSpeed: 38398.86 tx/s ( did=575984 in 15.0 sec ) totalTX=575984
...
2023/10/09 02:31:45 BoltSpeed: 39050.04 tx/s ( did=585750 in 15.0 sec ) totalTX=1161734
...
2023/10/09 02:33:30 BoltSpeed: 37048.84 tx/s ( did=555620 in 15.0 sec ) totalTX=5077207
...
2023/10/09 02:35:41 RUN test p=4 nntp-history added=0 dupes=2500220 cLock=5161855 addretry=0 retry=0 adddupes=0 cdupes=2337925 cretry1=0 10000000/100000000
2023/10/09 02:35:41 RUN test p=1 nntp-history added=0 dupes=2504373 cLock=5163754 addretry=0 retry=0 adddupes=0 cdupes=2331873 cretry1=0 10000000/100000000
2023/10/09 02:35:41 RUN test p=3 nntp-history added=0 dupes=2498858 cLock=5160476 addretry=0 retry=0 adddupes=0 cdupes=2340666 cretry1=0 10000000/100000000
2023/10/09 02:35:41 RUN test p=2 nntp-history added=0 dupes=2496549 cLock=5158810 addretry=0 retry=0 adddupes=0 cdupes=2344641 cretry1=0 10000000/100000000
...
2023/10/09 02:38:00 BoltSpeed: 39187.61 tx/s ( did=587800 in 15.0 sec ) totalTX=15272713
...
2023/10/09 02:40:04 RUN test p=3 nntp-history added=0 dupes=4995440 cLock=10327604 addretry=0 retry=0 adddupes=0 cdupes=4676956 cretry1=0 20000000/100000000
2023/10/09 02:40:04 RUN test p=1 nntp-history added=0 dupes=5004269 cLock=10330855 addretry=0 retry=0 adddupes=0 cdupes=4664876 cretry1=0 20000000/100000000
2023/10/09 02:40:04 RUN test p=4 nntp-history added=0 dupes=5003142 cLock=10336423 addretry=0 retry=0 adddupes=0 cdupes=4660435 cretry1=0 20000000/100000000
2023/10/09 02:40:04 RUN test p=2 nntp-history added=0 dupes=4997149 cLock=10329422 addretry=0 retry=0 adddupes=0 cdupes=4673429 cretry1=0 20000000/100000000
...
2023/10/09 02:40:15 BoltSpeed: 37070.41 tx/s ( did=556056 in 15.0 sec ) totalTX=20380275
...
2023/10/09 02:44:27 RUN test p=3 nntp-history added=0 dupes=7494001 cLock=15483389 addretry=0 retry=0 adddupes=0 cdupes=7022610 cretry1=0 30000000/100000000
2023/10/09 02:44:27 RUN test p=1 nntp-history added=0 dupes=7509512 cLock=15492433 addretry=0 retry=0 adddupes=0 cdupes=6998055 cretry1=0 30000000/100000000
2023/10/09 02:44:27 RUN test p=4 nntp-history added=0 dupes=7503162 cLock=15495167 addretry=0 retry=0 adddupes=0 cdupes=7001671 cretry1=0 30000000/100000000
2023/10/09 02:44:27 RUN test p=2 nntp-history added=0 dupes=7493325 cLock=15481820 addretry=0 retry=0 adddupes=0 cdupes=7024855 cretry1=0 30000000/100000000
...
2023/10/09 02:44:15 BoltSpeed: 38679.74 tx/s ( did=580025 in 15.0 sec ) totalTX=29503922
...
2023/10/09 02:49:03 RUN test p=1 nntp-history added=0 dupes=10009601 cLock=20563519 addretry=0 retry=0 adddupes=0 cdupes=9426880 cretry1=0 40000000/100000000
2023/10/09 02:49:03 RUN test p=3 nntp-history added=0 dupes=9993411 cLock=20556675 addretry=0 retry=0 adddupes=0 cdupes=9449914 cretry1=0 40000000/100000000
2023/10/09 02:49:03 RUN test p=4 nntp-history added=0 dupes=10003554 cLock=20565264 addretry=0 retry=0 adddupes=0 cdupes=9431182 cretry1=0 40000000/100000000
2023/10/09 02:49:03 RUN test p=2 nntp-history added=0 dupes=9993434 cLock=20553001 addretry=0 retry=0 adddupes=0 cdupes=9453565 cretry1=0 40000000/100000000
...
2023/10/09 02:49:15 BoltSpeed: 34267.44 tx/s ( did=514011 in 15.0 sec ) totalTX=40416626
...
2023/10/09 02:53:56 RUN test p=4 nntp-history added=0 dupes=12500732 cLock=25480212 addretry=0 retry=0 adddupes=0 cdupes=12019056 cretry1=0 50000000/100000000
2023/10/09 02:53:56 RUN test p=1 nntp-history added=0 dupes=12513541 cLock=25483340 addretry=0 retry=0 adddupes=0 cdupes=12003119 cretry1=0 50000000/100000000
2023/10/09 02:53:56 RUN test p=3 nntp-history added=0 dupes=12493422 cLock=25470240 addretry=0 retry=0 adddupes=0 cdupes=12036338 cretry1=0 50000000/100000000
2023/10/09 02:53:56 RUN test p=2 nntp-history added=0 dupes=12492305 cLock=25466755 addretry=0 retry=0 adddupes=0 cdupes=12040940 cretry1=0 50000000/100000000
...
2023/10/09 02:54:00 BoltSpeed: 34718.26 tx/s ( did=520775 in 15.0 sec ) totalTX=50123238
...
2023/10/09 02:58:51 RUN test p=4 nntp-history added=0 dupes=15000597 cLock=30421370 addretry=0 retry=0 adddupes=0 cdupes=14578033 cretry1=0 60000000/100000000
2023/10/09 02:58:51 RUN test p=1 nntp-history added=1 dupes=15014607 cLock=30425258 addretry=0 retry=0 adddupes=0 cdupes=14560134 cretry1=0 60000000/100000000
2023/10/09 02:58:51 RUN test p=3 nntp-history added=0 dupes=14992857 cLock=30411517 addretry=0 retry=0 adddupes=0 cdupes=14595626 cretry1=0 60000000/100000000
2023/10/09 02:58:51 RUN test p=2 nntp-history added=0 dupes=14991938 cLock=30407522 addretry=0 retry=0 adddupes=0 cdupes=14600540 cretry1=0 60000000/100000000
...
2023/10/09 02:59:00 BoltSpeed: 33578.42 tx/s ( did=503676 in 15.0 sec ) totalTX=60293960
...
2023/10/09 03:03:49 RUN test p=4 nntp-history added=2 dupes=17500874 cLock=35331502 addretry=0 retry=0 adddupes=0 cdupes=17167622 cretry1=0 70000000/100000000
2023/10/09 03:03:49 RUN test p=1 nntp-history added=1 dupes=17517141 cLock=35334683 addretry=0 retry=0 adddupes=0 cdupes=17148175 cretry1=0 70000000/100000000
2023/10/09 03:03:49 RUN test p=3 nntp-history added=0 dupes=17494729 cLock=35321535 addretry=0 retry=0 adddupes=0 cdupes=17183736 cretry1=0 70000000/100000000
2023/10/09 03:03:49 RUN test p=2 nntp-history added=0 dupes=17487253 cLock=35309511 addretry=0 retry=0 adddupes=0 cdupes=17203236 cretry1=0 70000000/100000000
...
2023/10/09 03:04:00 BoltSpeed: 33668.33 tx/s ( did=505023 in 15.0 sec ) totalTX=70375329
...
2023/10/09 03:08:47 RUN test p=3 nntp-history added=2 dupes=19993428 cLock=40180863 addretry=0 retry=0 adddupes=0 cdupes=19825707 cretry1=0 80000000/100000000
2023/10/09 03:08:47 RUN test p=4 nntp-history added=3 dupes=20004183 cLock=40196668 addretry=0 retry=0 adddupes=0 cdupes=19799146 cretry1=0 80000000/100000000
2023/10/09 03:08:47 RUN test p=1 nntp-history added=3 dupes=20018259 cLock=40198062 addretry=0 retry=0 adddupes=0 cdupes=19783676 cretry1=0 80000000/100000000
2023/10/09 03:08:47 RUN test p=2 nntp-history added=0 dupes=19984122 cLock=40166480 addretry=0 retry=0 adddupes=0 cdupes=19849398 cretry1=0 80000000/100000000
...
2023/10/09 03:09:00 BoltSpeed: 33418.85 tx/s ( did=501281 in 15.0 sec ) totalTX=80440580
...
2023/10/09 03:13:44 RUN test p=3 nntp-history added=2 dupes=22490655 cLock=45035827 addretry=0 retry=0 adddupes=0 cdupes=22473516 cretry1=0 90000000/100000000
2023/10/09 03:13:44 RUN test p=4 nntp-history added=5 dupes=22504722 cLock=45056242 addretry=0 retry=0 adddupes=0 cdupes=22439031 cretry1=0 90000000/100000000
2023/10/09 03:13:44 RUN test p=2 nntp-history added=0 dupes=22485767 cLock=45023503 addretry=0 retry=0 adddupes=0 cdupes=22490730 cretry1=0 90000000/100000000
2023/10/09 03:13:44 RUN test p=1 nntp-history added=3 dupes=22518846 cLock=45053591 addretry=0 retry=0 adddupes=0 cdupes=22427560 cretry1=0 90000000/100000000
...
2023/10/09 03:13:45 BoltSpeed: 32758.63 tx/s ( did=491480 in 15.0 sec ) totalTX=90021765
...
2023/10/09 03:18:30 BoltSpeed: 33427.87 tx/s ( did=501419 in 15.0 sec ) totalTX=99594153
...
2023/10/09 03:18:42 End test p=3 nntp-history added=2 dupes=24986842 cLock=49896315 addretry=0 retry=0 adddupes=0 cdupes=25116841 cretry1=0 sum=100000000/100000000 errors=0 locked=24986844
2023/10/09 03:18:42 End test p=2 nntp-history added=0 dupes=24983443 cLock=49883607 addretry=0 retry=0 adddupes=0 cdupes=25132950 cretry1=0 sum=100000000/100000000 errors=0 locked=24983443
2023/10/09 03:18:42 End test p=4 nntp-history added=6 dupes=25003874 cLock=49921016 addretry=0 retry=0 adddupes=0 cdupes=25075104 cretry1=0 sum=100000000/100000000 errors=0 locked=25003880
2023/10/09 03:18:42 End test p=1 nntp-history added=4 dupes=25025829 cLock=49922364 addretry=0 retry=0 adddupes=0 cdupes=25051803 cretry1=0 sum=100000000/100000000 errors=0 locked=25025833
...
2023/10/09 03:18:42 CLOSE_HISTORY: his.WriterChan <- nil
2023/10/09 03:18:42 WAIT CLOSE_HISTORY: lock1=true=1 lock2=true=1 lock3=true=16 lock4=true=16 lock5=true=256 batchQueued=false=0 batchLocked=false=0
2023/10/09 03:18:43 WAIT CLOSE_HISTORY: lock1=false=0 lock2=false=0 lock3=true=9 lock4=true=9 lock5=true=117 batchQueued=true=117 batchLocked=false=0
2023/10/09 03:18:44 CLOSE_HISTORY DONE
2023/10/09 03:18:44 key_add=0 key_app=12 total=12 fseeks=101148923 eof=0 BoltDB_decodedOffsets=99984932 addoffset=0 appoffset=12 trymultioffsets=2300913 tryoffset=97699087 searches=100000000 inserted=12
2023/10/09 03:18:44 L1LOCK=100000000 | Get: L2=15216 L3=15080 | wCBBS=~1023 conti=0 slept=5678
2023/10/09 03:18:44 done=400000000 (took 2847 seconds) (closewait 2 seconds)
2023/10/09 03:18:44 CrunchBatchLogs: did=12 dat=12
2023/10/09 03:18:44 CrunchLogs: inserted=00001:00001 wCBBS=01016:01024 µs=0000010894:0000012400
2023/10/09 03:18:44 Range 005%: inserted=00001:00001 wCBBS=01024:01024 µs=0000011029:0000011029 sum=1
2023/10/09 03:18:44 Range 010%: inserted=00001:00001 wCBBS=01024:01024 µs=0000012400:0000012400 sum=1
2023/10/09 03:18:44 Range 020%: inserted=00001:00001 wCBBS=01024:01024 µs=0000010916:0000010916 sum=1
2023/10/09 03:18:44 Range 025%: inserted=00001:00001 wCBBS=01024:01024 µs=0000011607:0000011607 sum=1
2023/10/09 03:18:44 Range 035%: inserted=00001:00001 wCBBS=01024:01024 µs=0000010895:0000010895 sum=1
2023/10/09 03:18:44 Range 045%: inserted=00001:00001 wCBBS=01024:01024 µs=0000011353:0000011353 sum=1
2023/10/09 03:18:44 Range 050%: inserted=00001:00001 wCBBS=01016:01016 µs=0000011087:0000011087 sum=1
2023/10/09 03:18:44 Range 060%: inserted=00001:00001 wCBBS=01024:01024 µs=0000010978:0000010978 sum=1
2023/10/09 03:18:44 Range 070%: inserted=00001:00001 wCBBS=01024:01024 µs=0000010894:0000010894 sum=1
2023/10/09 03:18:44 Range 075%: inserted=00001:00001 wCBBS=01024:01024 µs=0000012136:0000012136 sum=1
2023/10/09 03:18:44 Range 085%: inserted=00001:00001 wCBBS=01024:01024 µs=0000011191:0000011191 sum=1
2023/10/09 03:18:44 Range 095%: inserted=00001:00001 wCBBS=01024:01024 µs=0000011352:0000011352 sum=1
...
...
check should not add / app anything. problem exists in caching algo.
cache evicts before we've fully written the data of key:offset(s) tuple to disk.
```
