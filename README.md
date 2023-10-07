# nntp-history

nntp-history is a Go module designed for managing and storing NNTP (Network News Transfer Protocol) history records efficiently.

It provides a way to store and retrieve historical message records in a simple and scalable manner.

This module is suitable for building applications related to Usenet news servers or any system that requires managing message history efficiently.

```sh
go get github.com/go-while/nntp-history
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

```sh
./nntp-history-test -useHashDB=false -useGoCache=false
CPU=4/12 | useHashDB: false | useGoCache: false | jobs=4 | todo=1000000 | total=4000000 | keyalgo=11 | keylen=6 | BatchSize=1024
2023/09/29 15:46:20 History: HF='history/history.dat' DB='hashdb/history.dat.hash' C='<nil>' HT=11 HL=6
...
2023/09/29 15:46:26 history_Writer closed fp='history/history.dat' wbt=408000000 offset=408000062 wroteLines=4000000
2023/09/29 15:46:27 key_add=0 key_app=0 total=0
2023/09/29 15:46:27 done=4000000 took 7 seconds
```

## Benchmark with hashDB: `i` hashes
```sh
./nntp-history-test
CPU=4/12 | useHashDB: true | useGoCache: true | jobs=4 | todo=1000000 | total=4000000 | keyalgo=11 | keylen=6 | BatchSize=1024
2023/09/29 15:44:13 his.boltDB_Init() boltInitChan=4 boltSyncChan=1
2023/09/29 15:44:13 History: HF='history/history.dat' DB='hashdb/history.dat.hash' C='&{0xc00010cf00}' HT=11 HL=6
...
2023/09/29 15:44:33 history_Writer closed fp='history/history.dat' wbt=102000000 offset=102000062 wroteLines=1000000
2023/09/29 15:44:34 key_add=999886 key_app=114 total=1000000
2023/09/29 15:44:34 done=4000000 took 21 seconds
```

```sh
./nntp-history-test -useGoCache=false
CPU=4/12 | useHashDB: true | useGoCache: false | jobs=4 | todo=1000000 | total=4000000 | keyalgo=11 | keylen=6 | BatchSize=1024
2023/09/29 15:48:08 his.boltDB_Init() boltInitChan=4 boltSyncChan=1
2023/09/29 15:48:08 History: HF='history/history.dat' DB='hashdb/history.dat.hash' C='<nil>' HT=11 HL=6
...
2023/09/29 15:48:58 history_Writer closed fp='history/history.dat' wbt=126904014 offset=126904076 wroteLines=1244157
2023/09/29 15:48:59 key_add=1244033 key_app=124 total=1244157
2023/09/29 15:48:59 done=4000000 took 51 seconds
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

## Checking 400.000.000 `i` hashes (75% duplicates)
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
