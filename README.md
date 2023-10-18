# nntp-history - work in progress and every commit makes it worse! xD

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

## KeyAlgo (`HashShort`)

- The standard key algorithm used is `HashShort` (-keyalgo=11).

## Database Organization

To improve processing speed and optimize data storage, we organize data into multiple BoltDB databases based on the first character of the hash:

- We create 16 separate BoltDB databases, each corresponding to one of the characters (0-9 and a-f).

- Each of these 16 DBs is further divided into 256 buckets using the 2nd and 3rd character of the hash.

## Key Structure

The remaining portion of the hash (after the first two characters) is used as the key to store offsets in the database:

- The length of this remaining hash used as a key can be customized based on the `KeyLen` setting.
- A lower `KeyLen` results in more "multioffsets" stored per key, which can be beneficial for reducing storage space.
- However, a lower `KeyLen` also result in more frequent file seeks when accessing data.
- The recommended `KeyLen` is 6. The minimum `KeyLen` is 1. The maximum `KeyLen` is the length of the hash -3.
- Reasonable values for `KeyLen` range from 4 to 6. more if you expect more than 100M messages.
- Choose wisely. You can not change `KeyLen` or `KeyAlgo` later.
```sh

*** outdated numbers as we updated code to sub.buckets and KeyIndex ***

# Test   1M inserts with `KeyLen` = 3 (`i` hashes): appoffset=29256 (~3%) trymultioffsets=57928 (~6%)
# Test  10M inserts with `KeyLen` = 3 (`i` hashes): appoffset=2467646 (24%!!) trymultioffsets=4491877 (44%!!)
# Test 100M inserts with `KeyLen` = 3 (`i` hashes): appoffset=XXXX trymultioffsets=XXXX

# Test   1M inserts with `KeyLen` = 4 (`i` hashes): appoffset=1850 (<0.2%) trymultioffsets=3698 (<0.4%)
# Test  10M inserts with `KeyLen` = 4 (`i` hashes): appoffset=183973 (<2%) trymultioffsets=365670 (<4%)
# Test 100M inserts with `KeyLen` = 4 (`i` hashes): appoffset=XXXX trymultioffsets=XXXX

# Test   1M inserts with `KeyLen` = 5 (`i` hashes): appoffset=114 trymultioffsets=228
# Test  10M inserts with `KeyLen` = 5 (`i` hashes): appoffset=11667 (~0.1%) trymultioffsets=23321 (~0.2%)
# Test 100M inserts with `KeyLen` = 5 (`i` hashes): appoffset=XXXX trymultioffsets=XXXX

# Test   1M inserts with `KeyLen` = 6 (`i` hashes): appoffset=4 trymultioffsets=8
# Test  10M inserts with `KeyLen` = 6 (`i` hashes): appoffset=748 trymultioffsets=1496
# Test 100M inserts with `KeyLen` = 6 (`i` hashes): appoffset=16511583 trymultioffsets=XXXX

# Test   1M inserts with `KeyLen` = 8 (`i` hashes): appoffset=0 trymultioffsets=0
# Test  10M inserts with `KeyLen` = 8 (`i` hashes): appoffset=4 trymultioffsets=8
# Test 100M inserts with `KeyLen` = 8 (`i` hashes): appoffset=XXXX trymultioffsets=XXXX

```

## Example

Let's illustrate this approach with an example:

Suppose you have a Message-ID hash of "1a2b3c4d5e6f0...":

- We have 16 Databases: [0-9a-f]. The first character "1" selects the database "1".
- Each database holds 256 buckets. The next character "a2" selects the bucket "a2" within the "1" database.
- The remaining hash "b3c4d5e6f0..." is used as the key in the "a2" bucket based on the specified `KeyLen`.

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
- not using L1Cache results in 4mio lines written as there are 4 jobs running
```sh
./nntp-history-test -useHashDB=false -useL1Cache=true -todo=1000000
```

## Inserting 4.000.000 `i` hashes (75% duplicates) to history and hashdb
```sh
./nntp-history-test -todo=1000000 -DBG_BS_LOG=true
...
```

## Checking 4.000.000 `i` hashes (75% duplicates) vs hashdb
```sh
./nntp-history-test -todo=1000000 -DBG_BS_LOG=false
...
```

## Inserting 400.000.000 `i` hashes (75% duplicates) to history and hashdb (adaptive batchsize)
```sh
# history.DBG_BS_LOG = true // debugs BatchLOG for every batch insert!
# history.DBG_GOB_TEST = true // costly check: test decodes gob encoded data
#
./nntp-history-test -todo 100000000
...
```


## Inserting 400.000.000 `i` hashes (75% duplicates) to history and hashdb (NO adaptive batchsize)
```sh
# history.DBG_BS_LOG = false // debugs BatchLOG for every batch insert!
# history.DBG_GOB_TEST = false // costly check: test decodes gob encoded data
#
./nntp-history-test -todo=100000000
...
...
...
```


## Checking 400.000.000 `i` hashes (75% duplicates) vs hashdb
```sh
# history.DBG_BS_LOG = true // debugs BatchLOG for every batch insert!
#
./nntp-history-test -todo=100000000
...
```

## Sizes with 25M hashes inserted
- Filesystem: ZFS@linux
- ZFS compression: lz4
- ZFS blocksize=128K
```sh

```
