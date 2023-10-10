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
- Test with 100M inserts (`i` hashes) shows 1.2% stored multioffsets with `KeyLen` = 6.

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

## Inserting 4.000.000 `i` hashes (75% duplicates) to history and hashdb
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
# history.DBG_BS_LOG = true // debugs BatchLOG for every batch insert!
# history.DBG_GOB_TEST = true // costly check: test decodes gob encoded data
#
./nntp-history-test -todo 100000000
...
header lost in terminal
...

2023/10/10 00:23:47 L3: [fex=1081313/set:96932100] [del:96834016/bat:96912971] [g/s:147/121] cached:94044 (~5877/char)
2023/10/10 00:23:47 BoltSpeed: 3679.76 tx/s ( did=55208 in 15.0 sec ) totalTX=96942173
Alloc: 747 MiB, TotalAlloc: 10637437 MiB, Sys: 2095 MiB, NumGC: 25863
2023/10/10 00:24:02 L1: [fex=0/set:98109093] [del:96884568/bat:96999043] [g/s:147/118] cached:137139 (~8571/char)
2023/10/10 00:24:02 L2: [fex=1091725/set:97021714] [del:97977611/bat:98092315] [g/s:79/63] cached:135828 (~8489/char)
2023/10/10 00:24:02 L3: [fex=1083347/set:97021735] [del:96887005/bat:96999043] [g/s:147/121] cached:130689 (~8168/char)
2023/10/10 00:24:02 BoltSpeed: 5972.19 tx/s ( did=89649 in 15.0 sec ) totalTX=97031822
2023/10/10 00:24:17 L1: [fex=0/set:98190650] [del:96979510/bat:97084136] [g/s:147/118] cached:121907 (~7619/char)
2023/10/10 00:24:17 L2: [fex=1093592/set:97101414] [del:98072123/bat:98179409] [g/s:79/63] cached:122883 (~7680/char)
2023/10/10 00:24:17 L3: [fex=1085193/set:97101414] [del:96976995/bat:97084087] [g/s:147/121] cached:120377 (~7523/char)
2023/10/10 00:24:17 BoltSpeed: 5291.83 tx/s ( did=79694 in 15.1 sec ) totalTX=97111516
2023/10/10 00:24:32 L1: [fex=0/set:98253681] [del:97068682/bat:97134491] [g/s:147/118] cached:94447 (~5902/char)
Alloc: 608 MiB, TotalAlloc: 10658340 MiB, Sys: 2095 MiB, NumGC: 25908
2023/10/10 00:24:32 L2: [fex=1094923/set:97163127] [del:98153276/bat:98230867] [g/s:79/63] cached:104774 (~6548/char)
2023/10/10 00:24:32 L3: [fex=1086510/set:97163128] [del:97060082/bat:97134491] [g/s:147/121] cached:99004 (~6187/char)
2023/10/10 00:24:32 BoltSpeed: 4135.85 tx/s ( did=61730 in 14.9 sec ) totalTX=97173246
2023/10/10 00:24:47 L1: [fex=0/set:98305761] [del:97123113/bat:97175033] [g/s:147/118] cached:90914 (~5682/char)
2023/10/10 00:24:47 L2: [fex=1096122/set:97214023] [del:98220154/bat:98272325] [g/s:79/63] cached:89991 (~5624/char)
2023/10/10 00:24:47 L3: [fex=1087693/set:97214023] [del:97120201/bat:97175033] [g/s:147/121] cached:89781 (~5611/char)
2023/10/10 00:24:47 BoltSpeed: 3394.18 tx/s ( did=50912 in 15.0 sec ) totalTX=97224158
2023/10/10 00:25:02 L1: [fex=0/set:98355646] [del:97166124/bat:97244834] [g/s:147/118] cached:96702 (~6043/char)
2023/10/10 00:25:02 L2: [fex=1097211/set:97262833] [del:98261196/bat:98343722] [g/s:79/63] cached:98848 (~6178/char)
2023/10/10 00:25:02 L3: [fex=1088774/set:97262833] [del:97162677/bat:97244834] [g/s:147/121] cached:96111 (~6006/char)
2023/10/10 00:25:02 BoltSpeed: 3254.08 tx/s ( did=48823 in 15.0 sec ) totalTX=97272981
Alloc: 960 MiB, TotalAlloc: 10674525 MiB, Sys: 2095 MiB, NumGC: 25939
2023/10/10 00:25:17 L1: [fex=0/set:98436104] [del:97227779/bat:97320432] [g/s:147/118] cached:113746 (~7109/char)
2023/10/10 00:25:17 L2: [fex=1098994/set:97341521] [del:98326738/bat:98421024] [g/s:79/63] cached:113777 (~7111/char)
2023/10/10 00:25:17 L3: [fex=1090537/set:97341521] [del:97224866/bat:97320432] [g/s:147/121] cached:112610 (~7038/char)
2023/10/10 00:25:17 BoltSpeed: 5248.21 tx/s ( did=78703 in 15.0 sec ) totalTX=97351684
Alloc: 1010 MiB, TotalAlloc: 10692831 MiB, Sys: 2095 MiB, NumGC: 25980
2023/10/10 00:25:32 L1: [fex=0/set:98488634] [del:97301117/bat:97361916] [g/s:147/118] cached:91771 (~5735/char)
2023/10/10 00:25:32 L2: [fex=1100175/set:97392886] [del:98398427/bat:98463440] [g/s:79/63] cached:94634 (~5914/char)
2023/10/10 00:25:32 L3: [fex=1091698/set:97392886] [del:97296909/bat:97361916] [g/s:147/121] cached:91929 (~5745/char)
2023/10/10 00:25:32 BoltSpeed: 3423.47 tx/s ( did=51378 in 15.0 sec ) totalTX=97403062
2023/10/10 00:25:47 L1: [fex=0/set:98562246] [del:97350347/bat:97447382] [g/s:147/118] cached:114539 (~7158/char)
2023/10/10 00:25:47 L2: [fex=1101803/set:97464886] [del:98451719/bat:98550873] [g/s:79/63] cached:114970 (~7185/char)
2023/10/10 00:25:47 L3: [fex=1093308/set:97464886] [del:97347681/bat:97447382] [g/s:147/121] cached:113154 (~7072/char)
2023/10/10 00:25:47 BoltSpeed: 4803.32 tx/s ( did=72013 in 15.0 sec ) totalTX=97475075
Alloc: 1037 MiB, TotalAlloc: 10717009 MiB, Sys: 2095 MiB, NumGC: 26029
2023/10/10 00:26:02 L1: [fex=0/set:98640448] [del:97429437/bat:97518071] [g/s:147/118] cached:111940 (~6996/char)
2023/10/10 00:26:02 L2: [fex=1103531/set:97541373] [del:98528516/bat:98623184] [g/s:79/63] cached:116388 (~7274/char)
2023/10/10 00:26:02 L3: [fex=1095023/set:97541374] [del:97425145/bat:97518071] [g/s:147/121] cached:112178 (~7011/char)
2023/10/10 00:26:02 BoltSpeed: 5100.32 tx/s ( did=76505 in 15.0 sec ) totalTX=97551580
2023/10/10 00:26:17 L1: [fex=0/set:98694631] [del:97511052/bat:97558966] [g/s:147/118] cached:83300 (~5206/char)
2023/10/10 00:26:17 L2: [fex=1104758/set:97594348] [del:98612363/bat:98664989] [g/s:79/63] cached:86743 (~5421/char)
2023/10/10 00:26:17 L3: [fex=1096231/set:97594348] [del:97504877/bat:97558966] [g/s:147/121] cached:85420 (~5338/char)
2023/10/10 00:26:17 BoltSpeed: 3532.46 tx/s ( did=52989 in 15.0 sec ) totalTX=97604569
Alloc: 1066 MiB, TotalAlloc: 10730008 MiB, Sys: 2095 MiB, NumGC: 26053
2023/10/10 00:26:32 L1: [fex=0/set:98749292] [del:97549625/bat:97604193] [g/s:147/118] cached:98201 (~6137/char)
2023/10/10 00:26:32 L2: [fex=1105959/set:97647824] [del:98653878/bat:98711341] [g/s:79/63] cached:99905 (~6244/char)
2023/10/10 00:26:32 L3: [fex=1097417/set:97647824] [del:97546160/bat:97604292] [g/s:147/121] cached:97612 (~6100/char)
2023/10/10 00:26:32 BoltSpeed: 3549.19 tx/s ( did=53491 in 15.1 sec ) totalTX=97658060
2023/10/10 00:26:47 L1: [fex=0/set:98801812] [del:97589650/bat:97685239] [g/s:147/118] cached:109517 (~6844/char)
2023/10/10 00:26:47 L2: [fex=1107145/set:97699164] [del:98695919/bat:98794094] [g/s:79/63] cached:110390 (~6899/char)
2023/10/10 00:26:47 L3: [fex=1098592/set:97699164] [del:97587377/bat:97685239] [g/s:147/121] cached:107734 (~6733/char)
2023/10/10 00:26:47 BoltSpeed: 3440.17 tx/s ( did=51355 in 14.9 sec ) totalTX=97709415
Alloc: 460 MiB, TotalAlloc: 10753034 MiB, Sys: 2095 MiB, NumGC: 26107
2023/10/10 00:27:02 L1: [fex=0/set:98878796] [del:97667450/bat:97754614] [g/s:147/118] cached:106926 (~6682/char)
2023/10/10 00:27:02 L2: [fex=1108932/set:97774372] [del:98776476/bat:98865119] [g/s:79/63] cached:106828 (~6676/char)
2023/10/10 00:27:02 L3: [fex=1100366/set:97774372] [del:97663291/bat:97754614] [g/s:147/121] cached:107024 (~6689/char)
2023/10/10 00:27:02 BoltSpeed: 5014.39 tx/s ( did=75220 in 15.0 sec ) totalTX=97784635
2023/10/10 00:27:17 L1: [fex=0/set:98935299] [del:97742817/bat:97795588] [g/s:147/118] cached:86814 (~5425/char)
2023/10/10 00:27:17 L2: [fex=1110187/set:97829627] [del:98844565/bat:98907044] [g/s:79/63] cached:95249 (~5953/char)
2023/10/10 00:27:17 L3: [fex=1101612/set:97829627] [del:97732582/bat:97795588] [g/s:147/121] cached:92989 (~5811/char)
2023/10/10 00:27:17 BoltSpeed: 3684.98 tx/s ( did=55272 in 15.0 sec ) totalTX=97839907
2023/10/10 00:27:32 L1: [fex=0/set:98993866] [del:97785109/bat:97838138] [g/s:147/118] cached:101757 (~6359/char)
2023/10/10 00:27:32 L2: [fex=1111532/set:97886862] [del:98895547/bat:98950591] [g/s:79/63] cached:102847 (~6427/char)
2023/10/10 00:27:32 L3: [fex=1102943/set:97886862] [del:97781589/bat:97838138] [g/s:147/121] cached:101216 (~6326/char)
2023/10/10 00:27:32 BoltSpeed: 3816.11 tx/s ( did=57250 in 15.0 sec ) totalTX=97897157
Alloc: 833 MiB, TotalAlloc: 10765939 MiB, Sys: 2095 MiB, NumGC: 26133
2023/10/10 00:27:47 L1: [fex=0/set:99046129] [del:97826327/bat:97888154] [g/s:147/118] cached:111712 (~6982/char)
2023/10/10 00:27:47 L2: [fex=1112634/set:97938036] [del:98939345/bat:99001657] [g/s:79/63] cached:111325 (~6957/char)
2023/10/10 00:27:47 L3: [fex=1104032/set:97938036] [del:97825878/bat:97888154] [g/s:147/121] cached:108099 (~6756/char)
2023/10/10 00:27:47 BoltSpeed: 3412.94 tx/s ( did=51188 in 15.0 sec ) totalTX=97948345
Alloc: 778 MiB, TotalAlloc: 10786393 MiB, Sys: 2095 MiB, NumGC: 26174
2023/10/10 00:28:02 L1: [fex=0/set:99102978] [del:97869567/bat:97977460] [g/s:147/118] cached:124082 (~7755/char)
2023/10/10 00:28:02 L2: [fex=1113891/set:97993645] [del:98982978/bat:99092925] [g/s:79/63] cached:124558 (~7784/char)
2023/10/10 00:28:02 L3: [fex=1105273/set:97993645] [del:97867083/bat:97977460] [g/s:147/121] cached:122504 (~7656/char)
2023/10/10 00:28:02 BoltSpeed: 3708.51 tx/s ( did=55626 in 15.0 sec ) totalTX=98003971
2023/10/10 00:28:17 L1: [fex=0/set:99171883] [del:97961641/bat:98044329] [g/s:147/118] cached:99334 (~6208/char)
2023/10/10 00:28:17 L2: [fex=1115480/set:98060971] [del:99070063/bat:99161355] [g/s:79/63] cached:106388 (~6649/char)
2023/10/10 00:28:17 L3: [fex=1106850/set:98060971] [del:97953628/bat:98044329] [g/s:147/121] cached:103284 (~6455/char)
2023/10/10 00:28:17 BoltSpeed: 4489.13 tx/s ( did=67341 in 15.0 sec ) totalTX=98071312
2023/10/10 00:28:32 L1: [fex=0/set:99226470] [del:98031031/bat:98083990] [g/s:147/118] cached:83393 (~5212/char)
2023/10/10 00:28:32 L2: [fex=1116634/set:98114420] [del:99149472/bat:99201894] [g/s:79/63] cached:81582 (~5098/char)
2023/10/10 00:28:32 L3: [fex=1107987/set:98114420] [del:98030182/bat:98083990] [g/s:147/121] cached:80178 (~5011/char)
2023/10/10 00:28:32 BoltSpeed: 3563.95 tx/s ( did=53464 in 15.0 sec ) totalTX=98124776
Alloc: 916 MiB, TotalAlloc: 10803109 MiB, Sys: 2095 MiB, NumGC: 26212
2023/10/10 00:28:47 L1: [fex=0/set:99282035] [del:98074212/bat:98149830] [g/s:147/118] cached:94567 (~5910/char)
2023/10/10 00:28:47 L2: [fex=1117849/set:98168775] [del:99191077/bat:99269154] [g/s:79/63] cached:95547 (~5971/char)
2023/10/10 00:28:47 L3: [fex=1109193/set:98168775] [del:98073307/bat:98149830] [g/s:147/121] cached:91403 (~5712/char)
2023/10/10 00:28:47 BoltSpeed: 3624.31 tx/s ( did=54368 in 15.0 sec ) totalTX=98179144
Alloc: 724 MiB, TotalAlloc: 10824899 MiB, Sys: 2095 MiB, NumGC: 26261
2023/10/10 00:29:02 L1: [fex=0/set:99360818] [del:98128900/bat:98226889] [g/s:147/118] cached:116927 (~7307/char)
2023/10/10 00:29:02 L2: [fex=1119602/set:98245823] [del:99245696/bat:99347848] [g/s:79/63] cached:119729 (~7483/char)
2023/10/10 00:29:02 L3: [fex=1110926/set:98245823] [del:98123309/bat:98226690] [g/s:147/121] cached:118450 (~7403/char)
2023/10/10 00:29:02 BoltSpeed: 5136.33 tx/s ( did=77063 in 15.0 sec ) totalTX=98256207
2023/10/10 00:29:17 L1: [fex=0/set:99416788] [del:98207567/bat:98270942] [g/s:147/118] cached:92999 (~5812/char)
2023/10/10 00:29:17 L2: [fex=1120847/set:98300563] [del:99322545/bat:99393041] [g/s:79/63] cached:98865 (~6179/char)
2023/10/10 00:29:17 L3: [fex=1112156/set:98300564] [del:98200294/bat:98270942] [g/s:147/121] cached:96202 (~6012/char)
2023/10/10 00:29:17 BoltSpeed: 3651.91 tx/s ( did=54754 in 15.0 sec ) totalTX=98310961
Alloc: 1001 MiB, TotalAlloc: 10838160 MiB, Sys: 2095 MiB, NumGC: 26289
2023/10/10 00:29:32 L1: [fex=0/set:99475761] [del:98260909/bat:98313551] [g/s:147/118] cached:97307 (~6081/char)
2023/10/10 00:29:32 L2: [fex=1122182/set:98358212] [del:99381822/bat:99436632] [g/s:79/63] cached:98572 (~6160/char)
2023/10/10 00:29:32 L3: [fex=1113477/set:98358212] [del:98257925/bat:98313551] [g/s:147/121] cached:96219 (~6013/char)
2023/10/10 00:29:32 BoltSpeed: 3842.62 tx/s ( did=57663 in 15.0 sec ) totalTX=98368624
2023/10/10 00:29:47 L1: [fex=0/set:99521132] [del:98302626/bat:98386221] [g/s:147/118] cached:99953 (~6247/char)
2023/10/10 00:29:47 L2: [fex=1123197/set:98402703] [del:99423968/bat:99511305] [g/s:79/63] cached:101932 (~6370/char)
2023/10/10 00:29:47 L3: [fex=1114484/set:98402703] [del:98299195/bat:98386567] [g/s:147/121] cached:99435 (~6214/char)
2023/10/10 00:29:47 BoltSpeed: 2947.37 tx/s ( did=44502 in 15.1 sec ) totalTX=98413126
Alloc: 999 MiB, TotalAlloc: 10859874 MiB, Sys: 2095 MiB, NumGC: 26337
2023/10/10 00:30:02 L1: [fex=0/set:99592491] [del:98370893/bat:98458793] [g/s:147/118] cached:101471 (~6341/char)
2023/10/10 00:30:02 L2: [fex=1124784/set:98472360] [del:99486352/bat:99585178] [g/s:79/63] cached:110792 (~6924/char)
2023/10/10 00:30:02 L3: [fex=1116058/set:98472361] [del:98364697/bat:98458793] [g/s:147/121] cached:103593 (~6474/char)
2023/10/10 00:30:02 BoltSpeed: 4677.81 tx/s ( did=69676 in 14.9 sec ) totalTX=98482802
2023/10/10 00:30:17 L1: [fex=0/set:99682082] [del:98439374/bat:98543312] [g/s:147/118] cached:120531 (~7533/char)
2023/10/10 00:30:17 L2: [fex=1126854/set:98559903] [del:99561787/bat:99671659] [g/s:79/63] cached:124970 (~7810/char)
2023/10/10 00:30:17 L3: [fex=1118105/set:98559904] [del:98434359/bat:98543312] [g/s:147/121] cached:121471 (~7591/char)
2023/10/10 00:30:17 BoltSpeed: 5836.99 tx/s ( did=87557 in 15.0 sec ) totalTX=98570359
Alloc: 567 MiB, TotalAlloc: 10883890 MiB, Sys: 2095 MiB, NumGC: 26394
2023/10/10 00:30:32 L1: [fex=0/set:99756705] [del:98521871/bat:98609521] [g/s:147/118] cached:110938 (~6933/char)
2023/10/10 00:30:32 L2: [fex=1128594/set:98632805] [del:99649788/bat:99739450] [g/s:79/63] cached:111611 (~6975/char)
2023/10/10 00:30:32 L3: [fex=1119826/set:98632805] [del:98524762/bat:98609521] [g/s:147/121] cached:103971 (~6498/char)
2023/10/10 00:30:32 BoltSpeed: 4861.08 tx/s ( did=72918 in 15.0 sec ) totalTX=98643277
2023/10/10 00:30:47 L1: [fex=0/set:99812614] [del:98601024/bat:98649889] [g/s:147/118] cached:86519 (~5407/char)
2023/10/10 00:30:47 L2: [fex=1129781/set:98687539] [del:99729833/bat:99780782] [g/s:79/63] cached:87487 (~5467/char)
2023/10/10 00:30:47 L3: [fex=1120998/set:98687539] [del:98597495/bat:98649889] [g/s:147/121] cached:85971 (~5373/char)
2023/10/10 00:30:47 BoltSpeed: 3649.96 tx/s ( did=54749 in 15.0 sec ) totalTX=98698026
2023/10/10 00:31:02 L1: [fex=0/set:99865706] [del:98640885/bat:98698906] [g/s:147/118] cached:98577 (~6161/char)
2023/10/10 00:31:02 L2: [fex=1130958/set:98739462] [del:99767199/bat:99830846] [g/s:79/63] cached:103221 (~6451/char)
2023/10/10 00:31:02 L3: [fex=1122166/set:98739462] [del:98634682/bat:98698906] [g/s:147/121] cached:100702 (~6293/char)
2023/10/10 00:31:02 BoltSpeed: 3460.51 tx/s ( did=51934 in 15.0 sec ) totalTX=98749960
Alloc: 1148 MiB, TotalAlloc: 10897406 MiB, Sys: 2095 MiB, NumGC: 26421
2023/10/10 00:31:17 L1: [fex=0/set:99920572] [del:98682537/bat:98781255] [g/s:147/118] cached:110562 (~6910/char)
2023/10/10 00:31:17 L2: [fex=1132204/set:98793096] [del:99812929/bat:99915095] [g/s:79/63] cached:112371 (~7023/char)
2023/10/10 00:31:17 L3: [fex=1123397/set:98793096] [del:98678564/bat:98781255] [g/s:147/121] cached:110455 (~6903/char)
2023/10/10 00:31:17 BoltSpeed: 3578.71 tx/s ( did=53651 in 15.0 sec ) totalTX=98803611
Alloc: 1001 MiB, TotalAlloc: 10916770 MiB, Sys: 2095 MiB, NumGC: 26459
2023/10/10 00:31:32 L1: [fex=0/set:99986917] [del:98760503/bat:98828064] [g/s:147/118] cached:97465 (~6091/char)
2023/10/10 00:31:32 L2: [fex=1133703/set:98857964] [del:99894577/bat:99962986] [g/s:79/63] cached:97090 (~6068/char)
2023/10/10 00:31:32 L3: [fex=1124873/set:98857983] [del:98760327/bat:98828064] [g/s:147/121] cached:93578 (~5848/char)
2023/10/10 00:31:32 BoltSpeed: 4287.69 tx/s ( did=64906 in 15.1 sec ) totalTX=98868517
2023/10/10 00:31:47 L1: [fex=0/set:100051072] [del:98813572/bat:98908503] [g/s:147/118] cached:107127 (~6695/char)
2023/10/10 00:31:47 L2: [fex=1135147/set:98920696] [del:99944706/bat:100045247] [g/s:79/63] cached:111137 (~6946/char)
2023/10/10 00:31:47 L3: [fex=1126295/set:98920696] [del:98809145/bat:98908503] [g/s:147/121] cached:107473 (~6717/char)
2023/10/10 00:31:47 BoltSpeed: 4220.27 tx/s ( did=62725 in 14.9 sec ) totalTX=98931242
Alloc: 856 MiB, TotalAlloc: 10941578 MiB, Sys: 2095 MiB, NumGC: 26513
2023/10/10 00:32:02 L1: [fex=0/set:100133548] [del:98887001/bat:98986269] [g/s:147/118] cached:114365 (~7147/char)
2023/10/10 00:32:02 L2: [fex=1136974/set:99001363] [del:100018366/bat:100124781] [g/s:79/63] cached:119971 (~7498/char)
2023/10/10 00:32:02 L3: [fex=1128104/set:99001364] [del:98880798/bat:98986269] [g/s:147/121] cached:116488 (~7280/char)
2023/10/10 00:32:02 BoltSpeed: 5379.14 tx/s ( did=80685 in 15.0 sec ) totalTX=99011927
2023/10/10 00:32:17 L1: [fex=0/set:100190523] [del:98966551/bat:99027047] [g/s:147/118] cached:90515 (~5657/char)
2023/10/10 00:32:17 L2: [fex=1138264/set:99057062] [del:100107179/bat:100166502] [g/s:79/63] cached:88147 (~5509/char)
2023/10/10 00:32:17 L3: [fex=1129378/set:99057062] [del:98967016/bat:99027047] [g/s:147/121] cached:85967 (~5372/char)
2023/10/10 00:32:17 BoltSpeed: 3714.15 tx/s ( did=55712 in 15.0 sec ) totalTX=99067639
2023/10/10 00:32:32 L1: [fex=0/set:100248681] [del:99017704/bat:99068798] [g/s:147/118] cached:96184 (~6011/char)
2023/10/10 00:32:32 L2: [fex=1139612/set:99113884] [del:100154846/bat:100209251] [g/s:79/63] cached:98650 (~6165/char)
2023/10/10 00:32:32 L3: [fex=1130711/set:99113884] [del:99013968/bat:99068798] [g/s:147/121] cached:95834 (~5989/char)
2023/10/10 00:32:32 BoltSpeed: 3788.95 tx/s ( did=56835 in 15.0 sec ) totalTX=99124474
Alloc: 937 MiB, TotalAlloc: 10954372 MiB, Sys: 2095 MiB, NumGC: 26539
2023/10/10 00:32:47 L1: [fex=0/set:100297508] [del:99057232/bat:99149605] [g/s:147/118] cached:104362 (~6522/char)
2023/10/10 00:32:47 L2: [fex=1140749/set:99161590] [del:100193746/bat:100291937] [g/s:79/63] cached:108593 (~6787/char)
2023/10/10 00:32:47 L3: [fex=1131832/set:99161590] [del:99052016/bat:99149605] [g/s:147/121] cached:105492 (~6593/char)
2023/10/10 00:32:47 BoltSpeed: 3181.53 tx/s ( did=47722 in 15.0 sec ) totalTX=99172196
Alloc: 556 MiB, TotalAlloc: 10976550 MiB, Sys: 2095 MiB, NumGC: 26588
2023/10/10 00:33:02 L1: [fex=0/set:100377303] [del:99130918/bat:99215059] [g/s:147/118] cached:108741 (~6796/char)
2023/10/10 00:33:02 L2: [fex=1142504/set:99239655] [del:100270756/bat:100358882] [g/s:79/63] cached:111403 (~6962/char)
2023/10/10 00:33:02 L3: [fex=1133560/set:99239655] [del:99126096/bat:99215059] [g/s:147/121] cached:109474 (~6842/char)
2023/10/10 00:33:02 BoltSpeed: 5204.86 tx/s ( did=78078 in 15.0 sec ) totalTX=99250274
2023/10/10 00:33:17 L1: [fex=0/set:100431816] [del:99207095/bat:99269285] [g/s:147/118] cached:85906 (~5369/char)
2023/10/10 00:33:17 L2: [fex=1143686/set:99293006] [del:100350445/bat:100414356] [g/s:79/63] cached:86247 (~5390/char)
2023/10/10 00:33:17 L3: [fex=1134728/set:99293006] [del:99204435/bat:99269285] [g/s:147/121] cached:84484 (~5280/char)
2023/10/10 00:33:17 BoltSpeed: 3554.96 tx/s ( did=53365 in 15.0 sec ) totalTX=99303639
2023/10/10 00:33:32 L1: [fex=0/set:100506238] [del:99248766/bat:99350596] [g/s:147/118] cached:116963 (~7310/char)
2023/10/10 00:33:32 L2: [fex=1145399/set:99365729] [del:100393806/bat:100497545] [g/s:79/63] cached:117322 (~7332/char)
2023/10/10 00:33:32 L3: [fex=1136420/set:99365729] [del:99245662/bat:99350596] [g/s:147/121] cached:115979 (~7248/char)
2023/10/10 00:33:32 BoltSpeed: 4847.38 tx/s ( did=72738 in 15.0 sec ) totalTX=99376377
Alloc: 930 MiB, TotalAlloc: 10996886 MiB, Sys: 2095 MiB, NumGC: 26629
2023/10/10 00:33:47 L1: [fex=0/set:100584510] [del:99327213/bat:99424603] [g/s:147/118] cached:115089 (~7193/char)
2023/10/10 00:33:47 L2: [fex=1147114/set:99442300] [del:100469167/bat:100573238] [g/s:79/63] cached:120247 (~7515/char)
2023/10/10 00:33:47 L3: [fex=1138117/set:99442300] [del:99320104/bat:99424603] [g/s:147/121] cached:118105 (~7381/char)
2023/10/10 00:33:47 BoltSpeed: 5111.40 tx/s ( did=76584 in 15.0 sec ) totalTX=99452961
Alloc: 931 MiB, TotalAlloc: 11020439 MiB, Sys: 2095 MiB, NumGC: 26679
2023/10/10 00:34:02 L1: [fex=0/set:100668433] [del:99405899/bat:99500049] [g/s:147/118] cached:118457 (~7403/char)
2023/10/10 00:34:02 L2: [fex=1149005/set:99524353] [del:100554035/bat:100650392] [g/s:79/63] cached:119323 (~7457/char)
2023/10/10 00:34:02 L3: [fex=1139986/set:99524353] [del:99403775/bat:99500049] [g/s:147/121] cached:116485 (~7280/char)
2023/10/10 00:34:02 BoltSpeed: 5471.52 tx/s ( did=82068 in 15.0 sec ) totalTX=99535029
2023/10/10 00:34:17 L1: [fex=0/set:100726741] [del:99486702/bat:99542703] [g/s:147/118] cached:94614 (~5913/char)
2023/10/10 00:34:17 L2: [fex=1150360/set:99581316] [del:100635262/bat:100694037] [g/s:79/63] cached:96414 (~6025/char)
2023/10/10 00:34:17 L3: [fex=1141328/set:99581316] [del:99485233/bat:99542703] [g/s:147/121] cached:91987 (~5749/char)
2023/10/10 00:34:17 BoltSpeed: 3798.01 tx/s ( did=56976 in 15.0 sec ) totalTX=99592005
Alloc: 686 MiB, TotalAlloc: 11033058 MiB, Sys: 2095 MiB, NumGC: 26705
2023/10/10 00:34:32 L1: [fex=0/set:100781476] [del:99532863/bat:99583807] [g/s:147/118] cached:101955 (~6372/char)
2023/10/10 00:34:32 L2: [fex=1151606/set:99634814] [del:100684097/bat:100736133] [g/s:79/63] cached:102323 (~6395/char)
2023/10/10 00:34:32 L3: [fex=1142562/set:99634815] [del:99527385/bat:99583807] [g/s:147/121] cached:103332 (~6458/char)
2023/10/10 00:34:32 BoltSpeed: 3567.91 tx/s ( did=53513 in 15.0 sec ) totalTX=99645518
2023/10/10 00:34:47 L1: [fex=0/set:100831071] [del:99576192/bat:99668023] [g/s:147/118] cached:107102 (~6693/char)
2023/10/10 00:34:47 L2: [fex=1152728/set:99683292] [del:100724265/bat:100822275] [g/s:79/63] cached:111755 (~6984/char)
2023/10/10 00:34:47 L3: [fex=1143677/set:99683292] [del:99569922/bat:99668023] [g/s:147/121] cached:109270 (~6829/char)
2023/10/10 00:34:47 BoltSpeed: 3231.57 tx/s ( did=48490 in 15.0 sec ) totalTX=99694008
Alloc: 1434 MiB, TotalAlloc: 11056592 MiB, Sys: 2095 MiB, NumGC: 26753
2023/10/10 00:35:02 L1: [fex=0/set:100905077] [del:99651763/bat:99740947] [g/s:147/118] cached:103905 (~6494/char)
2023/10/10 00:35:02 L2: [fex=1154376/set:99755667] [del:100803920/bat:100896851] [g/s:79/63] cached:106123 (~6632/char)
2023/10/10 00:35:02 L3: [fex=1145308/set:99755667] [del:99650374/bat:99740947] [g/s:147/121] cached:101192 (~6324/char)
2023/10/10 00:35:02 BoltSpeed: 4827.64 tx/s ( did=72390 in 15.0 sec ) totalTX=99766398
2023/10/10 00:35:17 L1: [fex=0/set:100965547] [del:99722694/bat:99781521] [g/s:147/118] cached:92092 (~5755/char)
2023/10/10 00:35:17 L2: [fex=1155741/set:99814782] [del:100877244/bat:100938377] [g/s:79/63] cached:93279 (~5829/char)
2023/10/10 00:35:17 L3: [fex=1146663/set:99814782] [del:99717709/bat:99781521] [g/s:147/121] cached:92972 (~5810/char)
2023/10/10 00:35:17 BoltSpeed: 3941.74 tx/s ( did=59131 in 15.0 sec ) totalTX=99825529
Alloc: 582 MiB, TotalAlloc: 11069489 MiB, Sys: 2095 MiB, NumGC: 26779
2023/10/10 00:35:32 L1: [fex=0/set:101021996] [del:99770899/bat:99823288] [g/s:147/118] cached:99022 (~6188/char)
2023/10/10 00:35:32 L2: [fex=1157069/set:99869917] [del:100923430/bat:100981107] [g/s:79/63] cached:103556 (~6472/char)
2023/10/10 00:35:32 L3: [fex=1147975/set:99869917] [del:99765706/bat:99823288] [g/s:147/121] cached:100110 (~6256/char)
2023/10/10 00:35:32 BoltSpeed: 3676.96 tx/s ( did=55151 in 15.0 sec ) totalTX=99880680
2023/10/10 00:35:47 L1: [fex=0/set:101083526] [del:99815280/bat:99910998] [g/s:147/118] cached:114813 (~7175/char)
2023/10/10 00:35:47 L2: [fex=1158436/set:99930104] [del:100969689/bat:101070876] [g/s:79/63] cached:118851 (~7428/char)
2023/10/10 00:35:47 L3: [fex=1149332/set:99930104] [del:99810089/bat:99910998] [g/s:147/121] cached:115910 (~7244/char)
2023/10/10 00:35:47 BoltSpeed: 4012.05 tx/s ( did=60199 in 15.0 sec ) totalTX=99940879
2023/10/10 00:35:58 End test p=2 nntp-history added=25003092 dupes=0 cLock=43960003 addretry=0 retry=0 adddupes=0 cdupes=10 cretry1=31036895 cretry2=0 sum=100000000/100000000 errors=0 locked=25003092
2023/10/10 00:35:58 End test p=4 nntp-history added=24992638 dupes=0 cLock=43957937 addretry=0 retry=0 adddupes=0 cdupes=22 cretry1=31049403 cretry2=0 sum=100000000/100000000 errors=0 locked=24992638
2023/10/10 00:35:58 End test p=3 nntp-history added=25003319 dupes=0 cLock=43958747 addretry=0 retry=0 adddupes=0 cdupes=19 cretry1=31037915 cretry2=0 sum=100000000/100000000 errors=0 locked=25003319
2023/10/10 00:35:58 End test p=1 nntp-history added=25000951 dupes=0 cLock=43957768 addretry=0 retry=0 adddupes=0 cdupes=20 cretry1=31041261 cretry2=0 sum=100000000/100000000 errors=0 locked=25000951
2023/10/10 00:35:59 CLOSE_HISTORY: his.WriterChan <- nil
2023/10/10 00:35:59 WAIT CLOSE_HISTORY: lock1=true=1 lock2=true=1 lock3=true=16 lock4=true=16 lock5=true=256 batchQueued=true=17957 batchLocked=true=29
2023/10/10 00:36:00 WAIT CLOSE_HISTORY: lock1=false=0 lock2=false=0 lock3=true=16 lock4=true=16 lock5=true=256 batchQueued=true=11772 batchLocked=true=60
2023/10/10 00:36:01 WAIT CLOSE_HISTORY: lock1=false=0 lock2=false=0 lock3=true=16 lock4=true=16 lock5=true=242 batchQueued=true=6173 batchLocked=true=93
2023/10/10 00:36:02 WAIT CLOSE_HISTORY: lock1=false=0 lock2=false=0 lock3=true=16 lock4=true=16 lock5=true=219 batchQueued=true=3196 batchLocked=true=113
Alloc: 906 MiB, TotalAlloc: 11094078 MiB, Sys: 2095 MiB, NumGC: 26830
2023/10/10 00:36:02 L1: [fex=0/set:101155005] [del:99885787/bat:99987938] [g/s:147/118] cached:114213 (~7138/char)
2023/10/10 00:36:02 L2: [fex=1160011/set:100000000] [del:101047272/bat:101149535] [g/s:79/63] cached:112739 (~7046/char)
2023/10/10 00:36:02 L3: [fex=1150896/set:100000000] [del:99883072/bat:99987938] [g/s:147/121] cached:112819 (~7051/char)
2023/10/10 00:36:02 BoltSpeed: 4661.98 tx/s ( did=69908 in 15.0 sec ) totalTX=100010787
2023/10/10 00:36:03 WAIT CLOSE_HISTORY: lock1=false=0 lock2=false=0 lock3=true=16 lock4=true=16 lock5=true=194 batchQueued=true=732 batchLocked=true=142
2023/10/10 00:36:04 WAIT CLOSE_HISTORY: lock1=false=0 lock2=false=0 lock3=true=16 lock4=true=16 lock5=true=164 batchQueued=true=503 batchLocked=true=127
2023/10/10 00:36:05 WAIT CLOSE_HISTORY: lock1=false=0 lock2=false=0 lock3=true=16 lock4=true=16 lock5=true=115 batchQueued=true=238 batchLocked=true=105
2023/10/10 00:36:06 WAIT CLOSE_HISTORY: lock1=false=0 lock2=false=0 lock3=true=14 lock4=true=14 lock5=true=54 batchQueued=true=155 batchLocked=true=51
2023/10/10 00:36:07 WAIT CLOSE_HISTORY: lock1=false=0 lock2=false=0 lock3=false=0 lock4=false=0 lock5=true=2 batchQueued=true=2 batchLocked=false=0
2023/10/10 00:36:08 CLOSE_HISTORY DONE
2023/10/10 00:36:08 key_add=98844995 key_app=1155005 total=100000000 fseeks=1160011 eof=0 BoltDB_decodedOffsets=1150896 addoffset=0 appoffset=1155005 trymultioffsets=9085 tryoffset=1145920 searches=100000000 inserted=100000000
2023/10/10 00:36:08 L1LOCK=100000000 | Get: L2=1168267 L3=100004109 | wCBBS=~134 conti=2511 slept=25337
2023/10/10 00:36:08 done=400000000 (took 16107 seconds) (closewait 9 seconds)
2023/10/10 00:36:08 CrunchBatchLogs: did=643243 dat=643243
2023/10/10 00:36:08 CrunchLogs: inserted=00001:00429 wCBBS=00080:01024 µs=0000002608:0012505535
2023/10/10 00:36:08 Range 005%: inserted=00225:00429 wCBBS=00264:01024 µs=0000002608:0000501288 sum=38595
2023/10/10 00:36:08 Range 010%: inserted=00180:00320 wCBBS=00216:00328 µs=0000017455:0000461395 sum=32162
2023/10/10 00:36:08 Range 015%: inserted=00114:00272 wCBBS=00168:00280 µs=0000020280:0000755994 sum=32162
2023/10/10 00:36:08 Range 020%: inserted=00075:00240 wCBBS=00144:00248 µs=0000020589:0003690030 sum=32163
2023/10/10 00:36:08 Range 025%: inserted=00065:00216 wCBBS=00144:00224 µs=0000020555:0005208682 sum=32162
2023/10/10 00:36:08 Range 030%: inserted=00068:00200 wCBBS=00120:00208 µs=0000019895:0005181401 sum=32162
2023/10/10 00:36:08 Range 035%: inserted=00064:00192 wCBBS=00120:00200 µs=0000023041:0006063212 sum=32162
2023/10/10 00:36:08 Range 040%: inserted=00049:00200 wCBBS=00104:00208 µs=0000023725:0006130449 sum=32162
2023/10/10 00:36:08 Range 045%: inserted=00059:00184 wCBBS=00096:00192 µs=0000024810:0006497194 sum=32162
2023/10/10 00:36:08 Range 050%: inserted=00061:00184 wCBBS=00096:00192 µs=0000025354:0010481807 sum=32162
2023/10/10 00:36:08 Range 055%: inserted=00064:00184 wCBBS=00104:00192 µs=0000026839:0010561813 sum=32163
2023/10/10 00:36:08 Range 060%: inserted=00059:00184 wCBBS=00096:00192 µs=0000025756:0010729793 sum=32162
2023/10/10 00:36:08 Range 065%: inserted=00060:00200 wCBBS=00112:00208 µs=0000028559:0011533487 sum=32162
2023/10/10 00:36:08 Range 070%: inserted=00061:00186 wCBBS=00104:00192 µs=0000024507:0010140712 sum=32162
2023/10/10 00:36:08 Range 075%: inserted=00001:00184 wCBBS=00080:00192 µs=0000007109:0010816168 sum=32162
2023/10/10 00:36:08 Range 080%: inserted=00036:00168 wCBBS=00088:00176 µs=0000027820:0011250077 sum=32162
2023/10/10 00:36:08 Range 085%: inserted=00040:00160 wCBBS=00080:00168 µs=0000023709:0011231477 sum=32162
2023/10/10 00:36:08 Range 090%: inserted=00034:00168 wCBBS=00080:00176 µs=0000026480:0012505535 sum=32163
2023/10/10 00:36:08 Range 095%: inserted=00038:00168 wCBBS=00088:00176 µs=0000026889:0011382038 sum=32162
2023/10/10 00:36:08 Range 100%: inserted=00001:00160 wCBBS=-0001:00168 µs=0000022594:0010847919 sum=25729
Alloc: 458 MiB, TotalAlloc: 11095527 MiB, Sys: 2095 MiB, NumGC: 26834
2023/10/10 00:36:08 runtime.GC() [ 1 / 2 ] sleep 30 sec
2023/10/10 00:36:17 L1: [fex=0/set:101155005] [del:99979092/bat:100000000] [g/s:147/134] cached:20908 (~1306/char)
2023/10/10 00:36:17 L2: [fex=1160011/set:100000000] [del:101140358/bat:101161873] [g/s:79/70] cached:19653 (~1228/char)
2023/10/10 00:36:17 L3: [fex=1150896/set:100000000] [del:99976266/bat:100000000] [g/s:147/138] cached:19625 (~1226/char)
2023/10/10 00:36:32 L1: [fex=0/set:101155005] [del:100000000/bat:100000000] [g/s:147/166] cached:0 (~0/char)
2023/10/10 00:36:32 L2: [fex=1160011/set:100000000] [del:101160011/bat:101161873] [g/s:79/85] cached:0 (~0/char)
Alloc: 197 MiB, TotalAlloc: 11095564 MiB, Sys: 2095 MiB, NumGC: 26835
2023/10/10 00:36:32 L3: [fex=1150896/set:100000000] [del:99995891/bat:100000000] [g/s:147/168] cached:0 (~0/char)
Alloc: 198 MiB, TotalAlloc: 11095565 MiB, Sys: 2095 MiB, NumGC: 26835
2023/10/10 00:36:38 runtime.GC() [ 2 / 2 ] sleep 30 sec
2023/10/10 00:36:47 L1: [fex=0/set:101155005] [del:100000000/bat:100000000] [g/s:147/198] cached:0 (~0/char)
2023/10/10 00:36:47 L2: [fex=1160011/set:100000000] [del:101160011/bat:101161873] [g/s:79/101] cached:0 (~0/char)
2023/10/10 00:36:47 L3: [fex=1150896/set:100000000] [del:99995891/bat:100000000] [g/s:147/199] cached:0 (~0/char)
Alloc: 62 MiB, TotalAlloc: 11095566 MiB, Sys: 2095 MiB, NumGC: 26836
2023/10/10 00:37:02 L1: [fex=0/set:101155005] [del:100000000/bat:100000000] [g/s:147/214] cached:0 (~0/char)
2023/10/10 00:37:02 L2: [fex=1160011/set:100000000] [del:101160011/bat:101161873] [g/s:79/110] cached:0 (~0/char)
2023/10/10 00:37:02 L3: [fex=1150896/set:100000000] [del:99995891/bat:100000000] [g/s:147/201] cached:0 (~0/char)
```

## Checking 400.000.000 `i` hashes (75% duplicates) vs hashdb
```sh
# history.DBG_BS_LOG = true // debugs BatchLOG for every batch insert!
#
./nntp-history-test -todo=100000000
ARGS: CPU=4/12 | jobs=4 | todo=100000000 | total=400000000 | keyalgo=11 | keylen=6 | BatchSize=64
 useHashDB: true | IndexParallel=16
 boltOpts='&bbolt.Options{Timeout:9000000000, NoGrowSync:false, NoFreelistSync:false, PreLoadFreelist:false, FreelistType:"", ReadOnly:false, MmapFlags:0, InitialMmapSize:2147483648, PageSize:65536, NoSync:false, OpenFile:(func(string, int, fs.FileMode) (*os.File, error))(nil), Mlock:false}'
2023/10/10 01:07:56 History: new=false
  HF='history/history.dat' DB='hashdb/history.dat.hash.[0-9a-f]' NumQueueWriteChan=16 DefaultCacheExpires=16
2023/10/10 01:07:56   HashDB:{KeyAlgo=11 KeyLen=6 NumQueueIndexChan=16 NumQueueIndexChans=16 BatchSize=64 IndexParallel=16}
2023/10/10 01:08:11 L1: [fex=515785/set:0] [del:0/bat:0] [g/s:80/0] cached:515789 (~32236/char)
2023/10/10 01:08:11 L2: [fex=515792/set:0] [del:0/bat:0] [g/s:48/0] cached:515792 (~32237/char)
2023/10/10 01:08:11 L3: [fex=515758/set:0] [del:0/bat:0] [g/s:80/0] cached:515758 (~32234/char)
2023/10/10 01:08:11 BoltSpeed: 34384.57 tx/s ( did=515777 in 15.0 sec ) totalTX=515777
Alloc: 869 MiB, TotalAlloc: 8601 MiB, Sys: 977 MiB, NumGC: 95
2023/10/10 01:08:26 L1: [fex=1003154/set:0] [del:412925/bat:0] [g/s:80/0] cached:590230 (~36889/char)
2023/10/10 01:08:26 L2: [fex=1003180/set:0] [del:412925/bat:0] [g/s:48/0] cached:590255 (~36890/char)
2023/10/10 01:08:26 L3: [fex=1003065/set:0] [del:412906/bat:0] [g/s:80/0] cached:590159 (~36884/char)
2023/10/10 01:08:26 BoltSpeed: 32488.42 tx/s ( did=487322 in 15.0 sec ) totalTX=1003099
2023/10/10 01:08:41 L1: [fex=1460906/set:0] [del:939381/bat:0] [g/s:80/0] cached:521528 (~32595/char)
2023/10/10 01:08:41 L2: [fex=1461010/set:0] [del:939398/bat:0] [g/s:48/0] cached:521612 (~32600/char)
2023/10/10 01:08:41 L3: [fex=1460748/set:0] [del:939301/bat:0] [g/s:80/0] cached:521447 (~32590/char)
2023/10/10 01:08:41 BoltSpeed: 30513.57 tx/s ( did=457700 in 15.0 sec ) totalTX=1460799
2023/10/10 01:08:56 L1: [fex=1939822/set:0] [del:1314651/bat:0] [g/s:80/0] cached:625173 (~39073/char)
2023/10/10 01:08:56 L2: [fex=1940052/set:0] [del:1314727/bat:0] [g/s:48/0] cached:625325 (~39082/char)
2023/10/10 01:08:56 L3: [fex=1939603/set:0] [del:1314510/bat:0] [g/s:80/0] cached:625093 (~39068/char)
2023/10/10 01:08:56 BoltSpeed: 31924.44 tx/s ( did=478870 in 15.0 sec ) totalTX=1939669
Alloc: 838 MiB, TotalAlloc: 16479 MiB, Sys: 1011 MiB, NumGC: 113
2023/10/10 01:09:11 L1: [fex=2426170/set:0] [del:1809841/bat:0] [g/s:80/0] cached:616333 (~38520/char)
2023/10/10 01:09:11 L2: [fex=2426590/set:0] [del:1810027/bat:0] [g/s:48/0] cached:616563 (~38535/char)
2023/10/10 01:09:11 L3: [fex=2425889/set:0] [del:1809638/bat:0] [g/s:80/0] cached:616251 (~38515/char)
2023/10/10 01:09:11 BoltSpeed: 32402.80 tx/s ( did=486315 in 15.0 sec ) totalTX=2425984
Alloc: 481 MiB, TotalAlloc: 24563 MiB, Sys: 1011 MiB, NumGC: 133
2023/10/10 01:09:26 L1: [fex=2899344/set:0] [del:2329887/bat:0] [g/s:80/0] cached:569459 (~35591/char)
2023/10/10 01:09:26 L2: [fex=2899995/set:0] [del:2330264/bat:0] [g/s:48/0] cached:569731 (~35608/char)
2023/10/10 01:09:26 L3: [fex=2898982/set:0] [del:2329614/bat:0] [g/s:80/0] cached:569368 (~35585/char)
2023/10/10 01:09:26 BoltSpeed: 31557.76 tx/s ( did=473097 in 15.0 sec ) totalTX=2899081
2023/10/10 01:09:41 L1: [fex=3366545/set:0] [del:2833474/bat:0] [g/s:80/0] cached:533073 (~33317/char)
2023/10/10 01:09:41 L2: [fex=3367498/set:0] [del:2834089/bat:0] [g/s:48/0] cached:533409 (~33338/char)
2023/10/10 01:09:41 L3: [fex=3366122/set:0] [del:2833122/bat:0] [g/s:80/0] cached:533000 (~33312/char)
2023/10/10 01:09:41 BoltSpeed: 31143.41 tx/s ( did=467155 in 15.0 sec ) totalTX=3366236
Alloc: 593 MiB, TotalAlloc: 32574 MiB, Sys: 1016 MiB, NumGC: 152
2023/10/10 01:09:56 L1: [fex=3852777/set:0] [del:3203663/bat:0] [g/s:80/0] cached:649117 (~40569/char)
2023/10/10 01:09:56 L2: [fex=3854059/set:0] [del:3211933/bat:0] [g/s:48/0] cached:642126 (~40132/char)
2023/10/10 01:09:56 L3: [fex=3852281/set:0] [del:3203256/bat:0] [g/s:80/0] cached:649025 (~40564/char)
2023/10/10 01:09:56 BoltSpeed: 32411.97 tx/s ( did=486176 in 15.0 sec ) totalTX=3852412
2023/10/10 01:10:11 L1: [fex=4306654/set:0] [del:3724304/bat:0] [g/s:80/0] cached:582352 (~36397/char)
2023/10/10 01:10:11 L2: [fex=4308277/set:0] [del:3738312/bat:0] [g/s:48/0] cached:569965 (~35622/char)
2023/10/10 01:10:11 L3: [fex=4306083/set:0] [del:3723828/bat:0] [g/s:80/0] cached:582255 (~36390/char)
2023/10/10 01:10:11 BoltSpeed: 30229.36 tx/s ( did=453816 in 15.0 sec ) totalTX=4306228
2023/10/10 01:10:26 L1: [fex=4779905/set:0] [del:4207037/bat:0] [g/s:80/0] cached:572868 (~35804/char)
2023/10/10 01:10:26 L2: [fex=4781972/set:0] [del:4238677/bat:0] [g/s:48/0] cached:543295 (~33955/char)
2023/10/10 01:10:26 L3: [fex=4779281/set:0] [del:4206485/bat:0] [g/s:80/0] cached:572796 (~35799/char)
2023/10/10 01:10:26 BoltSpeed: 31572.03 tx/s ( did=473214 in 15.0 sec ) totalTX=4779442
Alloc: 661 MiB, TotalAlloc: 40393 MiB, Sys: 1016 MiB, NumGC: 171
2023/10/10 01:10:41 L1: [fex=5259556/set:0] [del:4710579/bat:0] [g/s:80/0] cached:548981 (~34311/char)
2023/10/10 01:10:41 L2: [fex=5262170/set:0] [del:4695275/bat:0] [g/s:48/0] cached:566895 (~35430/char)
2023/10/10 01:10:41 L3: [fex=5258897/set:0] [del:4709955/bat:0] [g/s:80/0] cached:548942 (~34308/char)
2023/10/10 01:10:41 BoltSpeed: 31913.49 tx/s ( did=479631 in 15.0 sec ) totalTX=5259073
Alloc: 584 MiB, TotalAlloc: 48505 MiB, Sys: 1016 MiB, NumGC: 191
2023/10/10 01:10:56 L1: [fex=5745360/set:0] [del:5096301/bat:0] [g/s:80/0] cached:649062 (~40566/char)
2023/10/10 01:10:56 L2: [fex=5748525/set:0] [del:5129944/bat:0] [g/s:48/0] cached:618581 (~38661/char)
2023/10/10 01:10:56 L3: [fex=5744596/set:0] [del:5097858/bat:0] [g/s:80/0] cached:646738 (~40421/char)
2023/10/10 01:10:56 BoltSpeed: 32445.69 tx/s ( did=485717 in 15.0 sec ) totalTX=5744790
2023/10/10 01:11:11 L1: [fex=6223613/set:0] [del:5624513/bat:0] [g/s:80/0] cached:599102 (~37443/char)
2023/10/10 01:11:11 L2: [fex=6227396/set:0] [del:5642190/bat:0] [g/s:48/0] cached:585206 (~36575/char)
2023/10/10 01:11:11 L3: [fex=6222778/set:0] [del:5625402/bat:0] [g/s:80/0] cached:597376 (~37336/char)
2023/10/10 01:11:11 BoltSpeed: 31880.23 tx/s ( did=478199 in 15.0 sec ) totalTX=6222989
Alloc: 635 MiB, TotalAlloc: 56730 MiB, Sys: 1016 MiB, NumGC: 211
2023/10/10 01:11:26 L1: [fex=6721709/set:0] [del:6132320/bat:0] [g/s:80/0] cached:589392 (~36837/char)
2023/10/10 01:11:26 L2: [fex=6726150/set:0] [del:6156598/bat:0] [g/s:48/0] cached:569552 (~35597/char)
2023/10/10 01:11:26 L3: [fex=6720805/set:0] [del:6140540/bat:0] [g/s:80/0] cached:580265 (~36266/char)
2023/10/10 01:11:26 BoltSpeed: 33202.76 tx/s ( did=498043 in 15.0 sec ) totalTX=6721032
2023/10/10 01:11:41 L1: [fex=7193382/set:0] [del:6645614/bat:0] [g/s:80/0] cached:547771 (~34235/char)
2023/10/10 01:11:41 L2: [fex=7198526/set:0] [del:6554926/bat:0] [g/s:48/0] cached:643600 (~40225/char)
2023/10/10 01:11:41 L3: [fex=7192414/set:0] [del:6653263/bat:0] [g/s:80/0] cached:539151 (~33696/char)
2023/10/10 01:11:41 BoltSpeed: 31441.54 tx/s ( did=471624 in 15.0 sec ) totalTX=7192656
Alloc: 451 MiB, TotalAlloc: 64644 MiB, Sys: 1016 MiB, NumGC: 231
2023/10/10 01:11:56 L1: [fex=7662390/set:0] [del:7064425/bat:0] [g/s:80/0] cached:597968 (~37373/char)
2023/10/10 01:11:56 L2: [fex=7668314/set:0] [del:7076149/bat:0] [g/s:48/0] cached:592165 (~37010/char)
2023/10/10 01:11:56 L3: [fex=7661363/set:0] [del:7065703/bat:0] [g/s:80/0] cached:595660 (~37228/char)
2023/10/10 01:11:56 BoltSpeed: 31264.39 tx/s ( did=468965 in 15.0 sec ) totalTX=7661621
2023/10/10 01:12:11 L1: [fex=8147839/set:0] [del:7559394/bat:0] [g/s:80/0] cached:588448 (~36778/char)
2023/10/10 01:12:11 L2: [fex=8154598/set:0] [del:7568677/bat:0] [g/s:48/0] cached:585921 (~36620/char)
2023/10/10 01:12:11 L3: [fex=8146751/set:0] [del:7560075/bat:0] [g/s:80/0] cached:586676 (~36667/char)
2023/10/10 01:12:11 BoltSpeed: 32360.35 tx/s ( did=485405 in 15.0 sec ) totalTX=8147026
Alloc: 844 MiB, TotalAlloc: 72630 MiB, Sys: 1016 MiB, NumGC: 250
2023/10/10 01:12:26 L1: [fex=8609968/set:0] [del:8076395/bat:0] [g/s:80/0] cached:533575 (~33348/char)
2023/10/10 01:12:26 L2: [fex=8617562/set:0] [del:8083017/bat:0] [g/s:48/0] cached:534545 (~33409/char)
2023/10/10 01:12:26 L3: [fex=8608820/set:0] [del:8075309/bat:0] [g/s:80/0] cached:533511 (~33344/char)
2023/10/10 01:12:26 BoltSpeed: 30805.46 tx/s ( did=462085 in 15.0 sec ) totalTX=8609111
2023/10/10 01:12:41 L1: [fex=9067015/set:0] [del:8487250/bat:0] [g/s:80/0] cached:579767 (~36235/char)
2023/10/10 01:12:41 L2: [fex=9075466/set:0] [del:8472106/bat:0] [g/s:48/0] cached:603360 (~37710/char)
2023/10/10 01:12:41 L3: [fex=9065798/set:0] [del:8478416/bat:0] [g/s:80/0] cached:587382 (~36711/char)
2023/10/10 01:12:41 BoltSpeed: 30466.47 tx/s ( did=456993 in 15.0 sec ) totalTX=9066104
Alloc: 769 MiB, TotalAlloc: 80260 MiB, Sys: 1016 MiB, NumGC: 269
2023/10/10 01:12:56 L1: [fex=9516765/set:0] [del:8941971/bat:0] [g/s:80/0] cached:574796 (~35924/char)
2023/10/10 01:12:56 L2: [fex=9526100/set:0] [del:8959754/bat:0] [g/s:48/0] cached:566346 (~35396/char)
2023/10/10 01:12:56 L3: [fex=9515488/set:0] [del:8940777/bat:0] [g/s:80/0] cached:574711 (~35919/char)
2023/10/10 01:12:56 BoltSpeed: 29979.20 tx/s ( did=449707 in 15.0 sec ) totalTX=9515811
2023/10/10 01:13:10 RUN test p=3 nntp-history added=0 dupes=2501009 cLock=4900648 addretry=0 retry=0 adddupes=0 cdupes=2598343 cretry1=0 cretry2=0 10000000/100000000
2023/10/10 01:13:10 RUN test p=2 nntp-history added=0 dupes=2498311 cLock=4897995 addretry=0 retry=0 adddupes=0 cdupes=2603694 cretry1=0 cretry2=0 10000000/100000000
2023/10/10 01:13:10 RUN test p=4 nntp-history added=0 dupes=2499256 cLock=4899057 addretry=0 retry=0 adddupes=0 cdupes=2601687 cretry1=0 cretry2=0 10000000/100000000
2023/10/10 01:13:10 RUN test p=1 nntp-history added=0 dupes=2501424 cLock=4901804 addretry=0 retry=0 adddupes=0 cdupes=2596772 cretry1=0 cretry2=0 10000000/100000000
2023/10/10 01:13:11 L1: [fex=10018670/set:0] [del:9422068/bat:0] [g/s:80/0] cached:596604 (~37287/char)
2023/10/10 01:13:11 L2: [fex=10029077/set:0] [del:9455686/bat:0] [g/s:48/0] cached:573391 (~35836/char)
2023/10/10 01:13:11 L3: [fex=10017354/set:0] [del:9420803/bat:0] [g/s:80/0] cached:596551 (~37284/char)
2023/10/10 01:13:11 BoltSpeed: 33460.19 tx/s ( did=501882 in 15.0 sec ) totalTX=10017693
2023/10/10 01:13:26 L1: [fex=10509643/set:0] [del:9950307/bat:0] [g/s:80/0] cached:559340 (~34958/char)
Alloc: 544 MiB, TotalAlloc: 88622 MiB, Sys: 1016 MiB, NumGC: 290
2023/10/10 01:13:26 L2: [fex=10521134/set:0] [del:9923877/bat:0] [g/s:48/0] cached:597257 (~37328/char)
2023/10/10 01:13:26 L3: [fex=10508268/set:0] [del:9948995/bat:0] [g/s:80/0] cached:559273 (~34954/char)
2023/10/10 01:13:26 BoltSpeed: 32704.82 tx/s ( did=490927 in 15.0 sec ) totalTX=10508620
2023/10/10 01:13:41 L1: [fex=10992386/set:0] [del:10344870/bat:0] [g/s:80/0] cached:647517 (~40469/char)
2023/10/10 01:13:41 L2: [fex=11005008/set:0] [del:10383386/bat:0] [g/s:48/0] cached:621622 (~38851/char)
2023/10/10 01:13:41 L3: [fex=10990945/set:0] [del:10343513/bat:0] [g/s:80/0] cached:647432 (~40464/char)
2023/10/10 01:13:41 BoltSpeed: 32202.56 tx/s ( did=482695 in 15.0 sec ) totalTX=10991315
Alloc: 737 MiB, TotalAlloc: 96420 MiB, Sys: 1016 MiB, NumGC: 308
2023/10/10 01:13:56 L1: [fex=11436331/set:0] [del:10855910/bat:0] [g/s:80/0] cached:580424 (~36276/char)
2023/10/10 01:13:56 L2: [fex=11450093/set:0] [del:10903292/bat:0] [g/s:48/0] cached:546801 (~34175/char)
2023/10/10 01:13:56 L3: [fex=11434837/set:0] [del:10854484/bat:0] [g/s:80/0] cached:580353 (~36272/char)
2023/10/10 01:13:56 BoltSpeed: 29563.40 tx/s ( did=443908 in 15.0 sec ) totalTX=11435223
2023/10/10 01:14:11 L1: [fex=11881360/set:0] [del:11342051/bat:0] [g/s:80/0] cached:539311 (~33706/char)
2023/10/10 01:14:11 L2: [fex=11896295/set:0] [del:11379276/bat:0] [g/s:48/0] cached:517019 (~32313/char)
2023/10/10 01:14:11 L3: [fex=11879807/set:0] [del:11340566/bat:0] [g/s:80/0] cached:539241 (~33702/char)
2023/10/10 01:14:11 BoltSpeed: 29696.59 tx/s ( did=444986 in 15.0 sec ) totalTX=11880209
Alloc: 616 MiB, TotalAlloc: 104127 MiB, Sys: 1016 MiB, NumGC: 327
2023/10/10 01:14:26 L1: [fex=12351187/set:0] [del:11824371/bat:0] [g/s:80/0] cached:526818 (~32926/char)
2023/10/10 01:14:26 L2: [fex=12367414/set:0] [del:11743514/bat:0] [g/s:48/0] cached:623900 (~38993/char)
2023/10/10 01:14:26 L3: [fex=12349567/set:0] [del:11822823/bat:0] [g/s:80/0] cached:526744 (~32921/char)
2023/10/10 01:14:26 BoltSpeed: 31318.39 tx/s ( did=469777 in 15.0 sec ) totalTX=12349986
2023/10/10 01:14:41 L1: [fex=12822620/set:0] [del:12189611/bat:0] [g/s:80/0] cached:633011 (~39563/char)
2023/10/10 01:14:41 L2: [fex=12840213/set:0] [del:12238904/bat:0] [g/s:48/0] cached:601309 (~37581/char)
2023/10/10 01:14:41 L3: [fex=12820939/set:0] [del:12188014/bat:0] [g/s:80/0] cached:632925 (~39557/char)
2023/10/10 01:14:41 BoltSpeed: 31388.74 tx/s ( did=471387 in 15.0 sec ) totalTX=12821373
Alloc: 836 MiB, TotalAlloc: 111899 MiB, Sys: 1016 MiB, NumGC: 345
2023/10/10 01:14:56 L1: [fex=13275127/set:0] [del:12687846/bat:0] [g/s:80/0] cached:587283 (~36705/char)
2023/10/10 01:14:56 L2: [fex=13294015/set:0] [del:12738779/bat:0] [g/s:48/0] cached:555236 (~34702/char)
2023/10/10 01:14:56 L3: [fex=13273384/set:0] [del:12686179/bat:0] [g/s:80/0] cached:587205 (~36700/char)
2023/10/10 01:14:56 BoltSpeed: 30199.19 tx/s ( did=452461 in 15.0 sec ) totalTX=13273834
2023/10/10 01:15:11 L1: [fex=13743617/set:0] [del:13179201/bat:0] [g/s:80/0] cached:564419 (~35276/char)
2023/10/10 01:15:11 L2: [fex=13763860/set:0] [del:13232718/bat:0] [g/s:48/0] cached:531142 (~33196/char)
2023/10/10 01:15:11 L3: [fex=13741800/set:0] [del:13177471/bat:0] [g/s:80/0] cached:564329 (~35270/char)
2023/10/10 01:15:11 BoltSpeed: 31229.47 tx/s ( did=468433 in 15.0 sec ) totalTX=13742267
Alloc: 752 MiB, TotalAlloc: 119722 MiB, Sys: 1016 MiB, NumGC: 364
2023/10/10 01:15:26 L1: [fex=14203775/set:0] [del:13676284/bat:0] [g/s:80/0] cached:527494 (~32968/char)
2023/10/10 01:15:26 L2: [fex=14225468/set:0] [del:13600523/bat:0] [g/s:48/0] cached:624945 (~39059/char)
2023/10/10 01:15:26 L3: [fex=14201886/set:0] [del:13674475/bat:0] [g/s:80/0] cached:527411 (~32963/char)
2023/10/10 01:15:26 BoltSpeed: 30673.50 tx/s ( did=460102 in 15.0 sec ) totalTX=14202369
2023/10/10 01:15:41 L1: [fex=14662595/set:0] [del:14052408/bat:0] [g/s:80/0] cached:610189 (~38136/char)
2023/10/10 01:15:41 L2: [fex=14685748/set:0] [del:14090964/bat:0] [g/s:48/0] cached:594784 (~37174/char)
2023/10/10 01:15:41 L3: [fex=14660633/set:0] [del:14047651/bat:0] [g/s:80/0] cached:612982 (~38311/char)
2023/10/10 01:15:41 BoltSpeed: 30584.08 tx/s ( did=458762 in 15.0 sec ) totalTX=14661131
Alloc: 802 MiB, TotalAlloc: 127705 MiB, Sys: 1016 MiB, NumGC: 383
2023/10/10 01:15:56 L1: [fex=15152798/set:0] [del:14547292/bat:0] [g/s:80/0] cached:605507 (~37844/char)
2023/10/10 01:15:56 L2: [fex=15177642/set:0] [del:14593036/bat:0] [g/s:48/0] cached:584606 (~36537/char)
2023/10/10 01:15:56 L3: [fex=15150763/set:0] [del:14543879/bat:0] [g/s:80/0] cached:606884 (~37930/char)
2023/10/10 01:15:56 BoltSpeed: 32675.37 tx/s ( did=490146 in 15.0 sec ) totalTX=15151277
2023/10/10 01:16:11 L1: [fex=15631475/set:0] [del:15075786/bat:0] [g/s:80/0] cached:555691 (~34730/char)
2023/10/10 01:16:11 L2: [fex=15657971/set:0] [del:15131775/bat:0] [g/s:48/0] cached:526196 (~32887/char)
2023/10/10 01:16:11 L3: [fex=15629371/set:0] [del:15073852/bat:0] [g/s:80/0] cached:555519 (~34719/char)
2023/10/10 01:16:11 BoltSpeed: 31909.32 tx/s ( did=478624 in 15.0 sec ) totalTX=15629901
Alloc: 472 MiB, TotalAlloc: 135868 MiB, Sys: 1016 MiB, NumGC: 404
2023/10/10 01:16:26 L1: [fex=16122133/set:0] [del:15552464/bat:0] [g/s:80/0] cached:569670 (~35604/char)
2023/10/10 01:16:26 L2: [fex=16150359/set:0] [del:15533464/bat:0] [g/s:48/0] cached:616895 (~38555/char)
2023/10/10 01:16:26 L3: [fex=16119978/set:0] [del:15565267/bat:0] [g/s:80/0] cached:554711 (~34669/char)
2023/10/10 01:16:26 BoltSpeed: 32704.58 tx/s ( did=490622 in 15.0 sec ) totalTX=16120523
2023/10/10 01:16:41 L1: [fex=16611240/set:0] [del:15986070/bat:0] [g/s:80/0] cached:625171 (~39073/char)
2023/10/10 01:16:41 L2: [fex=16641219/set:0] [del:16047396/bat:0] [g/s:48/0] cached:593823 (~37113/char)
2023/10/10 01:16:41 L3: [fex=16609019/set:0] [del:15987677/bat:0] [g/s:80/0] cached:621342 (~38833/char)
2023/10/10 01:16:41 BoltSpeed: 32607.53 tx/s ( did=489058 in 15.0 sec ) totalTX=16609581
Alloc: 842 MiB, TotalAlloc: 144055 MiB, Sys: 1016 MiB, NumGC: 423
2023/10/10 01:16:56 L1: [fex=17094774/set:0] [del:16507407/bat:0] [g/s:80/0] cached:587367 (~36710/char)
2023/10/10 01:16:56 L2: [fex=17126592/set:0] [del:16572089/bat:0] [g/s:48/0] cached:554503 (~34656/char)
2023/10/10 01:16:56 L3: [fex=17092499/set:0] [del:16505196/bat:0] [g/s:80/0] cached:587303 (~36706/char)
2023/10/10 01:16:56 BoltSpeed: 32231.90 tx/s ( did=483495 in 15.0 sec ) totalTX=17093076
2023/10/10 01:17:11 L1: [fex=17575417/set:0] [del:17036329/bat:0] [g/s:80/0] cached:539090 (~33693/char)
2023/10/10 01:17:11 L2: [fex=17609187/set:0] [del:16963387/bat:0] [g/s:48/0] cached:645800 (~40362/char)
2023/10/10 01:17:11 L3: [fex=17573080/set:0] [del:17034065/bat:0] [g/s:80/0] cached:539015 (~33688/char)
2023/10/10 01:17:11 BoltSpeed: 32040.92 tx/s ( did=480598 in 15.0 sec ) totalTX=17573674
Alloc: 743 MiB, TotalAlloc: 152167 MiB, Sys: 1016 MiB, NumGC: 443
2023/10/10 01:17:26 L1: [fex=18059503/set:0] [del:17415370/bat:0] [g/s:80/0] cached:644136 (~40258/char)
2023/10/10 01:17:26 L2: [fex=18095261/set:0] [del:17472486/bat:0] [g/s:48/0] cached:622775 (~38923/char)
2023/10/10 01:17:26 L3: [fex=18057103/set:0] [del:17413047/bat:0] [g/s:80/0] cached:644056 (~40253/char)
2023/10/10 01:17:26 BoltSpeed: 32269.19 tx/s ( did=484040 in 15.0 sec ) totalTX=18057714
2023/10/10 01:17:41 L1: [fex=18514349/set:0] [del:17922218/bat:0] [g/s:80/0] cached:592134 (~37008/char)
2023/10/10 01:17:41 L2: [fex=18552003/set:0] [del:17985148/bat:0] [g/s:48/0] cached:566855 (~35428/char)
2023/10/10 01:17:41 L3: [fex=18511893/set:0] [del:17919837/bat:0] [g/s:80/0] cached:592056 (~37003/char)
2023/10/10 01:17:41 BoltSpeed: 30320.03 tx/s ( did=454807 in 15.0 sec ) totalTX=18512521
Alloc: 714 MiB, TotalAlloc: 159980 MiB, Sys: 1016 MiB, NumGC: 462
2023/10/10 01:17:56 L1: [fex=18986327/set:0] [del:18417199/bat:0] [g/s:80/0] cached:569131 (~35570/char)
2023/10/10 01:17:56 L2: [fex=19025974/set:0] [del:18484493/bat:0] [g/s:48/0] cached:541481 (~33842/char)
2023/10/10 01:17:56 L3: [fex=18983798/set:0] [del:18414749/bat:0] [g/s:80/0] cached:569049 (~35565/char)
2023/10/10 01:17:56 BoltSpeed: 31461.79 tx/s ( did=471920 in 15.0 sec ) totalTX=18984441
2023/10/10 01:18:11 L1: [fex=19456396/set:0] [del:18919184/bat:0] [g/s:80/0] cached:537214 (~33575/char)
2023/10/10 01:18:11 L2: [fex=19498108/set:0] [del:18862227/bat:0] [g/s:48/0] cached:635881 (~39742/char)
2023/10/10 01:18:11 L3: [fex=19453811/set:0] [del:18916660/bat:0] [g/s:80/0] cached:537151 (~33571/char)
2023/10/10 01:18:11 BoltSpeed: 31335.36 tx/s ( did=470029 in 15.0 sec ) totalTX=19454470
Alloc: 592 MiB, TotalAlloc: 167942 MiB, Sys: 1016 MiB, NumGC: 482
2023/10/10 01:18:26 L1: [fex=19933515/set:0] [del:19293966/bat:0] [g/s:80/0] cached:639552 (~39972/char)
2023/10/10 01:18:26 L2: [fex=19977381/set:0] [del:19368252/bat:0] [g/s:48/0] cached:609129 (~38070/char)
2023/10/10 01:18:26 L3: [fex=19930871/set:0] [del:19291400/bat:0] [g/s:80/0] cached:639471 (~39966/char)
2023/10/10 01:18:26 BoltSpeed: 31805.04 tx/s ( did=477075 in 15.0 sec ) totalTX=19931545
2023/10/10 01:18:28 RUN test p=2 nntp-history added=0 dupes=4996093 cLock=9780786 addretry=0 retry=0 adddupes=0 cdupes=5223121 cretry1=0 cretry2=0 20000000/100000000
2023/10/10 01:18:28 RUN test p=1 nntp-history added=0 dupes=5003262 cLock=9788080 addretry=0 retry=0 adddupes=0 cdupes=5208658 cretry1=0 cretry2=0 20000000/100000000
2023/10/10 01:18:28 RUN test p=4 nntp-history added=0 dupes=5000405 cLock=9784924 addretry=0 retry=0 adddupes=0 cdupes=5214671 cretry1=0 cretry2=0 20000000/100000000
2023/10/10 01:18:28 RUN test p=3 nntp-history added=0 dupes=5000240 cLock=9783131 addretry=0 retry=0 adddupes=0 cdupes=5216629 cretry1=0 cretry2=0 20000000/100000000
2023/10/10 01:18:41 L1: [fex=20398074/set:0] [del:19805765/bat:0] [g/s:80/0] cached:592310 (~37019/char)
2023/10/10 01:18:41 L2: [fex=20443991/set:0] [del:19880465/bat:0] [g/s:48/0] cached:563526 (~35220/char)
2023/10/10 01:18:41 L3: [fex=20395350/set:0] [del:19803140/bat:0] [g/s:80/0] cached:592210 (~37013/char)
2023/10/10 01:18:41 BoltSpeed: 30966.21 tx/s ( did=464494 in 15.0 sec ) totalTX=20396039
2023/10/10 01:18:56 L1: [fex=20855348/set:0] [del:20296846/bat:0] [g/s:80/0] cached:558503 (~34906/char)
2023/10/10 01:18:56 L2: [fex=20903371/set:0] [del:20397923/bat:0] [g/s:48/0] cached:505448 (~31590/char)
2023/10/10 01:18:56 L3: [fex=20852562/set:0] [del:20294138/bat:0] [g/s:80/0] cached:558424 (~34901/char)
2023/10/10 01:18:56 BoltSpeed: 30481.86 tx/s ( did=457228 in 15.0 sec ) totalTX=20853267
Alloc: 718 MiB, TotalAlloc: 175719 MiB, Sys: 1016 MiB, NumGC: 501
2023/10/10 01:19:11 L1: [fex=21315441/set:0] [del:20788775/bat:0] [g/s:80/0] cached:526668 (~32916/char)
2023/10/10 01:19:11 L2: [fex=21365732/set:0] [del:20771617/bat:0] [g/s:48/0] cached:594115 (~37132/char)
2023/10/10 01:19:11 L3: [fex=21312593/set:0] [del:20785998/bat:0] [g/s:80/0] cached:526595 (~32912/char)
2023/10/10 01:19:11 BoltSpeed: 30670.02 tx/s ( did=460050 in 15.0 sec ) totalTX=21313317
Alloc: 725 MiB, TotalAlloc: 183518 MiB, Sys: 1016 MiB, NumGC: 520
2023/10/10 01:19:26 L1: [fex=21782986/set:0] [del:21166729/bat:0] [g/s:80/0] cached:616259 (~38516/char)
2023/10/10 01:19:26 L2: [fex=21835613/set:0] [del:21277011/bat:0] [g/s:48/0] cached:558602 (~34912/char)
2023/10/10 01:19:26 L3: [fex=21780068/set:0] [del:21166911/bat:0] [g/s:80/0] cached:613157 (~38322/char)
2023/10/10 01:19:26 BoltSpeed: 31166.01 tx/s ( did=467490 in 15.0 sec ) totalTX=21780807
2023/10/10 01:19:41 L1: [fex=22219076/set:0] [del:21656077/bat:0] [g/s:80/0] cached:563000 (~35187/char)
2023/10/10 01:19:41 L2: [fex=22273868/set:0] [del:21763449/bat:0] [g/s:48/0] cached:510419 (~31901/char)
2023/10/10 01:19:41 L3: [fex=22216100/set:0] [del:21655316/bat:0] [g/s:80/0] cached:560784 (~35049/char)
2023/10/10 01:19:41 BoltSpeed: 29069.85 tx/s ( did=436048 in 15.0 sec ) totalTX=22216855
Alloc: 616 MiB, TotalAlloc: 191193 MiB, Sys: 1016 MiB, NumGC: 539
2023/10/10 01:19:56 L1: [fex=22693401/set:0] [del:22147463/bat:0] [g/s:80/0] cached:545940 (~34121/char)
2023/10/10 01:19:56 L2: [fex=22750622/set:0] [del:22121302/bat:0] [g/s:48/0] cached:629320 (~39332/char)
2023/10/10 01:19:56 L3: [fex=22690365/set:0] [del:22138017/bat:0] [g/s:80/0] cached:552348 (~34521/char)
2023/10/10 01:19:56 BoltSpeed: 31618.87 tx/s ( did=474284 in 15.0 sec ) totalTX=22691139
2023/10/10 01:20:11 L1: [fex=23158277/set:0] [del:22628989/bat:0] [g/s:80/0] cached:529289 (~33080/char)
2023/10/10 01:20:11 L2: [fex=23217906/set:0] [del:22620211/bat:0] [g/s:48/0] cached:597695 (~37355/char)
2023/10/10 01:20:11 L3: [fex=23155186/set:0] [del:22623349/bat:0] [g/s:80/0] cached:531837 (~33239/char)
2023/10/10 01:20:11 BoltSpeed: 30961.69 tx/s ( did=464840 in 15.0 sec ) totalTX=23155979
Alloc: 690 MiB, TotalAlloc: 199100 MiB, Sys: 1016 MiB, NumGC: 558
2023/10/10 01:20:26 L1: [fex=23633360/set:0] [del:23027816/bat:0] [g/s:80/0] cached:605547 (~37846/char)
2023/10/10 01:20:26 L2: [fex=23695466/set:0] [del:23117264/bat:0] [g/s:48/0] cached:578202 (~36137/char)
2023/10/10 01:20:26 L3: [fex=23630207/set:0] [del:23022660/bat:0] [g/s:80/0] cached:607547 (~37971/char)
2023/10/10 01:20:26 BoltSpeed: 31697.08 tx/s ( did=475031 in 15.0 sec ) totalTX=23631010
2023/10/10 01:20:41 L1: [fex=24104534/set:0] [del:23532566/bat:0] [g/s:80/0] cached:571972 (~35748/char)
2023/10/10 01:20:41 L2: [fex=24169209/set:0] [del:23619606/bat:0] [g/s:48/0] cached:549603 (~34350/char)
2023/10/10 01:20:41 L3: [fex=24101321/set:0] [del:23527205/bat:0] [g/s:80/0] cached:574116 (~35882/char)
2023/10/10 01:20:41 BoltSpeed: 31408.77 tx/s ( did=471130 in 15.0 sec ) totalTX=24102140
Alloc: 792 MiB, TotalAlloc: 207091 MiB, Sys: 1016 MiB, NumGC: 577
2023/10/10 01:20:56 L1: [fex=24581628/set:0] [del:24044254/bat:0] [g/s:80/0] cached:537376 (~33586/char)
2023/10/10 01:20:56 L2: [fex=24649035/set:0] [del:24006360/bat:0] [g/s:48/0] cached:642675 (~40167/char)
2023/10/10 01:20:56 L3: [fex=24578355/set:0] [del:24041048/bat:0] [g/s:80/0] cached:537307 (~33581/char)
2023/10/10 01:20:56 BoltSpeed: 31802.65 tx/s ( did=477050 in 15.0 sec ) totalTX=24579190
2023/10/10 01:21:11 L1: [fex=25058387/set:0] [del:24446714/bat:0] [g/s:80/0] cached:611675 (~38229/char)
2023/10/10 01:21:11 L2: [fex=25128363/set:0] [del:24514100/bat:0] [g/s:48/0] cached:614263 (~38391/char)
2023/10/10 01:21:11 L3: [fex=25055041/set:0] [del:24450711/bat:0] [g/s:80/0] cached:604330 (~37770/char)
2023/10/10 01:21:11 BoltSpeed: 31780.68 tx/s ( did=476701 in 15.0 sec ) totalTX=25055891
Alloc: 486 MiB, TotalAlloc: 214892 MiB, Sys: 1016 MiB, NumGC: 596
2023/10/10 01:21:26 L1: [fex=25508673/set:0] [del:24930265/bat:0] [g/s:80/0] cached:578410 (~36150/char)
2023/10/10 01:21:26 L2: [fex=25581210/set:0] [del:25021380/bat:0] [g/s:48/0] cached:559830 (~34989/char)
2023/10/10 01:21:26 L3: [fex=25505278/set:0] [del:24926944/bat:0] [g/s:80/0] cached:578334 (~36145/char)
2023/10/10 01:21:26 BoltSpeed: 30016.98 tx/s ( did=450254 in 15.0 sec ) totalTX=25506145
2023/10/10 01:21:41 L1: [fex=25912056/set:0] [del:25419412/bat:0] [g/s:80/0] cached:492647 (~30790/char)
2023/10/10 01:21:41 L2: [fex=25986994/set:0] [del:25534757/bat:0] [g/s:48/0] cached:452237 (~28264/char)
2023/10/10 01:21:41 L3: [fex=25908607/set:0] [del:25416027/bat:0] [g/s:80/0] cached:492580 (~30786/char)
2023/10/10 01:21:41 BoltSpeed: 26889.48 tx/s ( did=403345 in 15.0 sec ) totalTX=25909490
Alloc: 512 MiB, TotalAlloc: 222167 MiB, Sys: 1016 MiB, NumGC: 615
2023/10/10 01:21:56 L1: [fex=26371818/set:0] [del:25856656/bat:0] [g/s:80/0] cached:515163 (~32197/char)
2023/10/10 01:21:56 L2: [fex=26449550/set:0] [del:25868681/bat:0] [g/s:48/0] cached:580869 (~36304/char)
2023/10/10 01:21:56 L3: [fex=26368308/set:0] [del:25853206/bat:0] [g/s:80/0] cached:515102 (~32193/char)
2023/10/10 01:21:56 BoltSpeed: 30647.83 tx/s ( did=459716 in 15.0 sec ) totalTX=26369206
2023/10/10 01:22:11 L1: [fex=26820115/set:0] [del:26220641/bat:0] [g/s:80/0] cached:599476 (~37467/char)
2023/10/10 01:22:11 L2: [fex=26900640/set:0] [del:26360868/bat:0] [g/s:48/0] cached:539772 (~33735/char)
2023/10/10 01:22:11 L3: [fex=26816542/set:0] [del:26217150/bat:0] [g/s:80/0] cached:599392 (~37462/char)
2023/10/10 01:22:11 BoltSpeed: 29880.95 tx/s ( did=448249 in 15.0 sec ) totalTX=26817455
Alloc: 552 MiB, TotalAlloc: 229377 MiB, Sys: 1016 MiB, NumGC: 633
2023/10/10 01:22:26 L1: [fex=27227546/set:0] [del:26694652/bat:0] [g/s:80/0] cached:532897 (~33306/char)
2023/10/10 01:22:26 L2: [fex=27310587/set:0] [del:26830820/bat:0] [g/s:48/0] cached:479767 (~29985/char)
2023/10/10 01:22:26 L3: [fex=27223921/set:0] [del:26691095/bat:0] [g/s:80/0] cached:532826 (~33301/char)
2023/10/10 01:22:26 BoltSpeed: 27161.88 tx/s ( did=407396 in 15.0 sec ) totalTX=27224851
2023/10/10 01:22:41 L1: [fex=27676201/set:0] [del:27156927/bat:0] [g/s:80/0] cached:519275 (~32454/char)
2023/10/10 01:22:41 L2: [fex=27762166/set:0] [del:27177636/bat:0] [g/s:48/0] cached:584530 (~36533/char)
2023/10/10 01:22:41 L3: [fex=27672527/set:0] [del:27153310/bat:0] [g/s:80/0] cached:519217 (~32451/char)
2023/10/10 01:22:41 BoltSpeed: 29907.96 tx/s ( did=448622 in 15.0 sec ) totalTX=27673473
Alloc: 570 MiB, TotalAlloc: 236761 MiB, Sys: 1016 MiB, NumGC: 652
2023/10/10 01:22:56 L1: [fex=28104422/set:0] [del:27618465/bat:0] [g/s:80/0] cached:485959 (~30372/char)
2023/10/10 01:22:56 L2: [fex=28193092/set:0] [del:27639429/bat:0] [g/s:48/0] cached:553663 (~34603/char)
2023/10/10 01:22:56 L3: [fex=28100698/set:0] [del:27614793/bat:0] [g/s:80/0] cached:485905 (~30369/char)
2023/10/10 01:22:56 BoltSpeed: 28545.90 tx/s ( did=428188 in 15.0 sec ) totalTX=28101661
2023/10/10 01:23:11 L1: [fex=28589187/set:0] [del:27960163/bat:0] [g/s:80/0] cached:629028 (~39314/char)
2023/10/10 01:23:11 L2: [fex=28680879/set:0] [del:28106410/bat:0] [g/s:48/0] cached:574469 (~35904/char)
2023/10/10 01:23:11 L3: [fex=28585393/set:0] [del:27956451/bat:0] [g/s:80/0] cached:628942 (~39308/char)
2023/10/10 01:23:11 BoltSpeed: 32314.21 tx/s ( did=484711 in 15.0 sec ) totalTX=28586372
Alloc: 612 MiB, TotalAlloc: 244965 MiB, Sys: 1016 MiB, NumGC: 672
2023/10/10 01:23:26 L1: [fex=29080400/set:0] [del:28460631/bat:0] [g/s:80/0] cached:619773 (~38735/char)
2023/10/10 01:23:26 L2: [fex=29175293/set:0] [del:28608075/bat:0] [g/s:48/0] cached:567218 (~35451/char)
2023/10/10 01:23:26 L3: [fex=29076534/set:0] [del:28455411/bat:0] [g/s:80/0] cached:621123 (~38820/char)
2023/10/10 01:23:26 BoltSpeed: 32743.85 tx/s ( did=491156 in 15.0 sec ) totalTX=29077528
2023/10/10 01:23:41 L1: [fex=29577541/set:0] [del:28985000/bat:0] [g/s:80/0] cached:592544 (~37034/char)
2023/10/10 01:23:41 L2: [fex=29675761/set:0] [del:28999838/bat:0] [g/s:48/0] cached:675923 (~42245/char)
2023/10/10 01:23:41 L3: [fex=29573596/set:0] [del:28974421/bat:0] [g/s:80/0] cached:599175 (~37448/char)
2023/10/10 01:23:41 BoltSpeed: 33138.17 tx/s ( did=497079 in 15.0 sec ) totalTX=29574607
2023/10/10 01:23:54 RUN test p=3 nntp-history added=0 dupes=7501606 cLock=14647703 addretry=0 retry=0 adddupes=0 cdupes=7850691 cretry1=0 cretry2=0 30000000/100000000
2023/10/10 01:23:54 RUN test p=2 nntp-history added=0 dupes=7493123 cLock=14643027 addretry=0 retry=0 adddupes=0 cdupes=7863850 cretry1=0 cretry2=0 30000000/100000000
2023/10/10 01:23:54 RUN test p=4 nntp-history added=0 dupes=7497571 cLock=14645310 addretry=0 retry=0 adddupes=0 cdupes=7857119 cretry1=0 cretry2=0 30000000/100000000
2023/10/10 01:23:54 RUN test p=1 nntp-history added=0 dupes=7507700 cLock=14655639 addretry=0 retry=0 adddupes=0 cdupes=7836661 cretry1=0 cretry2=0 30000000/100000000
Alloc: 749 MiB, TotalAlloc: 253287 MiB, Sys: 1016 MiB, NumGC: 692
2023/10/10 01:23:56 L1: [fex=30068196/set:0] [del:29520044/bat:0] [g/s:80/0] cached:548154 (~34259/char)
2023/10/10 01:23:56 L2: [fex=30169773/set:0] [del:29532117/bat:0] [g/s:48/0] cached:637656 (~39853/char)
2023/10/10 01:23:56 L3: [fex=30064180/set:0] [del:29511864/bat:0] [g/s:80/0] cached:552316 (~34519/char)
2023/10/10 01:23:56 BoltSpeed: 32697.23 tx/s ( did=490599 in 15.0 sec ) totalTX=30065206
2023/10/10 01:24:11 L1: [fex=30557747/set:0] [del:29920096/bat:0] [g/s:80/0] cached:637654 (~39853/char)
2023/10/10 01:24:11 L2: [fex=30662571/set:0] [del:30063998/bat:0] [g/s:48/0] cached:598573 (~37410/char)
2023/10/10 01:24:11 L3: [fex=30553645/set:0] [del:29904028/bat:0] [g/s:80/0] cached:649617 (~40601/char)
2023/10/10 01:24:11 BoltSpeed: 32641.77 tx/s ( did=489482 in 15.0 sec ) totalTX=30554688
Alloc: 651 MiB, TotalAlloc: 261592 MiB, Sys: 1016 MiB, NumGC: 712
2023/10/10 01:24:26 L1: [fex=31055566/set:0] [del:30441466/bat:0] [g/s:80/0] cached:614102 (~38381/char)
2023/10/10 01:24:26 L2: [fex=31163897/set:0] [del:30582666/bat:0] [g/s:48/0] cached:581231 (~36326/char)
2023/10/10 01:24:26 L3: [fex=31051385/set:0] [del:30426560/bat:0] [g/s:80/0] cached:624825 (~39051/char)
2023/10/10 01:24:26 BoltSpeed: 33183.56 tx/s ( did=497755 in 15.0 sec ) totalTX=31052443
2023/10/10 01:24:41 L1: [fex=31553527/set:0] [del:30973654/bat:0] [g/s:80/0] cached:579874 (~36242/char)
2023/10/10 01:24:41 L2: [fex=31665434/set:0] [del:30990238/bat:0] [g/s:48/0] cached:675196 (~42199/char)
2023/10/10 01:24:41 L3: [fex=31549271/set:0] [del:30959912/bat:0] [g/s:80/0] cached:589359 (~36834/char)
2023/10/10 01:24:41 BoltSpeed: 33160.27 tx/s ( did=497902 in 15.0 sec ) totalTX=31550345
Alloc: 685 MiB, TotalAlloc: 269850 MiB, Sys: 1016 MiB, NumGC: 731
2023/10/10 01:24:56 L1: [fex=32035573/set:0] [del:31419772/bat:0] [g/s:80/0] cached:615804 (~38487/char)
2023/10/10 01:24:56 L2: [fex=32150975/set:0] [del:31539971/bat:0] [g/s:48/0] cached:611004 (~38187/char)
2023/10/10 01:24:56 L3: [fex=32031246/set:0] [del:31448414/bat:0] [g/s:80/0] cached:582832 (~36427/char)
2023/10/10 01:24:56 BoltSpeed: 32165.13 tx/s ( did=481991 in 15.0 sec ) totalTX=32032336
2023/10/10 01:25:11 L1: [fex=32498838/set:0] [del:31902221/bat:0] [g/s:80/0] cached:596620 (~37288/char)
2023/10/10 01:25:11 L2: [fex=32617711/set:0] [del:32069782/bat:0] [g/s:48/0] cached:547929 (~34245/char)
2023/10/10 01:25:11 L3: [fex=32494456/set:0] [del:31897875/bat:0] [g/s:80/0] cached:596581 (~37286/char)
2023/10/10 01:25:11 BoltSpeed: 30880.77 tx/s ( did=463228 in 15.0 sec ) totalTX=32495564
Alloc: 793 MiB, TotalAlloc: 277855 MiB, Sys: 1016 MiB, NumGC: 750
2023/10/10 01:25:26 L1: [fex=32985703/set:0] [del:32404146/bat:0] [g/s:80/0] cached:581561 (~36347/char)
2023/10/10 01:25:26 L2: [fex=33108303/set:0] [del:32579624/bat:0] [g/s:48/0] cached:528679 (~33042/char)
2023/10/10 01:25:26 L3: [fex=32981278/set:0] [del:32399762/bat:0] [g/s:80/0] cached:581516 (~36344/char)
2023/10/10 01:25:26 BoltSpeed: 32406.02 tx/s ( did=486835 in 15.0 sec ) totalTX=32982399
2023/10/10 01:25:41 L1: [fex=33475349/set:0] [del:32920807/bat:0] [g/s:80/0] cached:554546 (~34659/char)
2023/10/10 01:25:41 L2: [fex=33601664/set:0] [del:32971478/bat:0] [g/s:48/0] cached:630186 (~39386/char)
2023/10/10 01:25:41 L3: [fex=33470832/set:0] [del:32916367/bat:0] [g/s:80/0] cached:554465 (~34654/char)
2023/10/10 01:25:41 BoltSpeed: 32689.35 tx/s ( did=489572 in 15.0 sec ) totalTX=33471971
Alloc: 590 MiB, TotalAlloc: 286166 MiB, Sys: 1016 MiB, NumGC: 770
2023/10/10 01:25:56 L1: [fex=33973843/set:0] [del:33317849/bat:0] [g/s:80/0] cached:655998 (~40999/char)
2023/10/10 01:25:56 L2: [fex=34103986/set:0] [del:33503134/bat:0] [g/s:48/0] cached:600852 (~37553/char)
2023/10/10 01:25:56 L3: [fex=33969266/set:0] [del:33313102/bat:0] [g/s:80/0] cached:656164 (~41010/char)
2023/10/10 01:25:56 BoltSpeed: 33230.01 tx/s ( did=498455 in 15.0 sec ) totalTX=33970426
2023/10/10 01:26:11 L1: [fex=34452274/set:0] [del:33837788/bat:0] [g/s:80/0] cached:614488 (~38405/char)
2023/10/10 01:26:11 L2: [fex=34586128/set:0] [del:34029697/bat:0] [g/s:48/0] cached:556431 (~34776/char)
2023/10/10 01:26:11 L3: [fex=34447646/set:0] [del:33833225/bat:0] [g/s:80/0] cached:614421 (~38401/char)
2023/10/10 01:26:11 BoltSpeed: 31863.08 tx/s ( did=478398 in 15.0 sec ) totalTX=34448824
Alloc: 542 MiB, TotalAlloc: 294279 MiB, Sys: 1016 MiB, NumGC: 789
2023/10/10 01:26:26 L1: [fex=34937026/set:0] [del:34353043/bat:0] [g/s:80/0] cached:583986 (~36499/char)
2023/10/10 01:26:26 L2: [fex=35074648/set:0] [del:34413184/bat:0] [g/s:48/0] cached:661464 (~41341/char)
2023/10/10 01:26:26 L3: [fex=34932301/set:0] [del:34348411/bat:0] [g/s:80/0] cached:583890 (~36493/char)
2023/10/10 01:26:26 BoltSpeed: 32341.79 tx/s ( did=484664 in 15.0 sec ) totalTX=34933488
2023/10/10 01:26:41 L1: [fex=35413601/set:0] [del:34869598/bat:0] [g/s:80/0] cached:544004 (~34000/char)
2023/10/10 01:26:41 L2: [fex=35555051/set:0] [del:34943923/bat:0] [g/s:48/0] cached:611128 (~38195/char)
2023/10/10 01:26:41 L3: [fex=35408821/set:0] [del:34864880/bat:0] [g/s:80/0] cached:543941 (~33996/char)
2023/10/10 01:26:41 BoltSpeed: 31768.79 tx/s ( did=476535 in 15.0 sec ) totalTX=35410023
2023/10/10 01:26:56 L1: [fex=35884377/set:0] [del:35241595/bat:0] [g/s:80/0] cached:642784 (~40174/char)
2023/10/10 01:26:56 L2: [fex=36029640/set:0] [del:35449359/bat:0] [g/s:48/0] cached:580281 (~36267/char)
2023/10/10 01:26:56 L3: [fex=35879523/set:0] [del:35236832/bat:0] [g/s:80/0] cached:642691 (~40168/char)
2023/10/10 01:26:56 BoltSpeed: 31381.34 tx/s ( did=470718 in 15.0 sec ) totalTX=35880741
Alloc: 814 MiB, TotalAlloc: 302257 MiB, Sys: 1016 MiB, NumGC: 807
2023/10/10 01:27:11 L1: [fex=36368736/set:0] [del:35754219/bat:0] [g/s:80/0] cached:614520 (~38407/char)
2023/10/10 01:27:11 L2: [fex=36518013/set:0] [del:35957425/bat:0] [g/s:48/0] cached:560588 (~35036/char)
2023/10/10 01:27:11 L3: [fex=36363815/set:0] [del:35749391/bat:0] [g/s:80/0] cached:614424 (~38401/char)
2023/10/10 01:27:11 BoltSpeed: 32287.32 tx/s ( did=484309 in 15.0 sec ) totalTX=36365050
Alloc: 702 MiB, TotalAlloc: 310197 MiB, Sys: 1016 MiB, NumGC: 826
2023/10/10 01:27:26 L1: [fex=36827233/set:0] [del:36268078/bat:0] [g/s:80/0] cached:559158 (~34947/char)
2023/10/10 01:27:26 L2: [fex=36980340/set:0] [del:36345428/bat:0] [g/s:48/0] cached:634912 (~39682/char)
2023/10/10 01:27:26 L3: [fex=36822247/set:0] [del:36263171/bat:0] [g/s:80/0] cached:559076 (~34942/char)
2023/10/10 01:27:26 BoltSpeed: 30561.92 tx/s ( did=458447 in 15.0 sec ) totalTX=36823497
2023/10/10 01:27:41 L1: [fex=37301887/set:0] [del:36762745/bat:0] [g/s:80/0] cached:539145 (~33696/char)
2023/10/10 01:27:41 L2: [fex=37458969/set:0] [del:36844983/bat:0] [g/s:48/0] cached:613986 (~38374/char)
2023/10/10 01:27:41 L3: [fex=37296841/set:0] [del:36756100/bat:0] [g/s:80/0] cached:540741 (~33796/char)
2023/10/10 01:27:41 BoltSpeed: 31641.84 tx/s ( did=474611 in 15.0 sec ) totalTX=37298108
Alloc: 840 MiB, TotalAlloc: 318227 MiB, Sys: 1016 MiB, NumGC: 845
2023/10/10 01:27:56 L1: [fex=37781383/set:0] [del:37144590/bat:0] [g/s:80/0] cached:636795 (~39799/char)
2023/10/10 01:27:56 L2: [fex=37942587/set:0] [del:37363234/bat:0] [g/s:48/0] cached:579353 (~36209/char)
2023/10/10 01:27:56 L3: [fex=37776267/set:0] [del:37133225/bat:0] [g/s:80/0] cached:643042 (~40190/char)
2023/10/10 01:27:56 BoltSpeed: 31962.86 tx/s ( did=479442 in 15.0 sec ) totalTX=37777550
2023/10/10 01:28:11 L1: [fex=38242925/set:0] [del:37666431/bat:0] [g/s:80/0] cached:576496 (~36031/char)
2023/10/10 01:28:11 L2: [fex=38408029/set:0] [del:37902435/bat:0] [g/s:48/0] cached:505594 (~31599/char)
2023/10/10 01:28:11 L3: [fex=38237750/set:0] [del:37655378/bat:0] [g/s:80/0] cached:582372 (~36398/char)
2023/10/10 01:28:11 BoltSpeed: 30745.70 tx/s ( did=461498 in 15.0 sec ) totalTX=38239048
Alloc: 465 MiB, TotalAlloc: 326068 MiB, Sys: 1016 MiB, NumGC: 864
2023/10/10 01:28:26 L1: [fex=38711536/set:0] [del:38165474/bat:0] [g/s:80/0] cached:546065 (~34129/char)
2023/10/10 01:28:26 L2: [fex=38880699/set:0] [del:38282213/bat:0] [g/s:48/0] cached:598486 (~37405/char)
2023/10/10 01:28:26 L3: [fex=38706303/set:0] [del:38153529/bat:0] [g/s:80/0] cached:552774 (~34548/char)
2023/10/10 01:28:26 BoltSpeed: 31259.30 tx/s ( did=468570 in 15.0 sec ) totalTX=38707618
2023/10/10 01:28:41 L1: [fex=39184638/set:0] [del:38631318/bat:0] [g/s:80/0] cached:553321 (~34582/char)
2023/10/10 01:28:41 L2: [fex=39357962/set:0] [del:38786562/bat:0] [g/s:48/0] cached:571400 (~35712/char)
2023/10/10 01:28:41 L3: [fex=39179335/set:0] [del:38641617/bat:0] [g/s:80/0] cached:537718 (~33607/char)
2023/10/10 01:28:41 BoltSpeed: 31524.66 tx/s ( did=473046 in 15.0 sec ) totalTX=39180664
2023/10/10 01:28:56 L1: [fex=39654661/set:0] [del:39047063/bat:0] [g/s:80/0] cached:607598 (~37974/char)
2023/10/10 01:28:56 L2: [fex=39832252/set:0] [del:39287163/bat:0] [g/s:48/0] cached:545089 (~34068/char)
2023/10/10 01:28:56 L3: [fex=39649291/set:0] [del:39030909/bat:0] [g/s:80/0] cached:618382 (~38648/char)
2023/10/10 01:28:56 BoltSpeed: 31342.94 tx/s ( did=469971 in 15.0 sec ) totalTX=39650635
Alloc: 784 MiB, TotalAlloc: 334003 MiB, Sys: 1016 MiB, NumGC: 882
2023/10/10 01:29:07 RUN test p=3 nntp-history added=0 dupes=10003382 cLock=19531368 addretry=0 retry=0 adddupes=0 cdupes=10465250 cretry1=0 cretry2=0 40000000/100000000
2023/10/10 01:29:07 RUN test p=1 nntp-history added=0 dupes=10009694 cLock=19538612 addretry=0 retry=0 adddupes=0 cdupes=10451694 cretry1=0 cretry2=0 40000000/100000000
2023/10/10 01:29:07 RUN test p=4 nntp-history added=0 dupes=9999624 cLock=19532235 addretry=0 retry=0 adddupes=0 cdupes=10468141 cretry1=0 cretry2=0 40000000/100000000
2023/10/10 01:29:07 RUN test p=2 nntp-history added=0 dupes=9987300 cLock=19519992 addretry=0 retry=0 adddupes=0 cdupes=10492708 cretry1=0 cretry2=0 40000000/100000000
2023/10/10 01:29:11 L1: [fex=40121841/set:0] [del:39549891/bat:0] [g/s:80/0] cached:571952 (~35747/char)
2023/10/10 01:29:11 L2: [fex=40303673/set:0] [del:39672267/bat:0] [g/s:48/0] cached:631406 (~39462/char)
2023/10/10 01:29:11 L3: [fex=40116422/set:0] [del:39532445/bat:0] [g/s:80/0] cached:583977 (~36498/char)
2023/10/10 01:29:11 BoltSpeed: 31116.15 tx/s ( did=467149 in 15.0 sec ) totalTX=40117784
Alloc: 786 MiB, TotalAlloc: 342045 MiB, Sys: 1016 MiB, NumGC: 901
2023/10/10 01:29:26 L1: [fex=40609403/set:0] [del:40043256/bat:0] [g/s:80/0] cached:566147 (~35384/char)
2023/10/10 01:29:26 L2: [fex=40795799/set:0] [del:40171082/bat:0] [g/s:48/0] cached:624717 (~39044/char)
2023/10/10 01:29:26 L3: [fex=40603901/set:0] [del:40027553/bat:0] [g/s:80/0] cached:576348 (~36021/char)
2023/10/10 01:29:26 BoltSpeed: 32527.69 tx/s ( did=487494 in 15.0 sec ) totalTX=40605278
2023/10/10 01:29:41 L1: [fex=41094476/set:0] [del:40473457/bat:0] [g/s:80/0] cached:621022 (~38813/char)
2023/10/10 01:29:41 L2: [fex=41285460/set:0] [del:40694329/bat:0] [g/s:48/0] cached:591131 (~36945/char)
2023/10/10 01:29:41 L3: [fex=41088905/set:0] [del:40491225/bat:0] [g/s:80/0] cached:597680 (~37355/char)
2023/10/10 01:29:41 BoltSpeed: 32335.15 tx/s ( did=485022 in 15.0 sec ) totalTX=41090300
Alloc: 843 MiB, TotalAlloc: 350171 MiB, Sys: 1016 MiB, NumGC: 921
2023/10/10 01:29:56 L1: [fex=41575176/set:0] [del:40955830/bat:0] [g/s:80/0] cached:619348 (~38709/char)
2023/10/10 01:29:56 L2: [fex=41770653/set:0] [del:41220379/bat:0] [g/s:48/0] cached:550274 (~34392/char)
2023/10/10 01:29:56 L3: [fex=41569537/set:0] [del:40946368/bat:0] [g/s:80/0] cached:623169 (~38948/char)
2023/10/10 01:29:56 BoltSpeed: 32042.72 tx/s ( did=480647 in 15.0 sec ) totalTX=41570947
2023/10/10 01:30:11 L1: [fex=42033151/set:0] [del:41474109/bat:0] [g/s:80/0] cached:559044 (~34940/char)
2023/10/10 01:30:11 L2: [fex=42233139/set:0] [del:41609542/bat:0] [g/s:48/0] cached:623597 (~38974/char)
2023/10/10 01:30:11 L3: [fex=42027455/set:0] [del:41466544/bat:0] [g/s:80/0] cached:560911 (~35056/char)
2023/10/10 01:30:11 BoltSpeed: 30529.16 tx/s ( did=457934 in 15.0 sec ) totalTX=42028881
Alloc: 617 MiB, TotalAlloc: 357749 MiB, Sys: 1016 MiB, NumGC: 939
2023/10/10 01:30:26 L1: [fex=42473061/set:0] [del:41971694/bat:0] [g/s:80/0] cached:501370 (~31335/char)
2023/10/10 01:30:26 L2: [fex=42677247/set:0] [del:42109541/bat:0] [g/s:48/0] cached:567706 (~35481/char)
2023/10/10 01:30:26 L3: [fex=42467309/set:0] [del:41966001/bat:0] [g/s:80/0] cached:501308 (~31331/char)
2023/10/10 01:30:26 BoltSpeed: 29324.73 tx/s ( did=439871 in 15.0 sec ) totalTX=42468752
2023/10/10 01:30:41 L1: [fex=42920647/set:0] [del:42327549/bat:0] [g/s:80/0] cached:593102 (~37068/char)
2023/10/10 01:30:41 L2: [fex=43129345/set:0] [del:42594599/bat:0] [g/s:48/0] cached:534746 (~33421/char)
2023/10/10 01:30:41 L3: [fex=42914849/set:0] [del:42352880/bat:0] [g/s:80/0] cached:561969 (~35123/char)
2023/10/10 01:30:41 BoltSpeed: 29835.42 tx/s ( did=447555 in 15.0 sec ) totalTX=42916307
Alloc: 817 MiB, TotalAlloc: 365700 MiB, Sys: 1016 MiB, NumGC: 959
2023/10/10 01:30:56 L1: [fex=43416951/set:0] [del:42796839/bat:0] [g/s:80/0] cached:620114 (~38757/char)
2023/10/10 01:30:56 L2: [fex=43630412/set:0] [del:43087523/bat:0] [g/s:48/0] cached:542889 (~33930/char)
2023/10/10 01:30:56 L3: [fex=43411087/set:0] [del:42791054/bat:0] [g/s:80/0] cached:620033 (~38752/char)
2023/10/10 01:30:56 BoltSpeed: 33084.66 tx/s ( did=496255 in 15.0 sec ) totalTX=43412562
2023/10/10 01:31:11 L1: [fex=43885425/set:0] [del:43318250/bat:0] [g/s:80/0] cached:567177 (~35448/char)
2023/10/10 01:31:11 L2: [fex=44103720/set:0] [del:43491453/bat:0] [g/s:48/0] cached:612267 (~38266/char)
2023/10/10 01:31:11 L3: [fex=43879483/set:0] [del:43312399/bat:0] [g/s:80/0] cached:567084 (~35442/char)
2023/10/10 01:31:11 BoltSpeed: 31228.34 tx/s ( did=468412 in 15.0 sec ) totalTX=43880974
Alloc: 793 MiB, TotalAlloc: 373559 MiB, Sys: 1016 MiB, NumGC: 978
2023/10/10 01:31:26 L1: [fex=44347869/set:0] [del:43823365/bat:0] [g/s:80/0] cached:524507 (~32781/char)
2023/10/10 01:31:26 L2: [fex=44570804/set:0] [del:44015383/bat:0] [g/s:48/0] cached:555421 (~34713/char)
2023/10/10 01:31:26 L3: [fex=44341874/set:0] [del:43817430/bat:0] [g/s:80/0] cached:524444 (~32777/char)
2023/10/10 01:31:26 BoltSpeed: 30826.85 tx/s ( did=462405 in 15.0 sec ) totalTX=44343379
2023/10/10 01:31:41 L1: [fex=44779420/set:0] [del:44196241/bat:0] [g/s:80/0] cached:583183 (~36448/char)
2023/10/10 01:31:41 L2: [fex=45006838/set:0] [del:44511936/bat:0] [g/s:48/0] cached:494902 (~30931/char)
2023/10/10 01:31:41 L3: [fex=44773377/set:0] [del:44190261/bat:0] [g/s:80/0] cached:583116 (~36444/char)
2023/10/10 01:31:41 BoltSpeed: 28768.06 tx/s ( did=431520 in 15.0 sec ) totalTX=44774899
2023/10/10 01:31:56 L1: [fex=45232926/set:0] [del:44637215/bat:0] [g/s:80/0] cached:595713 (~37232/char)
2023/10/10 01:31:56 L2: [fex=45465076/set:0] [del:44842269/bat:0] [g/s:48/0] cached:622807 (~38925/char)
2023/10/10 01:31:56 L3: [fex=45226824/set:0] [del:44631182/bat:0] [g/s:80/0] cached:595642 (~37227/char)
2023/10/10 01:31:56 BoltSpeed: 30230.99 tx/s ( did=453463 in 15.0 sec ) totalTX=45228362
Alloc: 746 MiB, TotalAlloc: 381014 MiB, Sys: 1016 MiB, NumGC: 997
2023/10/10 01:32:11 L1: [fex=45712496/set:0] [del:45131966/bat:0] [g/s:80/0] cached:580533 (~36283/char)
2023/10/10 01:32:11 L2: [fex=45949527/set:0] [del:45333040/bat:0] [g/s:48/0] cached:616487 (~38530/char)
2023/10/10 01:32:11 L3: [fex=45706334/set:0] [del:45125874/bat:0] [g/s:80/0] cached:580460 (~36278/char)
2023/10/10 01:32:11 BoltSpeed: 31968.42 tx/s ( did=479526 in 15.0 sec ) totalTX=45707888
2023/10/10 01:32:26 L1: [fex=46211529/set:0] [del:45645970/bat:0] [g/s:80/0] cached:565560 (~35347/char)
2023/10/10 01:32:26 L2: [fex=46454044/set:0] [del:45855126/bat:0] [g/s:48/0] cached:598918 (~37432/char)
2023/10/10 01:32:26 L3: [fex=46205303/set:0] [del:45641411/bat:0] [g/s:80/0] cached:563892 (~35243/char)
2023/10/10 01:32:26 BoltSpeed: 33248.73 tx/s ( did=498984 in 15.0 sec ) totalTX=46206872
Alloc: 786 MiB, TotalAlloc: 389266 MiB, Sys: 1016 MiB, NumGC: 1018
2023/10/10 01:32:41 L1: [fex=46713025/set:0] [del:46048874/bat:0] [g/s:80/0] cached:664153 (~41509/char)
2023/10/10 01:32:41 L2: [fex=46960934/set:0] [del:46384349/bat:0] [g/s:48/0] cached:576585 (~36036/char)
2023/10/10 01:32:41 L3: [fex=46706726/set:0] [del:46035109/bat:0] [g/s:80/0] cached:671617 (~41976/char)
2023/10/10 01:32:41 BoltSpeed: 33446.10 tx/s ( did=501439 in 15.0 sec ) totalTX=46708311
Alloc: 707 MiB, TotalAlloc: 397652 MiB, Sys: 1016 MiB, NumGC: 1039
2023/10/10 01:32:56 L1: [fex=47207101/set:0] [del:46587948/bat:0] [g/s:80/0] cached:619155 (~38697/char)
2023/10/10 01:32:56 L2: [fex=47460359/set:0] [del:46784080/bat:0] [g/s:48/0] cached:676279 (~42267/char)
2023/10/10 01:32:56 L3: [fex=47200722/set:0] [del:46572371/bat:0] [g/s:80/0] cached:628351 (~39271/char)
2023/10/10 01:32:56 BoltSpeed: 32934.01 tx/s ( did=494014 in 15.0 sec ) totalTX=47202325
2023/10/10 01:33:11 L1: [fex=47696448/set:0] [del:47122767/bat:0] [g/s:80/0] cached:573682 (~35855/char)
2023/10/10 01:33:11 L2: [fex=47955016/set:0] [del:47319641/bat:0] [g/s:48/0] cached:635375 (~39710/char)
2023/10/10 01:33:11 L3: [fex=47689984/set:0] [del:47104658/bat:0] [g/s:80/0] cached:585326 (~36582/char)
2023/10/10 01:33:11 BoltSpeed: 32618.68 tx/s ( did=489277 in 15.0 sec ) totalTX=47691602
Alloc: 858 MiB, TotalAlloc: 405957 MiB, Sys: 1016 MiB, NumGC: 1058
2023/10/10 01:33:26 L1: [fex=48190999/set:0] [del:47600138/bat:0] [g/s:80/0] cached:590861 (~36928/char)
2023/10/10 01:33:26 L2: [fex=48455052/set:0] [del:47869678/bat:0] [g/s:48/0] cached:585374 (~36585/char)
2023/10/10 01:33:26 L3: [fex=48184463/set:0] [del:47611706/bat:0] [g/s:80/0] cached:572757 (~35797/char)
2023/10/10 01:33:26 BoltSpeed: 32921.94 tx/s ( did=494493 in 15.0 sec ) totalTX=48186095
2023/10/10 01:33:41 L1: [fex=48676913/set:0] [del:48049437/bat:0] [g/s:80/0] cached:627479 (~39217/char)
2023/10/10 01:33:41 L2: [fex=48946553/set:0] [del:48414130/bat:0] [g/s:48/0] cached:532423 (~33276/char)
2023/10/10 01:33:41 L3: [fex=48670312/set:0] [del:48033041/bat:0] [g/s:80/0] cached:637271 (~39829/char)
2023/10/10 01:33:41 BoltSpeed: 32434.89 tx/s ( did=485868 in 15.0 sec ) totalTX=48671963
Alloc: 630 MiB, TotalAlloc: 414227 MiB, Sys: 1016 MiB, NumGC: 1079
2023/10/10 01:33:56 L1: [fex=49172784/set:0] [del:48565579/bat:0] [g/s:80/0] cached:607208 (~37950/char)
2023/10/10 01:33:56 L2: [fex=49447931/set:0] [del:48810593/bat:0] [g/s:48/0] cached:637338 (~39833/char)
2023/10/10 01:33:56 L3: [fex=49166106/set:0] [del:48548198/bat:0] [g/s:80/0] cached:617908 (~38619/char)
2023/10/10 01:33:56 BoltSpeed: 33054.19 tx/s ( did=495811 in 15.0 sec ) totalTX=49167774
2023/10/10 01:34:11 L1: [fex=49664957/set:0] [del:49094116/bat:0] [g/s:80/0] cached:570845 (~35677/char)
2023/10/10 01:34:11 L2: [fex=49945646/set:0] [del:49343772/bat:0] [g/s:48/0] cached:601874 (~37617/char)
2023/10/10 01:34:11 L3: [fex=49658209/set:0] [del:49076149/bat:0] [g/s:80/0] cached:582060 (~36378/char)
2023/10/10 01:34:11 BoltSpeed: 32807.76 tx/s ( did=492117 in 15.0 sec ) totalTX=49659891
2023/10/10 01:34:21 RUN test p=4 nntp-history added=0 dupes=12500412 cLock=24431882 addretry=0 retry=0 adddupes=0 cdupes=13067706 cretry1=0 cretry2=0 50000000/100000000
2023/10/10 01:34:21 RUN test p=3 nntp-history added=0 dupes=12505687 cLock=24431133 addretry=0 retry=0 adddupes=0 cdupes=13063180 cretry1=0 cretry2=0 50000000/100000000
2023/10/10 01:34:21 RUN test p=2 nntp-history added=0 dupes=12485753 cLock=24416965 addretry=0 retry=0 adddupes=0 cdupes=13097282 cretry1=0 cretry2=0 50000000/100000000
2023/10/10 01:34:21 RUN test p=1 nntp-history added=0 dupes=12508148 cLock=24433908 addretry=0 retry=0 adddupes=0 cdupes=13057944 cretry1=0 cretry2=0 50000000/100000000
Alloc: 471 MiB, TotalAlloc: 422549 MiB, Sys: 1016 MiB, NumGC: 1099
2023/10/10 01:34:26 L1: [fex=50159268/set:0] [del:49525137/bat:0] [g/s:80/0] cached:634134 (~39633/char)
2023/10/10 01:34:26 L2: [fex=50445707/set:0] [del:49876709/bat:0] [g/s:48/0] cached:568998 (~35562/char)
2023/10/10 01:34:26 L3: [fex=50152441/set:0] [del:49549761/bat:0] [g/s:80/0] cached:602680 (~37667/char)
2023/10/10 01:34:26 BoltSpeed: 32949.72 tx/s ( did=494247 in 15.0 sec ) totalTX=50154138
2023/10/10 01:34:41 L1: [fex=50614745/set:0] [del:50022996/bat:0] [g/s:80/0] cached:591752 (~36984/char)
2023/10/10 01:34:41 L2: [fex=50906448/set:0] [del:50279787/bat:0] [g/s:48/0] cached:626661 (~39166/char)
2023/10/10 01:34:41 L3: [fex=50607855/set:0] [del:50008803/bat:0] [g/s:80/0] cached:599052 (~37440/char)
2023/10/10 01:34:41 BoltSpeed: 30362.13 tx/s ( did=455431 in 15.0 sec ) totalTX=50609569
Alloc: 568 MiB, TotalAlloc: 430420 MiB, Sys: 1016 MiB, NumGC: 1118
2023/10/10 01:34:56 L1: [fex=51092291/set:0] [del:50511138/bat:0] [g/s:80/0] cached:581155 (~36322/char)
2023/10/10 01:34:56 L2: [fex=51389636/set:0] [del:50783299/bat:0] [g/s:48/0] cached:606337 (~37896/char)
2023/10/10 01:34:56 L3: [fex=51085346/set:0] [del:50502972/bat:0] [g/s:80/0] cached:582374 (~36398/char)
2023/10/10 01:34:56 BoltSpeed: 31833.84 tx/s ( did=477508 in 15.0 sec ) totalTX=51087077
2023/10/10 01:35:11 L1: [fex=51598752/set:0] [del:51022301/bat:0] [g/s:80/0] cached:576455 (~36028/char)
2023/10/10 01:35:11 L2: [fex=51902134/set:0] [del:51279528/bat:0] [g/s:48/0] cached:622606 (~38912/char)
2023/10/10 01:35:11 L3: [fex=51591739/set:0] [del:51010530/bat:0] [g/s:80/0] cached:581209 (~36325/char)
2023/10/10 01:35:11 BoltSpeed: 33760.55 tx/s ( did=506408 in 15.0 sec ) totalTX=51593485
2023/10/10 01:35:26 L1: [fex=52086120/set:0] [del:51438739/bat:0] [g/s:80/0] cached:647382 (~40461/char)
2023/10/10 01:35:26 L2: [fex=52395400/set:0] [del:51829920/bat:0] [g/s:48/0] cached:565480 (~35342/char)
2023/10/10 01:35:26 L3: [fex=52079036/set:0] [del:51464743/bat:0] [g/s:80/0] cached:614293 (~38393/char)
2023/10/10 01:35:26 BoltSpeed: 32487.39 tx/s ( did=487311 in 15.0 sec ) totalTX=52080796
Alloc: 851 MiB, TotalAlloc: 438796 MiB, Sys: 1016 MiB, NumGC: 1137
2023/10/10 01:35:41 L1: [fex=52550339/set:0] [del:51951537/bat:0] [g/s:80/0] cached:598803 (~37425/char)
2023/10/10 01:35:41 L2: [fex=52865168/set:0] [del:52229642/bat:0] [g/s:48/0] cached:635526 (~39720/char)
2023/10/10 01:35:41 L3: [fex=52543197/set:0] [del:51944473/bat:0] [g/s:80/0] cached:598724 (~37420/char)
2023/10/10 01:35:41 BoltSpeed: 30945.23 tx/s ( did=464178 in 15.0 sec ) totalTX=52544974
Alloc: 788 MiB, TotalAlloc: 446651 MiB, Sys: 1016 MiB, NumGC: 1156
2023/10/10 01:35:56 L1: [fex=53017095/set:0] [del:52455641/bat:0] [g/s:80/0] cached:561457 (~35091/char)
2023/10/10 01:35:56 L2: [fex=53337561/set:0] [del:52750133/bat:0] [g/s:48/0] cached:587428 (~36714/char)
2023/10/10 01:35:56 L3: [fex=53009891/set:0] [del:52448514/bat:0] [g/s:80/0] cached:561377 (~35086/char)
2023/10/10 01:35:56 BoltSpeed: 31110.67 tx/s ( did=466711 in 15.0 sec ) totalTX=53011685
2023/10/10 01:36:11 L1: [fex=53515295/set:0] [del:52949906/bat:0] [g/s:80/0] cached:565391 (~35336/char)
2023/10/10 01:36:11 L2: [fex=53841829/set:0] [del:53235997/bat:0] [g/s:48/0] cached:605832 (~37864/char)
2023/10/10 01:36:11 L3: [fex=53508033/set:0] [del:52942707/bat:0] [g/s:80/0] cached:565326 (~35332/char)
2023/10/10 01:36:11 BoltSpeed: 33212.43 tx/s ( did=498158 in 15.0 sec ) totalTX=53509843
Alloc: 720 MiB, TotalAlloc: 455092 MiB, Sys: 1016 MiB, NumGC: 1176
2023/10/10 01:36:26 L1: [fex=54019739/set:0] [del:53349474/bat:0] [g/s:80/0] cached:670268 (~41891/char)
2023/10/10 01:36:26 L2: [fex=54352481/set:0] [del:53776120/bat:0] [g/s:48/0] cached:576361 (~36022/char)
2023/10/10 01:36:26 L3: [fex=54012389/set:0] [del:53342211/bat:0] [g/s:80/0] cached:670178 (~41886/char)
2023/10/10 01:36:26 BoltSpeed: 33626.57 tx/s ( did=504373 in 15.0 sec ) totalTX=54014216
2023/10/10 01:36:41 L1: [fex=54510417/set:0] [del:53879417/bat:0] [g/s:80/0] cached:631002 (~39437/char)
2023/10/10 01:36:41 L2: [fex=54849333/set:0] [del:54192695/bat:0] [g/s:48/0] cached:656638 (~41039/char)
2023/10/10 01:36:41 L3: [fex=54502994/set:0] [del:53874093/bat:0] [g/s:80/0] cached:628901 (~39306/char)
2023/10/10 01:36:41 BoltSpeed: 32685.29 tx/s ( did=490620 in 15.0 sec ) totalTX=54504836
Alloc: 724 MiB, TotalAlloc: 463479 MiB, Sys: 1024 MiB, NumGC: 1195
2023/10/10 01:36:56 L1: [fex=55013260/set:0] [del:54413485/bat:0] [g/s:80/0] cached:599778 (~37486/char)
2023/10/10 01:36:56 L2: [fex=55358693/set:0] [del:54731007/bat:0] [g/s:48/0] cached:627686 (~39230/char)
2023/10/10 01:36:56 L3: [fex=55005761/set:0] [del:54401824/bat:0] [g/s:80/0] cached:603937 (~37746/char)
2023/10/10 01:36:56 BoltSpeed: 33542.32 tx/s ( did=502784 in 15.0 sec ) totalTX=55007620
2023/10/10 01:37:11 L1: [fex=55503797/set:0] [del:54954387/bat:0] [g/s:80/0] cached:549412 (~34338/char)
2023/10/10 01:37:11 L2: [fex=55855421/set:0] [del:55283393/bat:0] [g/s:48/0] cached:572028 (~35751/char)
2023/10/10 01:37:11 L3: [fex=55496234/set:0] [del:54935363/bat:0] [g/s:80/0] cached:560871 (~35054/char)
2023/10/10 01:37:11 BoltSpeed: 32699.30 tx/s ( did=490489 in 15.0 sec ) totalTX=55498109
Alloc: 526 MiB, TotalAlloc: 471836 MiB, Sys: 1024 MiB, NumGC: 1215
2023/10/10 01:37:26 L1: [fex=56004896/set:0] [del:55354909/bat:0] [g/s:80/0] cached:649991 (~40624/char)
2023/10/10 01:37:26 L2: [fex=56362928/set:0] [del:55739954/bat:0] [g/s:48/0] cached:622974 (~38935/char)
2023/10/10 01:37:26 L3: [fex=55997269/set:0] [del:55336801/bat:0] [g/s:80/0] cached:660468 (~41279/char)
2023/10/10 01:37:26 BoltSpeed: 33403.44 tx/s ( did=501051 in 15.0 sec ) totalTX=55999160
2023/10/10 01:37:41 L1: [fex=56504299/set:0] [del:55884652/bat:0] [g/s:80/0] cached:619649 (~38728/char)
2023/10/10 01:37:41 L2: [fex=56868908/set:0] [del:56225927/bat:0] [g/s:48/0] cached:642981 (~40186/char)
2023/10/10 01:37:41 L3: [fex=56496606/set:0] [del:55870345/bat:0] [g/s:80/0] cached:626261 (~39141/char)
2023/10/10 01:37:41 BoltSpeed: 33289.92 tx/s ( did=499352 in 15.0 sec ) totalTX=56498512
Alloc: 756 MiB, TotalAlloc: 480246 MiB, Sys: 1024 MiB, NumGC: 1234
2023/10/10 01:37:56 L1: [fex=57001241/set:0] [del:56420598/bat:0] [g/s:80/0] cached:580645 (~36290/char)
2023/10/10 01:37:56 L2: [fex=57372312/set:0] [del:56765039/bat:0] [g/s:48/0] cached:607273 (~37954/char)
2023/10/10 01:37:56 L3: [fex=56993483/set:0] [del:56403449/bat:0] [g/s:80/0] cached:590034 (~36877/char)
2023/10/10 01:37:56 BoltSpeed: 33125.67 tx/s ( did=496892 in 15.0 sec ) totalTX=56995404
2023/10/10 01:38:11 L1: [fex=57499205/set:0] [del:56886379/bat:0] [g/s:80/0] cached:612827 (~38301/char)
2023/10/10 01:38:11 L2: [fex=57876803/set:0] [del:57305555/bat:0] [g/s:48/0] cached:571248 (~35703/char)
2023/10/10 01:38:11 L3: [fex=57491376/set:0] [del:56914324/bat:0] [g/s:80/0] cached:577052 (~36065/char)
2023/10/10 01:38:11 BoltSpeed: 33194.54 tx/s ( did=497909 in 15.0 sec ) totalTX=57493313
2023/10/10 01:38:26 L1: [fex=57986509/set:0] [del:57349667/bat:0] [g/s:80/0] cached:636843 (~39802/char)
2023/10/10 01:38:26 L2: [fex=58370669/set:0] [del:57707568/bat:0] [g/s:48/0] cached:663101 (~41443/char)
2023/10/10 01:38:26 L3: [fex=57978616/set:0] [del:57336644/bat:0] [g/s:80/0] cached:641972 (~40123/char)
2023/10/10 01:38:26 BoltSpeed: 32483.58 tx/s ( did=487255 in 15.0 sec ) totalTX=57980568
Alloc: 912 MiB, TotalAlloc: 488550 MiB, Sys: 1033 MiB, NumGC: 1253
2023/10/10 01:38:41 L1: [fex=58477651/set:0] [del:57874583/bat:0] [g/s:80/0] cached:603070 (~37691/char)
2023/10/10 01:38:41 L2: [fex=58868449/set:0] [del:58234597/bat:0] [g/s:48/0] cached:633852 (~39615/char)
2023/10/10 01:38:41 L3: [fex=58469692/set:0] [del:57856518/bat:0] [g/s:80/0] cached:613174 (~38323/char)
2023/10/10 01:38:41 BoltSpeed: 32739.67 tx/s ( did=491094 in 15.0 sec ) totalTX=58471662
Alloc: 611 MiB, TotalAlloc: 496817 MiB, Sys: 1033 MiB, NumGC: 1273
2023/10/10 01:38:56 L1: [fex=58966061/set:0] [del:58399053/bat:0] [g/s:80/0] cached:567010 (~35438/char)
2023/10/10 01:38:56 L2: [fex=59363380/set:0] [del:58765025/bat:0] [g/s:48/0] cached:598355 (~37397/char)
2023/10/10 01:38:56 L3: [fex=58958044/set:0] [del:58379409/bat:0] [g/s:80/0] cached:578635 (~36164/char)
2023/10/10 01:38:56 BoltSpeed: 32557.78 tx/s ( did=488367 in 15.0 sec ) totalTX=58960029
2023/10/10 01:39:11 L1: [fex=59461790/set:0] [del:58840650/bat:0] [g/s:80/0] cached:621142 (~38821/char)
2023/10/10 01:39:11 L2: [fex=59866003/set:0] [del:59290416/bat:0] [g/s:48/0] cached:575587 (~35974/char)
2023/10/10 01:39:11 L3: [fex=59453698/set:0] [del:58844495/bat:0] [g/s:80/0] cached:609203 (~38075/char)
2023/10/10 01:39:11 BoltSpeed: 33044.68 tx/s ( did=495671 in 15.0 sec ) totalTX=59455700
Alloc: 660 MiB, TotalAlloc: 505152 MiB, Sys: 1033 MiB, NumGC: 1292
2023/10/10 01:39:26 L1: [fex=59956043/set:0] [del:59324532/bat:0] [g/s:80/0] cached:631513 (~39469/char)
2023/10/10 01:39:26 L2: [fex=60366934/set:0] [del:59697346/bat:0] [g/s:48/0] cached:669588 (~41849/char)
2023/10/10 01:39:26 L3: [fex=59947886/set:0] [del:59304758/bat:0] [g/s:80/0] cached:643128 (~40195/char)
2023/10/10 01:39:26 BoltSpeed: 32946.91 tx/s ( did=494203 in 15.0 sec ) totalTX=59949903
2023/10/10 01:39:27 RUN test p=4 nntp-history added=0 dupes=15001661 cLock=29315495 addretry=0 retry=0 adddupes=0 cdupes=15682844 cretry1=0 cretry2=0 60000000/100000000
2023/10/10 01:39:27 RUN test p=2 nntp-history added=0 dupes=14988264 cLock=29302115 addretry=0 retry=0 adddupes=0 cdupes=15709621 cretry1=0 cretry2=0 60000000/100000000
2023/10/10 01:39:27 RUN test p=3 nntp-history added=0 dupes=15006332 cLock=29315540 addretry=0 retry=0 adddupes=0 cdupes=15678128 cretry1=0 cretry2=0 60000000/100000000
2023/10/10 01:39:27 RUN test p=1 nntp-history added=0 dupes=15003743 cLock=29311595 addretry=0 retry=0 adddupes=0 cdupes=15684662 cretry1=0 cretry2=0 60000000/100000000
2023/10/10 01:39:41 L1: [fex=60447408/set:0] [del:59844933/bat:0] [g/s:80/0] cached:602478 (~37654/char)
2023/10/10 01:39:41 L2: [fex=60865046/set:0] [del:60225487/bat:0] [g/s:48/0] cached:639559 (~39972/char)
2023/10/10 01:39:41 L3: [fex=60439181/set:0] [del:59827204/bat:0] [g/s:80/0] cached:611977 (~38248/char)
2023/10/10 01:39:41 BoltSpeed: 32754.09 tx/s ( did=491312 in 15.0 sec ) totalTX=60441215
Alloc: 689 MiB, TotalAlloc: 513502 MiB, Sys: 1033 MiB, NumGC: 1311
2023/10/10 01:39:56 L1: [fex=60945140/set:0] [del:60380243/bat:0] [g/s:80/0] cached:564900 (~35306/char)
2023/10/10 01:39:56 L2: [fex=61369876/set:0] [del:60760841/bat:0] [g/s:48/0] cached:609035 (~38064/char)
2023/10/10 01:39:56 L3: [fex=60936842/set:0] [del:60362584/bat:0] [g/s:80/0] cached:574258 (~35891/char)
2023/10/10 01:39:56 BoltSpeed: 33178.56 tx/s ( did=497677 in 15.0 sec ) totalTX=60938892
2023/10/10 01:40:11 L1: [fex=61434644/set:0] [del:60778195/bat:0] [g/s:80/0] cached:656453 (~41028/char)
2023/10/10 01:40:11 L2: [fex=61866314/set:0] [del:61301909/bat:0] [g/s:48/0] cached:564405 (~35275/char)
2023/10/10 01:40:11 L3: [fex=61426262/set:0] [del:60814615/bat:0] [g/s:80/0] cached:611647 (~38227/char)
2023/10/10 01:40:11 BoltSpeed: 32617.94 tx/s ( did=489434 in 15.0 sec ) totalTX=61428326
Alloc: 568 MiB, TotalAlloc: 521789 MiB, Sys: 1033 MiB, NumGC: 1330
2023/10/10 01:40:26 L1: [fex=61928761/set:0] [del:61294439/bat:0] [g/s:80/0] cached:634326 (~39645/char)
2023/10/10 01:40:26 L2: [fex=62367452/set:0] [del:61711406/bat:0] [g/s:48/0] cached:656046 (~41002/char)
2023/10/10 01:40:26 L3: [fex=61920311/set:0] [del:61284322/bat:0] [g/s:80/0] cached:635989 (~39749/char)
2023/10/10 01:40:26 BoltSpeed: 32948.77 tx/s ( did=494067 in 15.0 sec ) totalTX=61922393
2023/10/10 01:40:41 L1: [fex=62426785/set:0] [del:61827063/bat:0] [g/s:80/0] cached:599725 (~37482/char)
2023/10/10 01:40:41 L2: [fex=62872654/set:0] [del:62266256/bat:0] [g/s:48/0] cached:606398 (~37899/char)
2023/10/10 01:40:41 L3: [fex=62418244/set:0] [del:61816600/bat:0] [g/s:80/0] cached:601644 (~37602/char)
2023/10/10 01:40:41 BoltSpeed: 33196.80 tx/s ( did=497949 in 15.0 sec ) totalTX=62420342
Alloc: 918 MiB, TotalAlloc: 530070 MiB, Sys: 1041 MiB, NumGC: 1348
2023/10/10 01:40:56 L1: [fex=62909643/set:0] [del:62354512/bat:0] [g/s:80/0] cached:555133 (~34695/char)
2023/10/10 01:40:56 L2: [fex=63362502/set:0] [del:62799344/bat:0] [g/s:48/0] cached:563158 (~35197/char)
2023/10/10 01:40:56 L3: [fex=62901041/set:0] [del:62341973/bat:0] [g/s:80/0] cached:559068 (~34941/char)
2023/10/10 01:40:56 BoltSpeed: 32187.29 tx/s ( did=482814 in 15.0 sec ) totalTX=62903156
2023/10/10 01:41:11 L1: [fex=63395520/set:0] [del:62759215/bat:0] [g/s:80/0] cached:636308 (~39769/char)
2023/10/10 01:41:11 L2: [fex=63855433/set:0] [del:63201890/bat:0] [g/s:48/0] cached:653543 (~40846/char)
2023/10/10 01:41:11 L3: [fex=63386827/set:0] [del:62756485/bat:0] [g/s:80/0] cached:630342 (~39396/char)
2023/10/10 01:41:11 BoltSpeed: 32386.82 tx/s ( did=485801 in 15.0 sec ) totalTX=63388957
Alloc: 763 MiB, TotalAlloc: 537894 MiB, Sys: 1041 MiB, NumGC: 1366
2023/10/10 01:41:26 L1: [fex=63838042/set:0] [del:63275148/bat:0] [g/s:80/0] cached:562898 (~35181/char)
2023/10/10 01:41:26 L2: [fex=64304448/set:0] [del:63733308/bat:0] [g/s:48/0] cached:571140 (~35696/char)
2023/10/10 01:41:26 L3: [fex=63829304/set:0] [del:63266477/bat:0] [g/s:80/0] cached:562827 (~35176/char)
2023/10/10 01:41:26 BoltSpeed: 29499.78 tx/s ( did=442495 in 15.0 sec ) totalTX=63831452
2023/10/10 01:41:41 L1: [fex=64319528/set:0] [del:63744915/bat:0] [g/s:80/0] cached:574616 (~35913/char)
2023/10/10 01:41:41 L2: [fex=64793002/set:0] [del:64209919/bat:0] [g/s:48/0] cached:583083 (~36442/char)
2023/10/10 01:41:41 L3: [fex=64310728/set:0] [del:63736181/bat:0] [g/s:80/0] cached:574547 (~35909/char)
2023/10/10 01:41:41 BoltSpeed: 32095.94 tx/s ( did=481438 in 15.0 sec ) totalTX=64312890
Alloc: 485 MiB, TotalAlloc: 545674 MiB, Sys: 1041 MiB, NumGC: 1385
2023/10/10 01:41:56 L1: [fex=64759048/set:0] [del:64251335/bat:0] [g/s:80/0] cached:507717 (~31732/char)
2023/10/10 01:41:56 L2: [fex=65238928/set:0] [del:64723785/bat:0] [g/s:48/0] cached:515143 (~32196/char)
2023/10/10 01:41:56 L3: [fex=64750184/set:0] [del:64242539/bat:0] [g/s:80/0] cached:507645 (~31727/char)
2023/10/10 01:41:56 BoltSpeed: 29298.19 tx/s ( did=439474 in 15.0 sec ) totalTX=64752364
2023/10/10 01:42:11 L1: [fex=65261331/set:0] [del:64607444/bat:0] [g/s:80/0] cached:653889 (~40868/char)
2023/10/10 01:42:11 L2: [fex=65748670/set:0] [del:65085126/bat:0] [g/s:48/0] cached:663544 (~41471/char)
2023/10/10 01:42:11 L3: [fex=65252392/set:0] [del:64606246/bat:0] [g/s:80/0] cached:646146 (~40384/char)
2023/10/10 01:42:11 BoltSpeed: 33481.43 tx/s ( did=502222 in 15.0 sec ) totalTX=65254586
Alloc: 726 MiB, TotalAlloc: 553681 MiB, Sys: 1041 MiB, NumGC: 1403
2023/10/10 01:42:26 L1: [fex=65710100/set:0] [del:65126605/bat:0] [g/s:80/0] cached:583498 (~36468/char)
2023/10/10 01:42:26 L2: [fex=66204302/set:0] [del:65611898/bat:0] [g/s:48/0] cached:592404 (~37025/char)
2023/10/10 01:42:26 L3: [fex=65701096/set:0] [del:65117687/bat:0] [g/s:80/0] cached:583409 (~36463/char)
2023/10/10 01:42:26 BoltSpeed: 29914.77 tx/s ( did=448721 in 15.0 sec ) totalTX=65703307
2023/10/10 01:42:41 L1: [fex=66193729/set:0] [del:65625027/bat:0] [g/s:80/0] cached:568704 (~35544/char)
2023/10/10 01:42:41 L2: [fex=66695265/set:0] [del:66111023/bat:0] [g/s:48/0] cached:584242 (~36515/char)
2023/10/10 01:42:41 L3: [fex=66184671/set:0] [del:65611864/bat:0] [g/s:80/0] cached:572807 (~35800/char)
2023/10/10 01:42:41 BoltSpeed: 32233.44 tx/s ( did=483591 in 15.0 sec ) totalTX=66186898
Alloc: 724 MiB, TotalAlloc: 561588 MiB, Sys: 1045 MiB, NumGC: 1421
2023/10/10 01:42:56 L1: [fex=66645040/set:0] [del:66145020/bat:0] [g/s:80/0] cached:500023 (~31251/char)
2023/10/10 01:42:56 L2: [fex=67153514/set:0] [del:66636604/bat:0] [g/s:48/0] cached:516910 (~32306/char)
2023/10/10 01:42:56 L3: [fex=66635910/set:0] [del:66130298/bat:0] [g/s:80/0] cached:505612 (~31600/char)
2023/10/10 01:42:56 BoltSpeed: 30089.20 tx/s ( did=451254 in 15.0 sec ) totalTX=66638152
2023/10/10 01:43:11 L1: [fex=67162892/set:0] [del:66512084/bat:0] [g/s:80/0] cached:650811 (~40675/char)
2023/10/10 01:43:11 L2: [fex=67679334/set:0] [del:67014828/bat:0] [g/s:48/0] cached:664506 (~41531/char)
2023/10/10 01:43:11 L3: [fex=67153705/set:0] [del:66500053/bat:0] [g/s:80/0] cached:653652 (~40853/char)
2023/10/10 01:43:11 BoltSpeed: 34518.37 tx/s ( did=517810 in 15.0 sec ) totalTX=67155962
Alloc: 811 MiB, TotalAlloc: 569863 MiB, Sys: 1049 MiB, NumGC: 1440
2023/10/10 01:43:26 L1: [fex=67627543/set:0] [del:67042323/bat:0] [g/s:80/0] cached:585222 (~36576/char)
2023/10/10 01:43:26 L2: [fex=68151162/set:0] [del:67554686/bat:0] [g/s:48/0] cached:596476 (~37279/char)
2023/10/10 01:43:26 L3: [fex=67618275/set:0] [del:67021737/bat:0] [g/s:80/0] cached:596538 (~37283/char)
2023/10/10 01:43:26 BoltSpeed: 30973.46 tx/s ( did=464587 in 15.0 sec ) totalTX=67620549
2023/10/10 01:43:41 L1: [fex=68113822/set:0] [del:67553265/bat:0] [g/s:80/0] cached:560561 (~35035/char)
2023/10/10 01:43:41 L2: [fex=68645095/set:0] [del:68077675/bat:0] [g/s:48/0] cached:567420 (~35463/char)
2023/10/10 01:43:41 L3: [fex=68104477/set:0] [del:67530395/bat:0] [g/s:80/0] cached:574082 (~35880/char)
2023/10/10 01:43:41 BoltSpeed: 32415.67 tx/s ( did=486218 in 15.0 sec ) totalTX=68106767
2023/10/10 01:43:56 L1: [fex=68624004/set:0] [del:68000263/bat:0] [g/s:80/0] cached:623742 (~38983/char)
2023/10/10 01:43:56 L2: [fex=69163246/set:0] [del:68531947/bat:0] [g/s:48/0] cached:631299 (~39456/char)
2023/10/10 01:43:56 L3: [fex=68614586/set:0] [del:68019749/bat:0] [g/s:80/0] cached:594837 (~37177/char)
2023/10/10 01:43:56 BoltSpeed: 34008.26 tx/s ( did=510124 in 15.0 sec ) totalTX=68616891
Alloc: 822 MiB, TotalAlloc: 578271 MiB, Sys: 1049 MiB, NumGC: 1459
2023/10/10 01:44:11 L1: [fex=69136392/set:0] [del:68476262/bat:0] [g/s:80/0] cached:660131 (~41258/char)
2023/10/10 01:44:11 L2: [fex=69683816/set:0] [del:69015093/bat:0] [g/s:48/0] cached:668723 (~41795/char)
2023/10/10 01:44:11 L3: [fex=69126908/set:0] [del:68457393/bat:0] [g/s:80/0] cached:669515 (~41844/char)
2023/10/10 01:44:11 BoltSpeed: 34155.92 tx/s ( did=512340 in 15.0 sec ) totalTX=69129231
Alloc: 523 MiB, TotalAlloc: 586897 MiB, Sys: 1049 MiB, NumGC: 1479
2023/10/10 01:44:26 L1: [fex=69647101/set:0] [del:69027036/bat:0] [g/s:80/0] cached:620068 (~38754/char)
2023/10/10 01:44:26 L2: [fex=70202810/set:0] [del:69575040/bat:0] [g/s:48/0] cached:627770 (~39235/char)
2023/10/10 01:44:26 L3: [fex=69637554/set:0] [del:69010493/bat:0] [g/s:80/0] cached:627061 (~39191/char)
2023/10/10 01:44:26 BoltSpeed: 34044.15 tx/s ( did=510662 in 15.0 sec ) totalTX=69639893
2023/10/10 01:44:36 RUN test p=4 nntp-history added=0 dupes=17499639 cLock=34155360 addretry=0 retry=0 adddupes=0 cdupes=18345001 cretry1=0 cretry2=0 70000000/100000000
2023/10/10 01:44:36 RUN test p=1 nntp-history added=0 dupes=17503793 cLock=34151247 addretry=0 retry=0 adddupes=0 cdupes=18344960 cretry1=0 cretry2=0 70000000/100000000
2023/10/10 01:44:36 RUN test p=3 nntp-history added=0 dupes=17510543 cLock=34159887 addretry=0 retry=0 adddupes=0 cdupes=18329570 cretry1=0 cretry2=0 70000000/100000000
2023/10/10 01:44:36 RUN test p=2 nntp-history added=0 dupes=17486025 cLock=34141661 addretry=0 retry=0 adddupes=0 cdupes=18372314 cretry1=0 cretry2=0 70000000/100000000
2023/10/10 01:44:41 L1: [fex=70172850/set:0] [del:69578457/bat:0] [g/s:80/0] cached:594395 (~37149/char)
2023/10/10 01:44:41 L2: [fex=70737088/set:0] [del:70135718/bat:0] [g/s:48/0] cached:601370 (~37585/char)
2023/10/10 01:44:41 L3: [fex=70163220/set:0] [del:69560956/bat:0] [g/s:80/0] cached:602264 (~37641/char)
2023/10/10 01:44:41 BoltSpeed: 35045.36 tx/s ( did=525682 in 15.0 sec ) totalTX=70165575
Alloc: 723 MiB, TotalAlloc: 595675 MiB, Sys: 1049 MiB, NumGC: 1498
2023/10/10 01:44:56 L1: [fex=70687696/set:0] [del:70006373/bat:0] [g/s:80/0] cached:681325 (~42582/char)
2023/10/10 01:44:56 L2: [fex=71260265/set:0] [del:70561216/bat:0] [g/s:48/0] cached:699049 (~43690/char)
2023/10/10 01:44:56 L3: [fex=70677983/set:0] [del:70036305/bat:0] [g/s:80/0] cached:641678 (~40104/char)
2023/10/10 01:44:56 BoltSpeed: 34318.58 tx/s ( did=514780 in 15.0 sec ) totalTX=70680355
2023/10/10 01:45:11 L1: [fex=71199131/set:0] [del:70547975/bat:0] [g/s:80/0] cached:651160 (~40697/char)
2023/10/10 01:45:11 L2: [fex=71780186/set:0] [del:71120375/bat:0] [g/s:48/0] cached:659811 (~41238/char)
2023/10/10 01:45:11 L3: [fex=71189345/set:0] [del:70534176/bat:0] [g/s:80/0] cached:655169 (~40948/char)
2023/10/10 01:45:11 BoltSpeed: 34092.07 tx/s ( did=511377 in 15.0 sec ) totalTX=71191732
Alloc: 685 MiB, TotalAlloc: 604427 MiB, Sys: 1049 MiB, NumGC: 1518
2023/10/10 01:45:26 L1: [fex=71724075/set:0] [del:71083235/bat:0] [g/s:80/0] cached:640843 (~40052/char)
2023/10/10 01:45:26 L2: [fex=72313769/set:0] [del:71664289/bat:0] [g/s:48/0] cached:649480 (~40592/char)
2023/10/10 01:45:26 L3: [fex=71714212/set:0] [del:71071489/bat:0] [g/s:80/0] cached:642723 (~40170/char)
2023/10/10 01:45:26 BoltSpeed: 34992.00 tx/s ( did=524882 in 15.0 sec ) totalTX=71716614
2023/10/10 01:45:41 L1: [fex=72231661/set:0] [del:71653408/bat:0] [g/s:80/0] cached:578257 (~36141/char)
2023/10/10 01:45:41 L2: [fex=72829736/set:0] [del:72244006/bat:0] [g/s:48/0] cached:585730 (~36608/char)
2023/10/10 01:45:41 L3: [fex=72221731/set:0] [del:71641350/bat:0] [g/s:80/0] cached:580381 (~36273/char)
2023/10/10 01:45:41 BoltSpeed: 33835.79 tx/s ( did=507536 in 15.0 sec ) totalTX=72224150
2023/10/10 01:45:56 L1: [fex=72758758/set:0] [del:72070096/bat:0] [g/s:80/0] cached:688666 (~43041/char)
2023/10/10 01:45:56 L2: [fex=73365498/set:0] [del:72656874/bat:0] [g/s:48/0] cached:708624 (~44289/char)
Alloc: 547 MiB, TotalAlloc: 613160 MiB, Sys: 1049 MiB, NumGC: 1538
2023/10/10 01:45:56 L3: [fex=72748741/set:0] [del:72081455/bat:0] [g/s:80/0] cached:667286 (~41705/char)
2023/10/10 01:45:56 BoltSpeed: 35132.13 tx/s ( did=527023 in 15.0 sec ) totalTX=72751173
2023/10/10 01:46:11 L1: [fex=73276060/set:0] [del:72624442/bat:0] [g/s:80/0] cached:651621 (~40726/char)
2023/10/10 01:46:11 L2: [fex=73891463/set:0] [del:73238463/bat:0] [g/s:48/0] cached:653000 (~40812/char)
2023/10/10 01:46:11 L3: [fex=73265964/set:0] [del:72610031/bat:0] [g/s:80/0] cached:655933 (~40995/char)
2023/10/10 01:46:11 BoltSpeed: 34485.58 tx/s ( did=517242 in 15.0 sec ) totalTX=73268415
Alloc: 641 MiB, TotalAlloc: 621902 MiB, Sys: 1049 MiB, NumGC: 1558
2023/10/10 01:46:26 L1: [fex=73792934/set:0] [del:73167312/bat:0] [g/s:80/0] cached:625624 (~39101/char)
2023/10/10 01:46:26 L2: [fex=74416969/set:0] [del:73797522/bat:0] [g/s:48/0] cached:619447 (~38715/char)
2023/10/10 01:46:26 L3: [fex=73782745/set:0] [del:73152994/bat:0] [g/s:80/0] cached:629751 (~39359/char)
2023/10/10 01:46:26 BoltSpeed: 34452.80 tx/s ( did=516795 in 15.0 sec ) totalTX=73785210
2023/10/10 01:46:41 L1: [fex=74278096/set:0] [del:73727958/bat:0] [g/s:80/0] cached:550141 (~34383/char)
2023/10/10 01:46:41 L2: [fex=74910384/set:0] [del:74355245/bat:0] [g/s:48/0] cached:555139 (~34696/char)
2023/10/10 01:46:41 L3: [fex=74267877/set:0] [del:73715180/bat:0] [g/s:80/0] cached:552697 (~34543/char)
2023/10/10 01:46:41 BoltSpeed: 32244.29 tx/s ( did=485148 in 15.0 sec ) totalTX=74270358
Alloc: 834 MiB, TotalAlloc: 630110 MiB, Sys: 1049 MiB, NumGC: 1576
2023/10/10 01:46:56 L1: [fex=74765843/set:0] [del:74108393/bat:0] [g/s:80/0] cached:657452 (~41090/char)
2023/10/10 01:46:56 L2: [fex=75406458/set:0] [del:74767378/bat:0] [g/s:48/0] cached:639080 (~39942/char)
2023/10/10 01:46:56 L3: [fex=74755524/set:0] [del:74115771/bat:0] [g/s:80/0] cached:639753 (~39984/char)
2023/10/10 01:46:56 BoltSpeed: 32610.70 tx/s ( did=487665 in 15.0 sec ) totalTX=74758023
2023/10/10 01:47:11 L1: [fex=75265844/set:0] [del:74638981/bat:0] [g/s:80/0] cached:626865 (~39179/char)
2023/10/10 01:47:11 L2: [fex=75915207/set:0] [del:75305337/bat:0] [g/s:48/0] cached:609870 (~38116/char)
2023/10/10 01:47:11 L3: [fex=75255457/set:0] [del:74624836/bat:0] [g/s:80/0] cached:630621 (~39413/char)
2023/10/10 01:47:11 BoltSpeed: 33330.08 tx/s ( did=499947 in 15.0 sec ) totalTX=75257970
Alloc: 904 MiB, TotalAlloc: 638667 MiB, Sys: 1049 MiB, NumGC: 1596
2023/10/10 01:47:26 L1: [fex=75778609/set:0] [del:75174916/bat:0] [g/s:80/0] cached:603693 (~37730/char)
2023/10/10 01:47:26 L2: [fex=76437121/set:0] [del:75846087/bat:0] [g/s:48/0] cached:591034 (~36939/char)
2023/10/10 01:47:26 L3: [fex=75768133/set:0] [del:75154010/bat:0] [g/s:80/0] cached:614123 (~38382/char)
2023/10/10 01:47:26 BoltSpeed: 34179.56 tx/s ( did=512693 in 15.0 sec ) totalTX=75770663
2023/10/10 01:47:41 L1: [fex=76286684/set:0] [del:75731005/bat:0] [g/s:80/0] cached:555681 (~34730/char)
2023/10/10 01:47:41 L2: [fex=76954083/set:0] [del:76263491/bat:0] [g/s:48/0] cached:690592 (~43162/char)
2023/10/10 01:47:41 L3: [fex=76276149/set:0] [del:75710550/bat:0] [g/s:80/0] cached:565599 (~35349/char)
2023/10/10 01:47:41 BoltSpeed: 33869.01 tx/s ( did=508033 in 15.0 sec ) totalTX=76278696
Alloc: 660 MiB, TotalAlloc: 646670 MiB, Sys: 1049 MiB, NumGC: 1615
2023/10/10 01:47:56 L1: [fex=76727076/set:0] [del:76126942/bat:0] [g/s:80/0] cached:600138 (~37508/char)
2023/10/10 01:47:56 L2: [fex=77402289/set:0] [del:76806156/bat:0] [g/s:48/0] cached:596133 (~37258/char)
2023/10/10 01:47:56 L3: [fex=76716480/set:0] [del:76105960/bat:0] [g/s:80/0] cached:610520 (~38157/char)
2023/10/10 01:47:56 BoltSpeed: 29356.42 tx/s ( did=440346 in 15.0 sec ) totalTX=76719042
2023/10/10 01:48:11 L1: [fex=77187003/set:0] [del:76629164/bat:0] [g/s:80/0] cached:557841 (~34865/char)
2023/10/10 01:48:11 L2: [fex=77870334/set:0] [del:77314121/bat:0] [g/s:48/0] cached:556213 (~34763/char)
2023/10/10 01:48:11 L3: [fex=77176355/set:0] [del:76611978/bat:0] [g/s:80/0] cached:564377 (~35273/char)
2023/10/10 01:48:11 BoltSpeed: 30659.50 tx/s ( did=459892 in 15.0 sec ) totalTX=77178934
Alloc: 474 MiB, TotalAlloc: 654132 MiB, Sys: 1049 MiB, NumGC: 1633
2023/10/10 01:48:26 L1: [fex=77609254/set:0] [del:77111859/bat:0] [g/s:80/0] cached:497398 (~31087/char)
2023/10/10 01:48:26 L2: [fex=78300176/set:0] [del:77803192/bat:0] [g/s:48/0] cached:496984 (~31061/char)
2023/10/10 01:48:26 L3: [fex=77598572/set:0] [del:77093506/bat:0] [g/s:80/0] cached:505066 (~31566/char)
2023/10/10 01:48:26 BoltSpeed: 28148.76 tx/s ( did=422232 in 15.0 sec ) totalTX=77601166
2023/10/10 01:48:41 L1: [fex=78096044/set:0] [del:77497964/bat:0] [g/s:80/0] cached:598084 (~37380/char)
2023/10/10 01:48:41 L2: [fex=78795682/set:0] [del:78144355/bat:0] [g/s:48/0] cached:651327 (~40707/char)
2023/10/10 01:48:41 L3: [fex=78085298/set:0] [del:77530398/bat:0] [g/s:80/0] cached:554900 (~34681/char)
2023/10/10 01:48:41 BoltSpeed: 32449.52 tx/s ( did=486742 in 15.0 sec ) totalTX=78087908
Alloc: 658 MiB, TotalAlloc: 662171 MiB, Sys: 1049 MiB, NumGC: 1652
2023/10/10 01:48:56 L1: [fex=78563916/set:0] [del:77956485/bat:0] [g/s:80/0] cached:607434 (~37964/char)
2023/10/10 01:48:56 L2: [fex=79272014/set:0] [del:78664808/bat:0] [g/s:48/0] cached:607206 (~37950/char)
2023/10/10 01:48:56 L3: [fex=78553105/set:0] [del:77932650/bat:0] [g/s:80/0] cached:620455 (~38778/char)
2023/10/10 01:48:56 BoltSpeed: 31188.17 tx/s ( did=467823 in 15.0 sec ) totalTX=78555731
2023/10/10 01:49:11 L1: [fex=79065907/set:0] [del:78436710/bat:0] [g/s:80/0] cached:629200 (~39325/char)
2023/10/10 01:49:11 L2: [fex=79783184/set:0] [del:79156548/bat:0] [g/s:48/0] cached:626636 (~39164/char)
2023/10/10 01:49:11 L3: [fex=79055035/set:0] [del:78414871/bat:0] [g/s:80/0] cached:640164 (~40010/char)
2023/10/10 01:49:11 BoltSpeed: 33462.99 tx/s ( did=501945 in 15.0 sec ) totalTX=79057676
Alloc: 602 MiB, TotalAlloc: 670696 MiB, Sys: 1049 MiB, NumGC: 1672
2023/10/10 01:49:26 L1: [fex=79573574/set:0] [del:78976970/bat:0] [g/s:80/0] cached:596607 (~37287/char)
2023/10/10 01:49:26 L2: [fex=80300173/set:0] [del:79722133/bat:0] [g/s:48/0] cached:578040 (~36127/char)
2023/10/10 01:49:26 L3: [fex=79562615/set:0] [del:78957808/bat:0] [g/s:80/0] cached:604807 (~37800/char)
2023/10/10 01:49:26 BoltSpeed: 33839.99 tx/s ( did=507599 in 15.0 sec ) totalTX=79565275
2023/10/10 01:49:38 RUN test p=2 nntp-history added=0 dupes=19983506 cLock=39001165 addretry=0 retry=0 adddupes=0 cdupes=21015329 cretry1=0 cretry2=0 80000000/100000000
2023/10/10 01:49:38 RUN test p=3 nntp-history added=0 dupes=20009143 cLock=39022225 addretry=0 retry=0 adddupes=0 cdupes=20968632 cretry1=0 cretry2=0 80000000/100000000
2023/10/10 01:49:38 RUN test p=4 nntp-history added=0 dupes=20001317 cLock=39018526 addretry=0 retry=0 adddupes=0 cdupes=20980157 cretry1=0 cretry2=0 80000000/100000000
2023/10/10 01:49:38 RUN test p=1 nntp-history added=0 dupes=20006034 cLock=39015517 addretry=0 retry=0 adddupes=0 cdupes=20978449 cretry1=0 cretry2=0 80000000/100000000
2023/10/10 01:49:41 L1: [fex=80084354/set:0] [del:79443441/bat:0] [g/s:80/0] cached:640915 (~40057/char)
2023/10/10 01:49:41 L2: [fex=80820296/set:0] [del:80146475/bat:0] [g/s:48/0] cached:673821 (~42113/char)
2023/10/10 01:49:41 L3: [fex=80073313/set:0] [del:79466067/bat:0] [g/s:80/0] cached:607246 (~37952/char)
2023/10/10 01:49:41 BoltSpeed: 34047.50 tx/s ( did=510713 in 15.0 sec ) totalTX=80075988
Alloc: 916 MiB, TotalAlloc: 679196 MiB, Sys: 1049 MiB, NumGC: 1692
2023/10/10 01:49:56 L1: [fex=80581036/set:0] [del:79939246/bat:0] [g/s:80/0] cached:641791 (~40111/char)
2023/10/10 01:49:56 L2: [fex=81326377/set:0] [del:80700901/bat:0] [g/s:48/0] cached:625476 (~39092/char)
2023/10/10 01:49:56 L3: [fex=80569924/set:0] [del:79917271/bat:0] [g/s:80/0] cached:652653 (~40790/char)
2023/10/10 01:49:56 BoltSpeed: 32896.19 tx/s ( did=496627 in 15.1 sec ) totalTX=80572615
2023/10/10 01:50:11 L1: [fex=81077879/set:0] [del:80473271/bat:0] [g/s:80/0] cached:604611 (~37788/char)
2023/10/10 01:50:11 L2: [fex=81832544/set:0] [del:81248178/bat:0] [g/s:48/0] cached:584366 (~36522/char)
2023/10/10 01:50:11 L3: [fex=81066701/set:0] [del:80456669/bat:0] [g/s:80/0] cached:610032 (~38127/char)
2023/10/10 01:50:11 BoltSpeed: 33334.48 tx/s ( did=496792 in 14.9 sec ) totalTX=81069407
2023/10/10 01:50:26 L1: [fex=81551724/set:0] [del:81005182/bat:0] [g/s:80/0] cached:546544 (~34159/char)
2023/10/10 01:50:26 L2: [fex=82315199/set:0] [del:81677951/bat:0] [g/s:48/0] cached:637248 (~39828/char)
2023/10/10 01:50:26 L3: [fex=81540466/set:0] [del:80988194/bat:0] [g/s:80/0] cached:552272 (~34517/char)
2023/10/10 01:50:26 BoltSpeed: 31585.30 tx/s ( did=473780 in 15.0 sec ) totalTX=81543187
Alloc: 922 MiB, TotalAlloc: 687394 MiB, Sys: 1049 MiB, NumGC: 1710
2023/10/10 01:50:41 L1: [fex=82063983/set:0] [del:81400512/bat:0] [g/s:80/0] cached:663472 (~41467/char)
2023/10/10 01:50:41 L2: [fex=82837141/set:0] [del:82184092/bat:0] [g/s:48/0] cached:653049 (~40815/char)
2023/10/10 01:50:41 L3: [fex=82052659/set:0] [del:81410936/bat:0] [g/s:80/0] cached:641723 (~40107/char)
2023/10/10 01:50:41 BoltSpeed: 34143.61 tx/s ( did=512210 in 15.0 sec ) totalTX=82055397
Alloc: 578 MiB, TotalAlloc: 695994 MiB, Sys: 1049 MiB, NumGC: 1731
2023/10/10 01:50:56 L1: [fex=82573315/set:0] [del:81910166/bat:0] [g/s:80/0] cached:663151 (~41446/char)
2023/10/10 01:50:56 L2: [fex=83356026/set:0] [del:82723238/bat:0] [g/s:48/0] cached:632788 (~39549/char)
2023/10/10 01:50:56 L3: [fex=82561902/set:0] [del:81896725/bat:0] [g/s:80/0] cached:665177 (~41573/char)
2023/10/10 01:50:56 BoltSpeed: 33954.48 tx/s ( did=509259 in 15.0 sec ) totalTX=82564656
2023/10/10 01:51:11 L1: [fex=83094799/set:0] [del:82461823/bat:0] [g/s:80/0] cached:632979 (~39561/char)
2023/10/10 01:51:11 L2: [fex=83887390/set:0] [del:83280093/bat:0] [g/s:48/0] cached:607297 (~37956/char)
2023/10/10 01:51:11 L3: [fex=83083311/set:0] [del:82446176/bat:0] [g/s:80/0] cached:637135 (~39820/char)
2023/10/10 01:51:11 BoltSpeed: 34761.68 tx/s ( did=521425 in 15.0 sec ) totalTX=83086081
2023/10/10 01:51:26 L1: [fex=83596814/set:0] [del:83018807/bat:0] [g/s:80/0] cached:578010 (~36125/char)
2023/10/10 01:51:26 L2: [fex=84399294/set:0] [del:83704364/bat:0] [g/s:48/0] cached:694930 (~43433/char)
2023/10/10 01:51:26 L3: [fex=83585263/set:0] [del:83003107/bat:0] [g/s:80/0] cached:582156 (~36384/char)
2023/10/10 01:51:26 BoltSpeed: 33462.88 tx/s ( did=501967 in 15.0 sec ) totalTX=83588048
Alloc: 859 MiB, TotalAlloc: 704648 MiB, Sys: 1049 MiB, NumGC: 1750
2023/10/10 01:51:41 L1: [fex=84098145/set:0] [del:83441475/bat:0] [g/s:80/0] cached:656673 (~41042/char)
2023/10/10 01:51:41 L2: [fex=84910336/set:0] [del:84258124/bat:0] [g/s:48/0] cached:652212 (~40763/char)
2023/10/10 01:51:41 L3: [fex=84086527/set:0] [del:83435862/bat:0] [g/s:80/0] cached:650665 (~40666/char)
2023/10/10 01:51:41 BoltSpeed: 33420.32 tx/s ( did=501282 in 15.0 sec ) totalTX=84089330
Alloc: 640 MiB, TotalAlloc: 713072 MiB, Sys: 1049 MiB, NumGC: 1770
2023/10/10 01:51:56 L1: [fex=84595981/set:0] [del:83962341/bat:0] [g/s:80/0] cached:633643 (~39602/char)
2023/10/10 01:51:56 L2: [fex=85417904/set:0] [del:84792452/bat:0] [g/s:48/0] cached:625452 (~39090/char)
2023/10/10 01:51:56 L3: [fex=84584322/set:0] [del:83950740/bat:0] [g/s:80/0] cached:633582 (~39598/char)
2023/10/10 01:51:56 BoltSpeed: 33185.16 tx/s ( did=497811 in 15.0 sec ) totalTX=84587141
2023/10/10 01:52:11 L1: [fex=85079527/set:0] [del:84495081/bat:0] [g/s:80/0] cached:584447 (~36527/char)
2023/10/10 01:52:11 L2: [fex=85910910/set:0] [del:85344204/bat:0] [g/s:48/0] cached:566706 (~35419/char)
2023/10/10 01:52:11 L3: [fex=85067781/set:0] [del:84483406/bat:0] [g/s:80/0] cached:584375 (~36523/char)
2023/10/10 01:52:11 BoltSpeed: 32233.58 tx/s ( did=483475 in 15.0 sec ) totalTX=85070616
Alloc: 562 MiB, TotalAlloc: 721401 MiB, Sys: 1049 MiB, NumGC: 1790
2023/10/10 01:52:26 L1: [fex=85583035/set:0] [del:85016442/bat:0] [g/s:80/0] cached:566595 (~35412/char)
2023/10/10 01:52:26 L2: [fex=86424382/set:0] [del:85747152/bat:0] [g/s:48/0] cached:677230 (~42326/char)
2023/10/10 01:52:26 L3: [fex=85571217/set:0] [del:85001435/bat:0] [g/s:80/0] cached:569782 (~35611/char)
2023/10/10 01:52:26 BoltSpeed: 33563.70 tx/s ( did=503452 in 15.0 sec ) totalTX=85574068
2023/10/10 01:52:41 L1: [fex=86074782/set:0] [del:85429605/bat:0] [g/s:80/0] cached:645180 (~40323/char)
2023/10/10 01:52:41 L2: [fex=86925668/set:0] [del:86311246/bat:0] [g/s:48/0] cached:614422 (~38401/char)
2023/10/10 01:52:41 L3: [fex=86062903/set:0] [del:85417908/bat:0] [g/s:80/0] cached:644995 (~40312/char)
2023/10/10 01:52:41 BoltSpeed: 32780.06 tx/s ( did=491701 in 15.0 sec ) totalTX=86065769
Alloc: 869 MiB, TotalAlloc: 729844 MiB, Sys: 1049 MiB, NumGC: 1810
2023/10/10 01:52:56 L1: [fex=86584439/set:0] [del:85943504/bat:0] [g/s:80/0] cached:640938 (~40058/char)
2023/10/10 01:52:56 L2: [fex=87445458/set:0] [del:86839353/bat:0] [g/s:48/0] cached:606105 (~37881/char)
2023/10/10 01:52:56 L3: [fex=86572487/set:0] [del:85916231/bat:0] [g/s:80/0] cached:656256 (~41016/char)
2023/10/10 01:52:56 BoltSpeed: 33973.02 tx/s ( did=509599 in 15.0 sec ) totalTX=86575368
2023/10/10 01:53:11 L1: [fex=87077443/set:0] [del:86498211/bat:0] [g/s:80/0] cached:579236 (~36202/char)
2023/10/10 01:53:11 L2: [fex=87948338/set:0] [del:87315001/bat:0] [g/s:48/0] cached:633337 (~39583/char)
2023/10/10 01:53:11 L3: [fex=87065423/set:0] [del:86475408/bat:0] [g/s:80/0] cached:590015 (~36875/char)
2023/10/10 01:53:11 BoltSpeed: 32853.20 tx/s ( did=492951 in 15.0 sec ) totalTX=87068319
Alloc: 723 MiB, TotalAlloc: 738340 MiB, Sys: 1054 MiB, NumGC: 1830
2023/10/10 01:53:26 L1: [fex=87590964/set:0] [del:86971321/bat:0] [g/s:80/0] cached:619645 (~38727/char)
2023/10/10 01:53:26 L2: [fex=88472310/set:0] [del:87799913/bat:0] [g/s:48/0] cached:672397 (~42024/char)
2023/10/10 01:53:26 L3: [fex=87578863/set:0] [del:86996884/bat:0] [g/s:80/0] cached:581979 (~36373/char)
2023/10/10 01:53:26 BoltSpeed: 34241.52 tx/s ( did=513459 in 15.0 sec ) totalTX=87581778
2023/10/10 01:53:41 L1: [fex=88081060/set:0] [del:87442156/bat:0] [g/s:80/0] cached:638906 (~39931/char)
2023/10/10 01:53:41 L2: [fex=88972332/set:0] [del:88355076/bat:0] [g/s:48/0] cached:617256 (~38578/char)
2023/10/10 01:53:41 L3: [fex=88068880/set:0] [del:87415301/bat:0] [g/s:80/0] cached:653579 (~40848/char)
2023/10/10 01:53:41 BoltSpeed: 32661.23 tx/s ( did=490032 in 15.0 sec ) totalTX=88071810
Alloc: 705 MiB, TotalAlloc: 746722 MiB, Sys: 1054 MiB, NumGC: 1849
2023/10/10 01:53:56 L1: [fex=88583898/set:0] [del:87977254/bat:0] [g/s:80/0] cached:606647 (~37915/char)
2023/10/10 01:53:56 L2: [fex=89485318/set:0] [del:88899660/bat:0] [g/s:48/0] cached:585658 (~36603/char)
2023/10/10 01:53:56 L3: [fex=88571665/set:0] [del:87950947/bat:0] [g/s:80/0] cached:620718 (~38794/char)
2023/10/10 01:53:56 BoltSpeed: 33527.94 tx/s ( did=502802 in 15.0 sec ) totalTX=88574612
2023/10/10 01:54:11 L1: [fex=89104972/set:0] [del:88505910/bat:0] [g/s:80/0] cached:599065 (~37441/char)
2023/10/10 01:54:11 L2: [fex=90017048/set:0] [del:89322067/bat:0] [g/s:48/0] cached:694981 (~43436/char)
2023/10/10 01:54:11 L3: [fex=89092667/set:0] [del:88484473/bat:0] [g/s:80/0] cached:608194 (~38012/char)
2023/10/10 01:54:11 BoltSpeed: 34734.55 tx/s ( did=521018 in 15.0 sec ) totalTX=89095630
Alloc: 794 MiB, TotalAlloc: 755495 MiB, Sys: 1058 MiB, NumGC: 1869
2023/10/10 01:54:26 L1: [fex=89624934/set:0] [del:88944957/bat:0] [g/s:80/0] cached:679979 (~42498/char)
2023/10/10 01:54:26 L2: [fex=90547779/set:0] [del:89868271/bat:0] [g/s:48/0] cached:679508 (~42469/char)
2023/10/10 01:54:26 L3: [fex=89612549/set:0] [del:88975359/bat:0] [g/s:80/0] cached:637190 (~39824/char)
2023/10/10 01:54:26 BoltSpeed: 34659.78 tx/s ( did=519898 in 15.0 sec ) totalTX=89615528
2023/10/10 01:54:37 RUN test p=2 nntp-history added=0 dupes=22481323 cLock=43888633 addretry=0 retry=0 adddupes=0 cdupes=23630044 cretry1=0 cretry2=0 90000000/100000000
2023/10/10 01:54:37 RUN test p=1 nntp-history added=0 dupes=22504273 cLock=43900576 addretry=0 retry=0 adddupes=0 cdupes=23595151 cretry1=0 cretry2=0 90000000/100000000
2023/10/10 01:54:37 RUN test p=3 nntp-history added=0 dupes=22511166 cLock=43911741 addretry=0 retry=0 adddupes=0 cdupes=23577093 cretry1=0 cretry2=0 90000000/100000000
2023/10/10 01:54:37 RUN test p=4 nntp-history added=0 dupes=22503238 cLock=43906262 addretry=0 retry=0 adddupes=0 cdupes=23590500 cretry1=0 cretry2=0 90000000/100000000
2023/10/10 01:54:41 L1: [fex=90119885/set:0] [del:89481178/bat:0] [g/s:80/0] cached:638710 (~39919/char)
2023/10/10 01:54:41 L2: [fex=91053109/set:0] [del:90439167/bat:0] [g/s:48/0] cached:613942 (~38371/char)
2023/10/10 01:54:41 L3: [fex=90107424/set:0] [del:89460056/bat:0] [g/s:80/0] cached:647368 (~40460/char)
2023/10/10 01:54:41 BoltSpeed: 32992.64 tx/s ( did=494890 in 15.0 sec ) totalTX=90110418
Alloc: 497 MiB, TotalAlloc: 763998 MiB, Sys: 1083 MiB, NumGC: 1889
2023/10/10 01:54:56 L1: [fex=90631750/set:0] [del:90017443/bat:0] [g/s:80/0] cached:614309 (~38394/char)
2023/10/10 01:54:56 L2: [fex=91575538/set:0] [del:90984177/bat:0] [g/s:48/0] cached:591361 (~36960/char)
2023/10/10 01:54:56 L3: [fex=90619211/set:0] [del:89996324/bat:0] [g/s:80/0] cached:622887 (~38930/char)
2023/10/10 01:54:56 BoltSpeed: 34120.33 tx/s ( did=511804 in 15.0 sec ) totalTX=90622222
2023/10/10 01:55:11 L1: [fex=91160808/set:0] [del:90561593/bat:0] [g/s:80/0] cached:599218 (~37451/char)
2023/10/10 01:55:11 L2: [fex=92115660/set:0] [del:91393569/bat:0] [g/s:48/0] cached:722091 (~45130/char)
2023/10/10 01:55:11 L3: [fex=91148194/set:0] [del:90541625/bat:0] [g/s:80/0] cached:606569 (~37910/char)
2023/10/10 01:55:11 BoltSpeed: 35266.45 tx/s ( did=528999 in 15.0 sec ) totalTX=91151221
Alloc: 643 MiB, TotalAlloc: 772837 MiB, Sys: 1083 MiB, NumGC: 1909
2023/10/10 01:55:26 L1: [fex=91679517/set:0] [del:90986514/bat:0] [g/s:80/0] cached:693007 (~43312/char)
2023/10/10 01:55:26 L2: [fex=92645071/set:0] [del:91969237/bat:0] [g/s:48/0] cached:675834 (~42239/char)
2023/10/10 01:55:26 L3: [fex=91666829/set:0] [del:91001844/bat:0] [g/s:80/0] cached:664985 (~41561/char)
2023/10/10 01:55:26 BoltSpeed: 34576.87 tx/s ( did=518651 in 15.0 sec ) totalTX=91669872
2023/10/10 01:55:41 L1: [fex=92184749/set:0] [del:91544420/bat:0] [g/s:80/0] cached:640331 (~40020/char)
2023/10/10 01:55:41 L2: [fex=93160981/set:0] [del:92561698/bat:0] [g/s:48/0] cached:599283 (~37455/char)
2023/10/10 01:55:41 L3: [fex=92171982/set:0] [del:91523946/bat:0] [g/s:80/0] cached:648036 (~40502/char)
2023/10/10 01:55:41 BoltSpeed: 33669.85 tx/s ( did=505167 in 15.0 sec ) totalTX=92175039
2023/10/10 01:55:56 L1: [fex=92654546/set:0] [del:92083633/bat:0] [g/s:80/0] cached:570917 (~35682/char)
Alloc: 709 MiB, TotalAlloc: 781095 MiB, Sys: 1087 MiB, NumGC: 1927
2023/10/10 01:55:56 L2: [fex=93640779/set:0] [del:93099372/bat:0] [g/s:48/0] cached:541407 (~33837/char)
2023/10/10 01:55:56 L3: [fex=92641734/set:0] [del:92065730/bat:0] [g/s:80/0] cached:576004 (~36000/char)
2023/10/10 01:55:56 BoltSpeed: 31298.41 tx/s ( did=469768 in 15.0 sec ) totalTX=92644807
2023/10/10 01:56:11 L1: [fex=93132720/set:0] [del:92586639/bat:0] [g/s:80/0] cached:546083 (~34130/char)
2023/10/10 01:56:11 L2: [fex=94129142/set:0] [del:93498844/bat:0] [g/s:48/0] cached:630298 (~39393/char)
2023/10/10 01:56:11 L3: [fex=93119832/set:0] [del:92571852/bat:0] [g/s:80/0] cached:547980 (~34248/char)
2023/10/10 01:56:11 BoltSpeed: 31864.34 tx/s ( did=478114 in 15.0 sec ) totalTX=93122921
2023/10/10 01:56:26 L1: [fex=93653368/set:0] [del:92986638/bat:0] [g/s:80/0] cached:666732 (~41670/char)
2023/10/10 01:56:26 L2: [fex=94661225/set:0] [del:94024649/bat:0] [g/s:48/0] cached:636576 (~39786/char)
2023/10/10 01:56:26 L3: [fex=93640413/set:0] [del:92993521/bat:0] [g/s:80/0] cached:646892 (~40430/char)
2023/10/10 01:56:26 BoltSpeed: 34747.15 tx/s ( did=520598 in 15.0 sec ) totalTX=93643519
Alloc: 845 MiB, TotalAlloc: 789513 MiB, Sys: 1087 MiB, NumGC: 1946
2023/10/10 01:56:41 L1: [fex=94144280/set:0] [del:93496595/bat:0] [g/s:80/0] cached:647687 (~40480/char)
2023/10/10 01:56:41 L2: [fex=95162771/set:0] [del:94574214/bat:0] [g/s:48/0] cached:588557 (~36784/char)
2023/10/10 01:56:41 L3: [fex=94131248/set:0] [del:93481428/bat:0] [g/s:80/0] cached:649820 (~40613/char)
2023/10/10 01:56:41 BoltSpeed: 32723.44 tx/s ( did=490852 in 15.0 sec ) totalTX=94134371
Alloc: 659 MiB, TotalAlloc: 797514 MiB, Sys: 1088 MiB, NumGC: 1964
2023/10/10 01:56:56 L1: [fex=94599764/set:0] [del:94042358/bat:0] [g/s:80/0] cached:557409 (~34838/char)
2023/10/10 01:56:56 L2: [fex=95628238/set:0] [del:95000924/bat:0] [g/s:48/0] cached:627314 (~39207/char)
2023/10/10 01:56:56 L3: [fex=94586675/set:0] [del:94025414/bat:0] [g/s:80/0] cached:561261 (~35078/char)
2023/10/10 01:56:56 BoltSpeed: 30362.81 tx/s ( did=455443 in 15.0 sec ) totalTX=94589814
2023/10/10 01:57:11 L1: [fex=95083281/set:0] [del:94538127/bat:0] [g/s:80/0] cached:545155 (~34072/char)
2023/10/10 01:57:11 L2: [fex=96122469/set:0] [del:95485259/bat:0] [g/s:48/0] cached:637210 (~39825/char)
2023/10/10 01:57:11 L3: [fex=95070134/set:0] [del:94512060/bat:0] [g/s:80/0] cached:558074 (~34879/char)
2023/10/10 01:57:11 BoltSpeed: 32231.49 tx/s ( did=483473 in 15.0 sec ) totalTX=95073287
2023/10/10 01:57:26 L1: [fex=95548844/set:0] [del:94939842/bat:0] [g/s:80/0] cached:609003 (~38062/char)
2023/10/10 01:57:26 L2: [fex=96598391/set:0] [del:96018783/bat:0] [g/s:48/0] cached:579608 (~36225/char)
2023/10/10 01:57:26 L3: [fex=95535639/set:0] [del:94921563/bat:0] [g/s:80/0] cached:614076 (~38379/char)
2023/10/10 01:57:26 BoltSpeed: 31034.85 tx/s ( did=465521 in 15.0 sec ) totalTX=95538808
Alloc: 867 MiB, TotalAlloc: 805518 MiB, Sys: 1088 MiB, NumGC: 1982
2023/10/10 01:57:41 L1: [fex=96003978/set:0] [del:95441514/bat:0] [g/s:80/0] cached:562465 (~35154/char)
2023/10/10 01:57:41 L2: [fex=97063485/set:0] [del:96537724/bat:0] [g/s:48/0] cached:525761 (~32860/char)
2023/10/10 01:57:41 L3: [fex=95990710/set:0] [del:95415000/bat:0] [g/s:80/0] cached:575710 (~35981/char)
2023/10/10 01:57:41 BoltSpeed: 30338.79 tx/s ( did=455087 in 15.0 sec ) totalTX=95993895
Alloc: 546 MiB, TotalAlloc: 813521 MiB, Sys: 1088 MiB, NumGC: 2001
2023/10/10 01:57:56 L1: [fex=96497170/set:0] [del:95924139/bat:0] [g/s:80/0] cached:573033 (~35814/char)
2023/10/10 01:57:56 L2: [fex=97567783/set:0] [del:96904474/bat:0] [g/s:48/0] cached:663309 (~41456/char)
2023/10/10 01:57:56 L3: [fex=96483835/set:0] [del:95904190/bat:0] [g/s:80/0] cached:579645 (~36227/char)
2023/10/10 01:57:56 BoltSpeed: 32876.57 tx/s ( did=493143 in 15.0 sec ) totalTX=96487038
2023/10/10 01:58:11 L1: [fex=96992266/set:0] [del:96405922/bat:0] [g/s:80/0] cached:586345 (~36646/char)
2023/10/10 01:58:11 L2: [fex=98073849/set:0] [del:97431303/bat:0] [g/s:48/0] cached:642546 (~40159/char)
2023/10/10 01:58:11 L3: [fex=96978867/set:0] [del:96431328/bat:0] [g/s:80/0] cached:547539 (~34221/char)
2023/10/10 01:58:11 BoltSpeed: 32973.81 tx/s ( did=495058 in 15.0 sec ) totalTX=96982096
Alloc: 832 MiB, TotalAlloc: 821869 MiB, Sys: 1088 MiB, NumGC: 2019
2023/10/10 01:58:26 L1: [fex=97487648/set:0] [del:96850137/bat:0] [g/s:80/0] cached:637512 (~39844/char)
2023/10/10 01:58:26 L2: [fex=98580445/set:0] [del:97964972/bat:0] [g/s:48/0] cached:615473 (~38467/char)
2023/10/10 01:58:26 L3: [fex=97474174/set:0] [del:96830936/bat:0] [g/s:80/0] cached:643238 (~40202/char)
2023/10/10 01:58:26 BoltSpeed: 33049.49 tx/s ( did=495312 in 15.0 sec ) totalTX=97477408
2023/10/10 01:58:41 L1: [fex=97983432/set:0] [del:97383419/bat:0] [g/s:80/0] cached:600016 (~37501/char)
2023/10/10 01:58:41 L2: [fex=99087500/set:0] [del:98507596/bat:0] [g/s:48/0] cached:579904 (~36244/char)
2023/10/10 01:58:41 L3: [fex=97969886/set:0] [del:97353062/bat:0] [g/s:80/0] cached:616824 (~38551/char)
2023/10/10 01:58:41 BoltSpeed: 33050.07 tx/s ( did=495729 in 15.0 sec ) totalTX=97973137
Alloc: 697 MiB, TotalAlloc: 830220 MiB, Sys: 1088 MiB, NumGC: 2038
2023/10/10 01:58:56 L1: [fex=98476853/set:0] [del:97914471/bat:0] [g/s:80/0] cached:562386 (~35149/char)
2023/10/10 01:58:56 L2: [fex=99592084/set:0] [del:98930689/bat:0] [g/s:48/0] cached:661395 (~41337/char)
2023/10/10 01:58:56 L3: [fex=98463242/set:0] [del:97884870/bat:0] [g/s:80/0] cached:578372 (~36148/char)
2023/10/10 01:58:56 BoltSpeed: 32891.49 tx/s ( did=493372 in 15.0 sec ) totalTX=98466509
2023/10/10 01:59:11 L1: [fex=98959652/set:0] [del:98322224/bat:0] [g/s:80/0] cached:637430 (~39839/char)
2023/10/10 01:59:11 L2: [fex=100085983/set:0] [del:99482428/bat:0] [g/s:48/0] cached:603555 (~37722/char)
2023/10/10 01:59:11 L3: [fex=98945994/set:0] [del:98349245/bat:0] [g/s:80/0] cached:596749 (~37296/char)
2023/10/10 01:59:11 BoltSpeed: 32183.45 tx/s ( did=482767 in 15.0 sec ) totalTX=98949276
Alloc: 572 MiB, TotalAlloc: 838389 MiB, Sys: 1088 MiB, NumGC: 2057
2023/10/10 01:59:26 L1: [fex=99445588/set:0] [del:98814322/bat:0] [g/s:80/0] cached:631269 (~39454/char)
2023/10/10 01:59:26 L2: [fex=100583035/set:0] [del:100005117/bat:0] [g/s:48/0] cached:577918 (~36119/char)
2023/10/10 01:59:26 L3: [fex=99431866/set:0] [del:98791458/bat:0] [g/s:80/0] cached:640408 (~40025/char)
2023/10/10 01:59:26 BoltSpeed: 32393.55 tx/s ( did=485889 in 15.0 sec ) totalTX=99435165
2023/10/10 01:59:41 L1: [fex=99942788/set:0] [del:99342217/bat:0] [g/s:80/0] cached:600574 (~37535/char)
2023/10/10 01:59:41 L2: [fex=101091717/set:0] [del:100439175/bat:0] [g/s:48/0] cached:652542 (~40783/char)
2023/10/10 01:59:41 L3: [fex=99928987/set:0] [del:99321259/bat:0] [g/s:80/0] cached:607728 (~37983/char)
2023/10/10 01:59:41 BoltSpeed: 33142.44 tx/s ( did=497136 in 15.0 sec ) totalTX=99932301
2023/10/10 01:59:42 End test p=1 nntp-history added=0 dupes=25003211 cLock=48736909 addretry=0 retry=0 adddupes=0 cdupes=26259880 cretry1=0 cretry2=0 sum=100000000/100000000 errors=0 locked=25003211
2023/10/10 01:59:42 End test p=4 nntp-history added=0 dupes=25003323 cLock=48742752 addretry=0 retry=0 adddupes=0 cdupes=26253925 cretry1=0 cretry2=0 sum=100000000/100000000 errors=0 locked=25003323
2023/10/10 01:59:42 End test p=3 nntp-history added=0 dupes=25012315 cLock=48748072 addretry=0 retry=0 adddupes=0 cdupes=26239613 cretry1=0 cretry2=0 sum=100000000/100000000 errors=0 locked=25012315
2023/10/10 01:59:42 End test p=2 nntp-history added=0 dupes=24981151 cLock=48724079 addretry=0 retry=0 adddupes=0 cdupes=26294770 cretry1=0 cretry2=0 sum=100000000/100000000 errors=0 locked=24981151
2023/10/10 01:59:42 CLOSE_HISTORY: his.WriterChan <- nil
2023/10/10 01:59:42 WAIT CLOSE_HISTORY: lock1=true=1 lock2=true=1 lock3=true=16 lock4=true=16 lock5=true=256 batchQueued=false=0 batchLocked=false=0
2023/10/10 01:59:43 CLOSE_HISTORY DONE
2023/10/10 01:59:43 key_add=0 key_app=0 total=0 fseeks=101150197 eof=0 BoltDB_decodedOffsets=99986182 addoffset=0 appoffset=0 trymultioffsets=2300925 tryoffset=97699075 searches=100000000 inserted=0
2023/10/10 01:59:43 L1LOCK=100000000 | Get: L2=13942 L3=13818 | wCBBS=~64 conti=0 slept=6192
2023/10/10 01:59:43 done=400000000 (took 3106 seconds) (closewait 1 seconds)
Alloc: 793 MiB, TotalAlloc: 843078 MiB, Sys: 1088 MiB, NumGC: 2067
2023/10/10 01:59:43 runtime.GC() [ 1 / 2 ] sleep 30 sec
2023/10/10 01:59:56 L1: [fex=100000000/set:0] [del:99865795/bat:0] [g/s:80/0] cached:134205 (~8387/char)
Alloc: 456 MiB, TotalAlloc: 843180 MiB, Sys: 1120 MiB, NumGC: 2068
2023/10/10 01:59:56 L2: [fex=101150197/set:0] [del:100939818/bat:0] [g/s:48/8] cached:210379 (~13148/char)
2023/10/10 01:59:56 L3: [fex=99986182/set:0] [del:99844543/bat:0] [g/s:80/0] cached:141639 (~8852/char)
2023/10/10 02:00:11 L1: [fex=100000000/set:0] [del:100000000/bat:0] [g/s:80/32] cached:0 (~0/char)
2023/10/10 02:00:11 L2: [fex=101150197/set:0] [del:101150197/bat:0] [g/s:48/24] cached:0 (~0/char)
2023/10/10 02:00:11 L3: [fex=99986182/set:0] [del:99986182/bat:0] [g/s:80/32] cached:0 (~0/char)
Alloc: 569 MiB, TotalAlloc: 843292 MiB, Sys: 1173 MiB, NumGC: 2068
2023/10/10 02:00:14 runtime.GC() [ 2 / 2 ] sleep 30 sec
Alloc: 26 MiB, TotalAlloc: 843302 MiB, Sys: 1173 MiB, NumGC: 2069
2023/10/10 02:00:26 L1: [fex=100000000/set:0] [del:100000000/bat:0] [g/s:80/64] cached:0 (~0/char)
2023/10/10 02:00:26 L2: [fex=101150197/set:0] [del:101150197/bat:0] [g/s:48/40] cached:0 (~0/char)
2023/10/10 02:00:26 L3: [fex=99986182/set:0] [del:99986182/bat:0] [g/s:80/64] cached:0 (~0/char)
2023/10/10 02:00:41 L1: [fex=100000000/set:0] [del:100000000/bat:0] [g/s:80/96] cached:0 (~0/char)
2023/10/10 02:00:41 L2: [fex=101150197/set:0] [del:101150197/bat:0] [g/s:48/56] cached:0 (~0/char)
2023/10/10 02:00:41 L3: [fex=99986182/set:0] [del:99986182/bat:0] [g/s:80/96] cached:0 (~0/char)
```
