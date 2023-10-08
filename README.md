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
2023/10/08 14:21:21 History: new=true
  HF='history/history.dat' DB='hashdb/history.dat.hash.[0-9a-f]'
  KeyAlgo=11 KeyLen=6 NumQueueWriteChan=16
  HashDBQueues:{NumQueueIndexChan=16 NumQueueIndexChans=4 BatchSize=1024 IndexParallel=16}
2023/10/08 14:21:43 End test p=4 nntp-history added=251227 dupes=0 cachehits=418534 addretry=0 retry=0 adddupes=0 cachedupes=330239 cacheretry1=0 sum=1000000/1000000 errors=0 locked=251227
2023/10/08 14:21:43 End test p=1 nntp-history added=251137 dupes=0 cachehits=418103 addretry=0 retry=0 adddupes=0 cachedupes=330760 cacheretry1=0 sum=1000000/1000000 errors=0 locked=251137
2023/10/08 14:21:43 End test p=3 nntp-history added=247685 dupes=0 cachehits=417019 addretry=0 retry=0 adddupes=0 cachedupes=335296 cacheretry1=0 sum=1000000/1000000 errors=0 locked=247685
2023/10/08 14:21:43 End test p=2 nntp-history added=249951 dupes=0 cachehits=417742 addretry=0 retry=0 adddupes=0 cachedupes=332307 cacheretry1=0 sum=1000000/1000000 errors=0 locked=249951
2023/10/08 14:21:43 CLOSE_HISTORY: his.WriterChan <- nil
2023/10/08 14:21:43 WAIT CLOSE_HISTORY: lock1=true=1 lock2=true=1 lock3=true=16 lock4=true=16 lock5=true=256 batchQueued=true=54164 batchLocked=false=0
2023/10/08 14:21:44 WAIT CLOSE_HISTORY: lock1=false=0 lock2=false=0 lock3=true=16 lock4=true=16 lock5=true=256 batchQueued=true=23874 batchLocked=true=16
2023/10/08 14:21:44 Quit HDBZW char=f added=62371 passed=0 dupes=0 processed=62371 searches=62371 retry=0
2023/10/08 14:21:44 Quit HDBZW char=3 added=62122 passed=0 dupes=0 processed=62122 searches=62122 retry=0
2023/10/08 14:21:44 Quit HDBZW char=d added=62645 passed=0 dupes=0 processed=62645 searches=62645 retry=0
2023/10/08 14:21:45 Quit HDBZW char=7 added=62625 passed=0 dupes=0 processed=62625 searches=62625 retry=0
2023/10/08 14:21:45 Quit HDBZW char=6 added=62455 passed=0 dupes=0 processed=62455 searches=62455 retry=0
2023/10/08 14:21:45 Quit HDBZW char=b added=62471 passed=0 dupes=0 processed=62471 searches=62471 retry=0
2023/10/08 14:21:45 Quit HDBZW char=2 added=62530 passed=0 dupes=0 processed=62530 searches=62530 retry=0
2023/10/08 14:21:45 Quit HDBZW char=5 added=62388 passed=0 dupes=0 processed=62388 searches=62388 retry=0
2023/10/08 14:21:45 Quit HDBZW char=8 added=62452 passed=0 dupes=0 processed=62452 searches=62452 retry=0
2023/10/08 14:21:45 Quit HDBZW char=1 added=62477 passed=0 dupes=0 processed=62477 searches=62477 retry=0
2023/10/08 14:21:45 Quit HDBZW char=0 added=62326 passed=0 dupes=0 processed=62326 searches=62326 retry=0
2023/10/08 14:21:45 Quit HDBZW char=9 added=62851 passed=0 dupes=0 processed=62851 searches=62851 retry=0
2023/10/08 14:21:45 Quit HDBZW char=4 added=62322 passed=0 dupes=0 processed=62322 searches=62322 retry=0
2023/10/08 14:21:45 Quit HDBZW char=a added=62633 passed=0 dupes=0 processed=62633 searches=62633 retry=0
2023/10/08 14:21:45 Quit HDBZW char=c added=62497 passed=0 dupes=0 processed=62497 searches=62497 retry=0
2023/10/08 14:21:45 Quit HDBZW char=e added=62835 passed=0 dupes=0 processed=62835 searches=62835 retry=0
2023/10/08 14:21:45 WAIT CLOSE_HISTORY: lock1=false=0 lock2=false=0 lock3=false=0 lock4=false=0 lock5=true=256 batchQueued=true=256 batchLocked=false=0
2023/10/08 14:21:46 WAIT CLOSE_HISTORY: lock1=false=0 lock2=false=0 lock3=false=0 lock4=false=0 lock5=true=256 batchQueued=true=256 batchLocked=false=0
2023/10/08 14:21:47 WAIT CLOSE_HISTORY: lock1=false=0 lock2=false=0 lock3=false=0 lock4=false=0 lock5=true=144 batchQueued=true=144 batchLocked=false=0
2023/10/08 14:21:48 CLOSE_HISTORY DONE
2023/10/08 14:21:48 key_add=999886 key_app=114 total=1000000 fseeks=0 eof=0 BoltDB_decodedOffsets=0 addmultioffsets=114 trymultioffsets=0 tryoffset=114 searches=1000000 inserted=1000000
2023/10/08 14:21:48 L1LOCK=1000000 | Get: L2=228 L3=1000114 | wCBBS=~992 conti=4 slept=50
2023/10/08 14:21:48 done=4000000 (took 22 seconds) (closewait 5 seconds)
2023/10/08 14:21:48 CrunchBatchLogs: did=1280 dat=1280
2023/10/08 14:21:48 CrunchLogs: BatchSize=00126:01024 t=00000029360:00000788384 µs
2023/10/08 14:21:48 Percentile 005%: BatchSize=00891:01024 t=00000029360:00000080867 µs
2023/10/08 14:21:48 Percentile 010%: BatchSize=00902:01008 t=00000038559:00000127094 µs
2023/10/08 14:21:48 Percentile 015%: BatchSize=00886:01024 t=00000041175:00000155500 µs
2023/10/08 14:21:48 Percentile 020%: BatchSize=00885:01024 t=00000062375:00000315057 µs
2023/10/08 14:21:48 Percentile 025%: BatchSize=00867:01016 t=00000062692:00000100756 µs
2023/10/08 14:21:48 Percentile 030%: BatchSize=00836:01006 t=00000068475:00000775682 µs
2023/10/08 14:21:48 Percentile 035%: BatchSize=00859:01016 t=00000108629:00000788384 µs
2023/10/08 14:21:48 Percentile 040%: BatchSize=00718:01006 t=00000031893:00000777471 µs
2023/10/08 14:21:48 Percentile 045%: BatchSize=00739:00884 t=00000032187:00000134806 µs
2023/10/08 14:21:48 Percentile 050%: BatchSize=00770:00983 t=00000032450:00000139218 µs
2023/10/08 14:21:48 Percentile 055%: BatchSize=00871:00983 t=00000036441:00000094281 µs
2023/10/08 14:21:48 Percentile 060%: BatchSize=00869:01021 t=00000035521:00000111251 µs
2023/10/08 14:21:48 Percentile 065%: BatchSize=00827:00976 t=00000039681:00000105287 µs
2023/10/08 14:21:48 Percentile 070%: BatchSize=00868:01000 t=00000041798:00000138386 µs
2023/10/08 14:21:48 Percentile 075%: BatchSize=00836:00975 t=00000041976:00000147046 µs
2023/10/08 14:21:48 Percentile 080%: BatchSize=00154:00974 t=00000045212:00000167484 µs
2023/10/08 14:21:48 Percentile 085%: BatchSize=00126:00342 t=00000101608:00000108787 µs
2023/10/08 14:21:48 Percentile 090%: BatchSize=00128:00324 t=00000101732:00000108013 µs
2023/10/08 14:21:48 Percentile 095%: BatchSize=00135:00331 t=00000101527:00000104882 µs
2023/10/08 14:21:48 Percentile 100%: BatchSize=00135:00324 t=00000101400:00000105517 µs
```

## Checking 4.000.000 `i` hashes (75% duplicates) vs hashdb
```sh
./nntp-history-test
ARGS: CPU=4/12 | jobs=4 | todo=1000000 | total=4000000 | keyalgo=11 | keylen=6 | BatchSize=1024
 useHashDB: true | IndexParallel=16
 boltOpts='&bbolt.Options{Timeout:9000000000, NoGrowSync:false, NoFreelistSync:false, PreLoadFreelist:false, FreelistType:"", ReadOnly:false, MmapFlags:0, InitialMmapSize:2147483648, PageSize:65536, NoSync:false, OpenFile:(func(string, int, fs.FileMode) (*os.File, error))(nil), Mlock:false}'
2023/10/08 14:22:28 History: new=false
  HF='history/history.dat' DB='hashdb/history.dat.hash.[0-9a-f]'
  KeyAlgo=11 KeyLen=6 NumQueueWriteChan=16
  HashDBQueues:{NumQueueIndexChan=16 NumQueueIndexChans=4 BatchSize=1024 IndexParallel=16}
2023/10/08 14:22:52 End test p=1 nntp-history added=0 dupes=249713 cachehits=547175 addretry=0 retry=0 adddupes=0 cachedupes=203112 cacheretry1=0 sum=1000000/1000000 errors=0 locked=249713
2023/10/08 14:22:52 End test p=2 nntp-history added=0 dupes=250392 cachehits=547410 addretry=0 retry=0 adddupes=0 cachedupes=202198 cacheretry1=0 sum=1000000/1000000 errors=0 locked=250392
2023/10/08 14:22:52 End test p=4 nntp-history added=0 dupes=250461 cachehits=548165 addretry=0 retry=0 adddupes=0 cachedupes=201374 cacheretry1=0 sum=1000000/1000000 errors=0 locked=250461
2023/10/08 14:22:52 End test p=3 nntp-history added=0 dupes=249434 cachehits=547102 addretry=0 retry=0 adddupes=0 cachedupes=203464 cacheretry1=0 sum=1000000/1000000 errors=0 locked=249434
2023/10/08 14:22:52 CLOSE_HISTORY: his.WriterChan <- nil
2023/10/08 14:22:52 WAIT CLOSE_HISTORY: lock1=true=1 lock2=true=1 lock3=true=16 lock4=true=16 lock5=true=256 batchQueued=false=0 batchLocked=false=0
2023/10/08 14:22:53 Quit HDBZW char=f added=0 passed=0 dupes=0 processed=0 searches=62371 retry=0
2023/10/08 14:22:53 Quit HDBZW char=9 added=0 passed=0 dupes=0 processed=0 searches=62851 retry=0
2023/10/08 14:22:53 Quit HDBZW char=1 added=0 passed=0 dupes=0 processed=0 searches=62477 retry=0
2023/10/08 14:22:53 Quit HDBZW char=4 added=0 passed=0 dupes=0 processed=0 searches=62322 retry=0
2023/10/08 14:22:53 Quit HDBZW char=5 added=0 passed=0 dupes=0 processed=0 searches=62388 retry=0
2023/10/08 14:22:53 Quit HDBZW char=0 added=0 passed=0 dupes=0 processed=0 searches=62326 retry=0
2023/10/08 14:22:53 Quit HDBZW char=2 added=0 passed=0 dupes=0 processed=0 searches=62530 retry=0
2023/10/08 14:22:53 Quit HDBZW char=6 added=0 passed=0 dupes=0 processed=0 searches=62455 retry=0
2023/10/08 14:22:53 Quit HDBZW char=7 added=0 passed=0 dupes=0 processed=0 searches=62625 retry=0
2023/10/08 14:22:53 Quit HDBZW char=3 added=0 passed=0 dupes=0 processed=0 searches=62122 retry=0
2023/10/08 14:22:53 Quit HDBZW char=a added=0 passed=0 dupes=0 processed=0 searches=62633 retry=0
2023/10/08 14:22:53 Quit HDBZW char=b added=0 passed=0 dupes=0 processed=0 searches=62471 retry=0
2023/10/08 14:22:53 Quit HDBZW char=c added=0 passed=0 dupes=0 processed=0 searches=62497 retry=0
2023/10/08 14:22:53 Quit HDBZW char=8 added=0 passed=0 dupes=0 processed=0 searches=62452 retry=0
2023/10/08 14:22:53 Quit HDBZW char=d added=0 passed=0 dupes=0 processed=0 searches=62645 retry=0
2023/10/08 14:22:53 Quit HDBZW char=e added=0 passed=0 dupes=0 processed=0 searches=62835 retry=0
2023/10/08 14:22:53 CLOSE_HISTORY DONE
2023/10/08 14:22:53 key_add=0 key_app=0 total=0 fseeks=1000000 eof=0 BoltDB_decodedOffsets=999886 addmultioffsets=0 trymultioffsets=228 tryoffset=999772 searches=1000000 inserted=0
2023/10/08 14:22:53 L1LOCK=1000000 | Get: L2=114 L3=114 | wCBBS=~1024 conti=0 slept=50
2023/10/08 14:22:53 done=4000000 (took 24 seconds) (closewait 1 seconds)
2023/10/08 14:22:53 CrunchBatchLogs: did=0 dat=0
2023/10/08 14:22:53 CrunchLogs: BatchSize=18446744073709551615:00000 t=9223372036854775807:00000000000 µs
2023/10/08 14:22:53 Percentile 005%: BatchSize=00000:00000 t=00000000000:00000000000 µs
2023/10/08 14:22:53 Percentile 010%: BatchSize=00000:00000 t=00000000000:00000000000 µs
2023/10/08 14:22:53 Percentile 015%: BatchSize=00000:00000 t=00000000000:00000000000 µs
2023/10/08 14:22:53 Percentile 020%: BatchSize=00000:00000 t=00000000000:00000000000 µs
2023/10/08 14:22:53 Percentile 025%: BatchSize=00000:00000 t=00000000000:00000000000 µs
2023/10/08 14:22:53 Percentile 030%: BatchSize=00000:00000 t=00000000000:00000000000 µs
2023/10/08 14:22:53 Percentile 035%: BatchSize=00000:00000 t=00000000000:00000000000 µs
2023/10/08 14:22:53 Percentile 040%: BatchSize=00000:00000 t=00000000000:00000000000 µs
2023/10/08 14:22:53 Percentile 045%: BatchSize=00000:00000 t=00000000000:00000000000 µs
2023/10/08 14:22:53 Percentile 050%: BatchSize=00000:00000 t=00000000000:00000000000 µs
2023/10/08 14:22:53 Percentile 055%: BatchSize=00000:00000 t=00000000000:00000000000 µs
2023/10/08 14:22:53 Percentile 060%: BatchSize=00000:00000 t=00000000000:00000000000 µs
2023/10/08 14:22:53 Percentile 065%: BatchSize=00000:00000 t=00000000000:00000000000 µs
2023/10/08 14:22:53 Percentile 070%: BatchSize=00000:00000 t=00000000000:00000000000 µs
2023/10/08 14:22:53 Percentile 075%: BatchSize=00000:00000 t=00000000000:00000000000 µs
2023/10/08 14:22:53 Percentile 080%: BatchSize=00000:00000 t=00000000000:00000000000 µs
2023/10/08 14:22:53 Percentile 085%: BatchSize=00000:00000 t=00000000000:00000000000 µs
2023/10/08 14:22:53 Percentile 090%: BatchSize=00000:00000 t=00000000000:00000000000 µs
2023/10/08 14:22:53 Percentile 095%: BatchSize=00000:00000 t=00000000000:00000000000 µs
2023/10/08 14:22:53 Percentile 100%: BatchSize=00000:00000 t=00000000000:00000000000 µs
```

## Inserting 400.000.000 `i` hashes (75% duplicates) to history and hashdb
```sh
./nntp-history-test -todo=100000000
ARGS: CPU=4/12 | useHashDB: true | jobs=4 | todo=100000000 | total=400000000 | keyalgo=11 | keylen=6 | BatchSize=1024
2023/10/04 16:24:43 History: new=true
  HF='history/history.dat' DB='hashdb/history.dat.hash.[0-9a-f]'
  KeyAlgo=11 KeyLen=6 NumQueueWriteChan=16
  HashDBQueues:{NumQueueIndexChan=16 NumQueueIndexChans=16}
...
2023/10/04 18:08:52 key_add=98845096 key_app=1154904 total=100000000 fseeks=1158418 eof=0 BoltDB_decodedOffsets=1149297 addmultioffsets=9084 trymultioffsets=9084 searches=100000000 inserted1=100000000 inserted2=0
2023/10/04 18:08:52 L1LOCK=100000000 | Get: L2=1169656 L3=1160511
2023/10/04 18:08:52 done=400000000 (took 6244 seconds) (closewait 5 seconds)
```

## Checking 400.000.000 `i` hashes (75% duplicates) vs hashdb
```sh
/nntp-history-test -todo=100000000
ARGS: CPU=4/12 | useHashDB: true | jobs=4 | todo=100000000 | total=400000000 | keyalgo=11 | keylen=6 | BatchSize=1024
2023/10/04 18:16:33 History: new=false
  HF='history/history.dat' DB='hashdb/history.dat.hash.[0-9a-f]'
  KeyAlgo=11 KeyLen=6 NumQueueWriteChan=16
  HashDBQueues:{NumQueueIndexChan=16 NumQueueIndexChans=16}
...
2023/10/04 18:51:03 key_add=0 key_app=102 total=102 fseeks=101151631 eof=0 BoltDB_decodedOffsets=99987676 addmultioffsets=1 trymultioffsets=2300824 searches=100000000 inserted1=102 inserted2=0
2023/10/04 18:51:03 L1LOCK=100000000 | Get: L2=12509 L3=12426
2023/10/04 18:51:03 done=400000000 (took 2063 seconds) (closewait 7 seconds)
```
^^ something is buggy ^^ the second run should not add or app anything ... but appended key_app=102 ?! ....
