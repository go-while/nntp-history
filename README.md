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
2023/10/08 14:31:52 End test p=4 nntp-history added=250339 dupes=0 cachehits=423214 addretry=0 retry=0 adddupes=0 cachedupes=326447 cacheretry1=0 sum=1000000/1000000 errors=0 locked=250339
2023/10/08 14:31:52 End test p=3 nntp-history added=249431 dupes=0 cachehits=423462 addretry=0 retry=0 adddupes=0 cachedupes=327107 cacheretry1=0 sum=1000000/1000000 errors=0 locked=249431
2023/10/08 14:31:52 End test p=1 nntp-history added=251125 dupes=0 cachehits=423355 addretry=0 retry=0 adddupes=0 cachedupes=325520 cacheretry1=0 sum=1000000/1000000 errors=0 locked=251125
2023/10/08 14:31:52 End test p=2 nntp-history added=249105 dupes=0 cachehits=423066 addretry=0 retry=0 adddupes=0 cachedupes=327829 cacheretry1=0 sum=1000000/1000000 errors=0 locked=249105
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
2023/10/08 14:32:56 End test p=2 nntp-history added=0 dupes=249482 cachehits=547547 addretry=0 retry=0 adddupes=0 cachedupes=202971 cacheretry1=0 sum=1000000/1000000 errors=0 locked=249482
2023/10/08 14:32:56 End test p=3 nntp-history added=0 dupes=250551 cachehits=548727 addretry=0 retry=0 adddupes=0 cachedupes=200722 cacheretry1=0 sum=1000000/1000000 errors=0 locked=250551
2023/10/08 14:32:56 End test p=4 nntp-history added=0 dupes=249623 cachehits=547928 addretry=0 retry=0 adddupes=0 cachedupes=202449 cacheretry1=0 sum=1000000/1000000 errors=0 locked=249623
2023/10/08 14:32:56 End test p=1 nntp-history added=0 dupes=250344 cachehits=548689 addretry=0 retry=0 adddupes=0 cachedupes=200967 cacheretry1=0 sum=1000000/1000000 errors=0 locked=250344
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
2023/10/08 18:06:00 RUN test p=4 nntp-history added=2492416 dupes=0 cachehits=4021442 addretry=0 retry=0 adddupes=0 cachedupes=3486142 cacheretry1=0 10000000/100000000
2023/10/08 18:06:00 RUN test p=3 nntp-history added=2502228 dupes=0 cachehits=4025283 addretry=0 retry=0 adddupes=0 cachedupes=3472489 cacheretry1=0 10000000/100000000
2023/10/08 18:06:00 RUN test p=1 nntp-history added=2501394 dupes=0 cachehits=4021422 addretry=0 retry=0 adddupes=0 cachedupes=3477184 cacheretry1=0 10000000/100000000
2023/10/08 18:06:00 RUN test p=2 nntp-history added=2503962 dupes=0 cachehits=4020087 addretry=0 retry=0 adddupes=0 cachedupes=3475951 cacheretry1=0 10000000/100000000
...
2023/10/08 18:08:29 BoltSpeed: 28428.51 tx/s ( did=426429 in 15.0 sec ) totalTX=14539993
2023/10/08 18:08:44 BoltSpeed: 28750.23 tx/s ( did=431270 in 15.0 sec ) totalTX=14971263
...
2023/10/08 18:12:00 RUN test p=3 nntp-history added=5006126 dupes=0 cachehits=7599269 addretry=0 retry=0 adddupes=0 cachedupes=7394605 cacheretry1=0 20000000/100000000
2023/10/08 18:12:00 RUN test p=4 nntp-history added=4988108 dupes=0 cachehits=7590973 addretry=0 retry=0 adddupes=0 cachedupes=7420919 cacheretry1=0 20000000/100000000
2023/10/08 18:12:00 RUN test p=1 nntp-history added=5007827 dupes=0 cachehits=7592780 addretry=0 retry=0 adddupes=0 cachedupes=7399393 cacheretry1=0 20000000/100000000
2023/10/08 18:12:00 RUN test p=2 nntp-history added=4997939 dupes=0 cachehits=7589744 addretry=0 retry=0 adddupes=0 cachedupes=7412317 cacheretry1=0 20000000/100000000
...
2023/10/08 18:19:29 BoltSpeed: 18166.40 tx/s ( did=272511 in 15.0 sec ) totalTX=27857110
2023/10/08 18:19:44 BoltSpeed: 13151.54 tx/s ( did=197255 in 15.0 sec ) totalTX=28054365
...
2023/10/08 18:22:12 RUN test p=4 nntp-history added=7475027 dupes=0 cachehits=10624599 addretry=0 retry=0 adddupes=0 cachedupes=11900374 cacheretry1=0 30000000/100000000
2023/10/08 18:22:12 RUN test p=2 nntp-history added=7488813 dupes=0 cachehits=10623678 addretry=0 retry=0 adddupes=0 cachedupes=11887509 cacheretry1=0 30000000/100000000
2023/10/08 18:22:12 RUN test p=3 nntp-history added=7513602 dupes=0 cachehits=10636308 addretry=0 retry=0 adddupes=0 cachedupes=11850090 cacheretry1=0 30000000/100000000
2023/10/08 18:22:12 RUN test p=1 nntp-history added=7522558 dupes=0 cachehits=10629385 addretry=0 retry=0 adddupes=0 cachedupes=11848057 cacheretry1=0 30000000/100000000
...
2023/10/08 18:22:44 BoltSpeed: 14160.60 tx/s ( did=212199 in 15.0 sec ) totalTX=30427951
2023/10/08 18:22:59 BoltSpeed: 14050.44 tx/s ( did=210745 in 15.0 sec ) totalTX=30638696
...
2023/10/08 18:29:29 BoltSpeed: 14335.57 tx/s ( did=215021 in 15.0 sec ) totalTX=35436346
2023/10/08 18:29:44 BoltSpeed: 16308.28 tx/s ( did=244628 in 15.0 sec ) totalTX=35680974
...
2023/10/08 18:36:16 RUN test p=1 nntp-history added=9997081 dupes=0 cachehits=13404293 addretry=0 retry=0 adddupes=0 cachedupes=16598626 cacheretry1=0 40000000/100000000
2023/10/08 18:36:16 RUN test p=2 nntp-history added=9976454 dupes=0 cachehits=13397623 addretry=0 retry=0 adddupes=0 cachedupes=16625923 cacheretry1=0 40000000/100000000
2023/10/08 18:36:16 RUN test p=3 nntp-history added=10041923 dupes=0 cachehits=13412518 addretry=0 retry=0 adddupes=0 cachedupes=16545559 cacheretry1=0 40000000/100000000
2023/10/08 18:36:16 RUN test p=4 nntp-history added=9984542 dupes=0 cachehits=13399590 addretry=0 retry=0 adddupes=0 cachedupes=16615868 cacheretry1=0 40000000/100000000
...
2023/10/08 18:36:29 BoltSpeed: 15694.29 tx/s ( did=235991 in 15.0 sec ) totalTX=40183347
2023/10/08 18:36:44 BoltSpeed: 10035.14 tx/s ( did=150161 in 15.0 sec ) totalTX=40333508
...
2023/10/08 18:44:29 BoltSpeed: 11611.33 tx/s ( did=174149 in 15.0 sec ) totalTX=45049808
2023/10/08 18:44:59 BoltSpeed: 11397.50 tx/s ( did=170961 in 15.0 sec ) totalTX=45311296
...
2023/10/08 18:54:18 RUN test p=2 nntp-history added=12484775 dupes=0 cachehits=15895032 addretry=0 retry=0 adddupes=0 cachedupes=21620193 cacheretry1=0 50000000/100000000
2023/10/08 18:54:18 RUN test p=4 nntp-history added=12472563 dupes=0 cachehits=15898842 addretry=0 retry=0 adddupes=0 cachedupes=21628595 cacheretry1=0 50000000/100000000
2023/10/08 18:54:18 RUN test p=3 nntp-history added=12527524 dupes=0 cachehits=15913534 addretry=0 retry=0 adddupes=0 cachedupes=21558942 cacheretry1=0 50000000/100000000
2023/10/08 18:54:18 RUN test p=1 nntp-history added=12515138 dupes=0 cachehits=15902903 addretry=0 retry=0 adddupes=0 cachedupes=21581959 cacheretry1=0 50000000/100000000
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
```

## Checking 400.000.000 `i` hashes (75% duplicates) vs hashdb
```sh
...
...
...
...
```
