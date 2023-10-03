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

- If desired, one could only use the `IndexChan` and avoid writing the history file. Use the full `KeyLen` (-4) of hash and provide a uniq up-counter for their Offsets.

- If the `IndexRetChan` channel is provided, it receives one of the following (int) values:
```go
  /*
  0: Indicates "not a duplicate."
  1: Indicates "duplicate."
  2: Indicates "retry later."
  */
```

## File Descriptors (max 33)
- History.dat: 1
- HashDB: 16
- DB Fseeks: 16


## Sending History Data

- To send historical data for writing, you create a HistoryObject and send it through the channel.

## Error Handling

The `History_Boot` function performs basic error checks and logs errors if the history system is already booted.

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
2023/09/29 15:45:46 History_Writer opened fp='history/history.dat' filesize=62
2023/09/29 15:45:48 RUN test p=2 nntp-history done=65722/1000000 added=0 dupes=0 cachehits=184278 retry=0 adddupes=0 cachedupes=0 cacheretry=0
2023/09/29 15:45:48 RUN test p=4 nntp-history done=67386/1000000 added=0 dupes=0 cachehits=182614 retry=0 adddupes=0 cachedupes=0 cacheretry=0
2023/09/29 15:45:48 RUN test p=1 nntp-history done=67463/1000000 added=0 dupes=0 cachehits=182537 retry=0 adddupes=0 cachedupes=0 cacheretry=0
2023/09/29 15:45:48 RUN test p=3 nntp-history done=67478/1000000 added=0 dupes=0 cachehits=182522 retry=0 adddupes=0 cachedupes=0 cacheretry=0
2023/09/29 15:45:49 RUN test p=4 nntp-history done=136933/1000000 added=0 dupes=0 cachehits=363067 retry=0 adddupes=0 cachedupes=0 cacheretry=0
2023/09/29 15:45:49 RUN test p=3 nntp-history done=130967/1000000 added=0 dupes=0 cachehits=369033 retry=0 adddupes=0 cachedupes=0 cacheretry=0
2023/09/29 15:45:49 RUN test p=2 nntp-history done=133465/1000000 added=0 dupes=0 cachehits=366535 retry=0 adddupes=0 cachedupes=0 cacheretry=0
2023/09/29 15:45:49 RUN test p=1 nntp-history done=135011/1000000 added=0 dupes=0 cachehits=364989 retry=0 adddupes=0 cachedupes=0 cacheretry=0
2023/09/29 15:45:50 RUN test p=3 nntp-history done=202699/1000000 added=0 dupes=0 cachehits=547301 retry=0 adddupes=0 cachedupes=0 cacheretry=0
2023/09/29 15:45:50 RUN test p=1 nntp-history done=201056/1000000 added=0 dupes=0 cachehits=548944 retry=0 adddupes=0 cachedupes=0 cacheretry=0
2023/09/29 15:45:50 RUN test p=2 nntp-history done=194763/1000000 added=0 dupes=0 cachehits=555237 retry=0 adddupes=0 cachedupes=0 cacheretry=0
2023/09/29 15:45:50 RUN test p=4 nntp-history done=204561/1000000 added=0 dupes=0 cachehits=545439 retry=0 adddupes=0 cachedupes=0 cacheretry=0
2023/09/29 15:45:52 End test p=1 nntp-history done=232167/1000000 added=0 dupes=0 cachehits=767833 retry=0 adddupes=0 cachedupes=0 cacheretry=0
2023/09/29 15:45:52 End test p=2 nntp-history done=222214/1000000 added=0 dupes=0 cachehits=777786 retry=0 adddupes=0 cachedupes=0 cacheretry=0
2023/09/29 15:45:52 End test p=4 nntp-history done=229147/1000000 added=0 dupes=0 cachehits=770853 retry=0 adddupes=0 cachedupes=0 cacheretry=0
2023/09/29 15:45:52 End test p=3 nntp-history done=376752/1000000 added=0 dupes=0 cachehits=623248 retry=0 adddupes=0 cachedupes=0 cacheretry=0
2023/09/29 15:45:52 History_Writer closed fp='history/history.dat' wbt=108148560 offset=108148622 wroteLines=1060280
2023/09/29 15:45:52 key_add=0 key_app=0 total=0
2023/09/29 15:45:52 done=4000000 took 6 seconds
```

```sh
./nntp-history-test -useHashDB=false -useGoCache=false
CPU=4/12 | useHashDB: false | useGoCache: false | jobs=4 | todo=1000000 | total=4000000 | keyalgo=11 | keylen=6 | BatchSize=1024
2023/09/29 15:46:20 History: HF='history/history.dat' DB='hashdb/history.dat.hash' C='<nil>' HT=11 HL=6
2023/09/29 15:46:20 History_Writer opened fp='history/history.dat' filesize=62
2023/09/29 15:46:21 RUN test p=1 nntp-history done=250000/1000000 added=0 dupes=0 cachehits=0 retry=0 adddupes=0 cachedupes=0 cacheretry=0
2023/09/29 15:46:21 RUN test p=3 nntp-history done=250000/1000000 added=0 dupes=0 cachehits=0 retry=0 adddupes=0 cachedupes=0 cacheretry=0
2023/09/29 15:46:22 RUN test p=2 nntp-history done=250000/1000000 added=0 dupes=0 cachehits=0 retry=0 adddupes=0 cachedupes=0 cacheretry=0
2023/09/29 15:46:22 RUN test p=4 nntp-history done=250000/1000000 added=0 dupes=0 cachehits=0 retry=0 adddupes=0 cachedupes=0 cacheretry=0
2023/09/29 15:46:23 RUN test p=1 nntp-history done=500000/1000000 added=0 dupes=0 cachehits=0 retry=0 adddupes=0 cachedupes=0 cacheretry=0
2023/09/29 15:46:23 RUN test p=3 nntp-history done=500000/1000000 added=0 dupes=0 cachehits=0 retry=0 adddupes=0 cachedupes=0 cacheretry=0
2023/09/29 15:46:23 RUN test p=2 nntp-history done=500000/1000000 added=0 dupes=0 cachehits=0 retry=0 adddupes=0 cachedupes=0 cacheretry=0
2023/09/29 15:46:23 RUN test p=4 nntp-history done=500000/1000000 added=0 dupes=0 cachehits=0 retry=0 adddupes=0 cachedupes=0 cacheretry=0
2023/09/29 15:46:25 RUN test p=1 nntp-history done=750000/1000000 added=0 dupes=0 cachehits=0 retry=0 adddupes=0 cachedupes=0 cacheretry=0
2023/09/29 15:46:25 RUN test p=2 nntp-history done=750000/1000000 added=0 dupes=0 cachehits=0 retry=0 adddupes=0 cachedupes=0 cacheretry=0
2023/09/29 15:46:25 RUN test p=3 nntp-history done=750000/1000000 added=0 dupes=0 cachehits=0 retry=0 adddupes=0 cachedupes=0 cacheretry=0
2023/09/29 15:46:25 RUN test p=4 nntp-history done=750000/1000000 added=0 dupes=0 cachehits=0 retry=0 adddupes=0 cachedupes=0 cacheretry=0
2023/09/29 15:46:26 End test p=1 nntp-history done=1000000/1000000 added=0 dupes=0 cachehits=0 retry=0 adddupes=0 cachedupes=0 cacheretry=0
2023/09/29 15:46:26 End test p=3 nntp-history done=1000000/1000000 added=0 dupes=0 cachehits=0 retry=0 adddupes=0 cachedupes=0 cacheretry=0
2023/09/29 15:46:26 End test p=4 nntp-history done=1000000/1000000 added=0 dupes=0 cachehits=0 retry=0 adddupes=0 cachedupes=0 cacheretry=0
2023/09/29 15:46:26 End test p=2 nntp-history done=1000000/1000000 added=0 dupes=0 cachehits=0 retry=0 adddupes=0 cachedupes=0 cacheretry=0
2023/09/29 15:46:26 History_Writer closed fp='history/history.dat' wbt=408000000 offset=408000062 wroteLines=4000000
2023/09/29 15:46:27 key_add=0 key_app=0 total=0
2023/09/29 15:46:27 done=4000000 took 7 seconds
```

## Benchmark with hashDB: `i` hashes
```sh
./nntp-history-test
CPU=4/12 | useHashDB: true | useGoCache: true | jobs=4 | todo=1000000 | total=4000000 | keyalgo=11 | keylen=6 | BatchSize=1024
2023/09/29 15:44:13 his.History_DBZinit() boltInitChan=4 boltSyncChan=1
2023/09/29 15:44:13 History: HF='history/history.dat' DB='hashdb/history.dat.hash' C='&{0xc00010cf00}' HT=11 HL=6
2023/09/29 15:44:13 History_Writer opened fp='history/history.dat' filesize=62
2023/09/29 15:44:18 RUN test p=4 nntp-history done=66125/1000000 added=64386 dupes=3148 cachehits=89941 retry=0 adddupes=1739 cachedupes=90786 cacheretry=0
2023/09/29 15:44:18 RUN test p=1 nntp-history done=63638/1000000 added=62016 dupes=3118 cachehits=89882 retry=0 adddupes=1622 cachedupes=93362 cacheretry=0
2023/09/29 15:44:18 RUN test p=3 nntp-history done=62252/1000000 added=60547 dupes=3125 cachehits=89409 retry=0 adddupes=1705 cachedupes=95214 cacheretry=0
2023/09/29 15:44:18 RUN test p=2 nntp-history done=64618/1000000 added=63051 dupes=3177 cachehits=89829 retry=0 adddupes=1567 cachedupes=92376 cacheretry=0
2023/09/29 15:44:22 RUN test p=4 nntp-history done=131546/1000000 added=128070 dupes=6360 cachehits=177400 retry=0 adddupes=3476 cachedupes=184694 cacheretry=0
2023/09/29 15:44:22 RUN test p=2 nntp-history done=128566/1000000 added=125239 dupes=6480 cachehits=177275 retry=0 adddupes=3327 cachedupes=187679 cacheretry=0
2023/09/29 15:44:22 RUN test p=1 nntp-history done=127340/1000000 added=123911 dupes=6324 cachehits=177103 retry=0 adddupes=3429 cachedupes=189233 cacheretry=0
2023/09/29 15:44:22 RUN test p=3 nntp-history done=126229/1000000 added=122780 dupes=6444 cachehits=176874 retry=0 adddupes=3449 cachedupes=190453 cacheretry=0
2023/09/29 15:44:27 RUN test p=4 nntp-history done=192911/1000000 added=187824 dupes=9069 cachehits=259693 retry=0 adddupes=5087 cachedupes=288327 cacheretry=0
2023/09/29 15:44:27 RUN test p=2 nntp-history done=195328/1000000 added=190463 dupes=9204 cachehits=260037 retry=0 adddupes=4865 cachedupes=285431 cacheretry=0
2023/09/29 15:44:27 RUN test p=1 nntp-history done=191687/1000000 added=186691 dupes=9094 cachehits=259852 retry=0 adddupes=4996 cachedupes=289367 cacheretry=0
2023/09/29 15:44:27 RUN test p=3 nntp-history done=190050/1000000 added=185022 dupes=9176 cachehits=259582 retry=0 adddupes=5028 cachedupes=291192 cacheretry=0
2023/09/29 15:44:33 End test p=1 nntp-history done=256741/1000000 added=250047 dupes=12598 cachehits=345830 retry=0 adddupes=6694 cachedupes=384831 cacheretry=0
2023/09/29 15:44:33 End test p=2 nntp-history done=257822/1000000 added=251147 dupes=12667 cachehits=345757 retry=0 adddupes=6675 cachedupes=383754 cacheretry=0
2023/09/29 15:44:33 End test p=3 nntp-history done=253099/1000000 added=246321 dupes=12705 cachehits=345426 retry=0 adddupes=6778 cachedupes=388770 cacheretry=0
2023/09/29 15:44:33 End test p=4 nntp-history done=259333/1000000 added=252485 dupes=12619 cachehits=345690 retry=0 adddupes=6848 cachedupes=382358 cacheretry=0
2023/09/29 15:44:33 Stopping History_DBZ IndexChan received nil pointer
2023/09/29 15:44:33 History_Writer closed fp='history/history.dat' wbt=102000000 offset=102000062 wroteLines=1000000
2023/09/29 15:44:33 Quit History_DBZ
2023/09/29 15:44:34 Quit HDBZW char=6 added=62455 dupes=1707 processed=64162 searches=67334 retry=0
2023/09/29 15:44:34 Quit HDBZW char=e added=62835 dupes=1703 processed=64538 searches=67721 retry=0
2023/09/29 15:44:34 Quit HDBZW char=a added=62633 dupes=1656 processed=64289 searches=67386 retry=0
2023/09/29 15:44:34 Quit HDBZW char=b added=62471 dupes=1722 processed=64193 searches=67431 retry=0
2023/09/29 15:44:34 Quit HDBZW char=5 added=62388 dupes=1699 processed=64087 searches=67263 retry=0
2023/09/29 15:44:34 Quit HDBZW char=c added=62497 dupes=1636 processed=64133 searches=67268 retry=0
2023/09/29 15:44:34 Quit HDBZW char=4 added=62322 dupes=1739 processed=64061 searches=67209 retry=0
2023/09/29 15:44:34 Quit HDBZW char=8 added=62452 dupes=1727 processed=64179 searches=67363 retry=0
2023/09/29 15:44:34 Quit HDBZW char=7 added=62625 dupes=1606 processed=64231 searches=67370 retry=0
2023/09/29 15:44:34 Quit HDBZW char=9 added=62851 dupes=1713 processed=64564 searches=67763 retry=0
2023/09/29 15:44:34 Quit HDBZW char=d added=62645 dupes=1678 processed=64323 searches=67505 retry=0
2023/09/29 15:44:34 Quit HDBZW char=3 added=62122 dupes=1730 processed=63852 searches=67024 retry=0
2023/09/29 15:44:34 Quit HDBZW char=f added=62371 dupes=1666 processed=64037 searches=67083 retry=0
2023/09/29 15:44:34 Quit HDBZW char=0 added=62326 dupes=1697 processed=64023 searches=67209 retry=0
2023/09/29 15:44:34 Quit HDBZW char=2 added=62530 dupes=1682 processed=64212 searches=67473 retry=0
2023/09/29 15:44:34 Quit HDBZW char=1 added=62477 dupes=1634 processed=64111 searches=67182 retry=0
2023/09/29 15:44:34 key_add=999886 key_app=114 total=1000000
2023/09/29 15:44:34 done=4000000 took 21 seconds
```

```sh
./nntp-history-test -useGoCache=false
CPU=4/12 | useHashDB: true | useGoCache: false | jobs=4 | todo=1000000 | total=4000000 | keyalgo=11 | keylen=6 | BatchSize=1024
2023/09/29 15:48:08 his.History_DBZinit() boltInitChan=4 boltSyncChan=1
2023/09/29 15:48:08 History: HF='history/history.dat' DB='hashdb/history.dat.hash' C='<nil>' HT=11 HL=6
2023/09/29 15:48:08 History_Writer opened fp='history/history.dat' filesize=62
2023/09/29 15:48:17 RUN test p=3 nntp-history done=249350/1000000 added=249350 dupes=650 cachehits=0 retry=0 adddupes=0 cachedupes=0 cacheretry=0
2023/09/29 15:48:21 RUN test p=4 nntp-history done=102854/1000000 added=102850 dupes=147146 cachehits=0 retry=0 adddupes=4 cachedupes=0 cacheretry=0
2023/09/29 15:48:21 RUN test p=2 nntp-history done=70701/1000000 added=70697 dupes=179299 cachehits=0 retry=0 adddupes=4 cachedupes=0 cacheretry=0
2023/09/29 15:48:21 RUN test p=1 nntp-history done=58940/1000000 added=58933 dupes=191060 cachehits=0 retry=0 adddupes=7 cachedupes=0 cacheretry=0
2023/09/29 15:48:24 RUN test p=3 nntp-history done=499350/1000000 added=499350 dupes=650 cachehits=0 retry=0 adddupes=0 cachedupes=0 cacheretry=0
2023/09/29 15:48:31 RUN test p=3 nntp-history done=749350/1000000 added=749350 dupes=650 cachehits=0 retry=0 adddupes=0 cachedupes=0 cacheretry=0
2023/09/29 15:48:35 RUN test p=4 nntp-history done=102854/1000000 added=102850 dupes=397146 cachehits=0 retry=0 adddupes=4 cachedupes=0 cacheretry=0
2023/09/29 15:48:35 RUN test p=1 nntp-history done=58940/1000000 added=58933 dupes=441060 cachehits=0 retry=0 adddupes=7 cachedupes=0 cacheretry=0
2023/09/29 15:48:35 RUN test p=2 nntp-history done=70701/1000000 added=70697 dupes=429299 cachehits=0 retry=0 adddupes=4 cachedupes=0 cacheretry=0
2023/09/29 15:48:38 End test p=3 nntp-history done=999350/1000000 added=999350 dupes=650 cachehits=0 retry=0 adddupes=0 cachedupes=0 cacheretry=0
2023/09/29 15:48:46 RUN test p=4 nntp-history done=102855/1000000 added=102851 dupes=647145 cachehits=0 retry=0 adddupes=4 cachedupes=0 cacheretry=0
2023/09/29 15:48:47 RUN test p=2 nntp-history done=70702/1000000 added=70698 dupes=679298 cachehits=0 retry=0 adddupes=4 cachedupes=0 cacheretry=0
2023/09/29 15:48:47 RUN test p=1 nntp-history done=58941/1000000 added=58934 dupes=691059 cachehits=0 retry=0 adddupes=7 cachedupes=0 cacheretry=0
2023/09/29 15:48:57 End test p=4 nntp-history done=109803/1000000 added=109799 dupes=890197 cachehits=0 retry=0 adddupes=4 cachedupes=0 cacheretry=0
2023/09/29 15:48:58 End test p=1 nntp-history done=61656/1000000 added=61649 dupes=938344 cachehits=0 retry=0 adddupes=7 cachedupes=0 cacheretry=0
2023/09/29 15:48:58 End test p=2 nntp-history done=73363/1000000 added=73359 dupes=926637 cachehits=0 retry=0 adddupes=4 cachedupes=0 cacheretry=0
2023/09/29 15:48:58 History_Writer closed fp='history/history.dat' wbt=126904014 offset=126904076 wroteLines=1244157
2023/09/29 15:48:58 Stopping History_DBZ IndexChan received nil pointer
2023/09/29 15:48:58 Quit History_DBZ
2023/09/29 15:48:59 Quit HDBZW char=3 added=77445 dupes=1 processed=77446 searches=248488 retry=0
2023/09/29 15:48:59 Quit HDBZW char=2 added=78047 dupes=1 processed=78048 searches=250120 retry=0
2023/09/29 15:48:59 Quit HDBZW char=0 added=77612 dupes=1 processed=77613 searches=249304 retry=0
2023/09/29 15:48:59 Quit HDBZW char=a added=77812 dupes=0 processed=77812 searches=250532 retry=0
2023/09/29 15:48:59 Quit HDBZW char=9 added=78184 dupes=0 processed=78184 searches=251404 retry=0
2023/09/29 15:48:59 Quit HDBZW char=c added=77658 dupes=1 processed=77659 searches=249988 retry=0
2023/09/29 15:48:59 Quit HDBZW char=5 added=78027 dupes=1 processed=78028 searches=249552 retry=0
2023/09/29 15:48:59 Quit HDBZW char=d added=78178 dupes=2 processed=78180 searches=250580 retry=0
2023/09/29 15:48:59 Quit HDBZW char=b added=77477 dupes=1 processed=77478 searches=249884 retry=0
2023/09/29 15:48:59 Quit HDBZW char=1 added=77228 dupes=0 processed=77228 searches=249908 retry=0
2023/09/29 15:48:59 Quit HDBZW char=f added=77369 dupes=1 processed=77370 searches=249484 retry=0
2023/09/29 15:48:59 Quit HDBZW char=7 added=77370 dupes=2 processed=77372 searches=250500 retry=0
2023/09/29 15:48:59 Quit HDBZW char=4 added=77795 dupes=2 processed=77797 searches=249288 retry=0
2023/09/29 15:48:59 Quit HDBZW char=8 added=77613 dupes=1 processed=77614 searches=249808 retry=0
2023/09/29 15:48:59 Quit HDBZW char=6 added=78176 dupes=0 processed=78176 searches=249820 retry=0
2023/09/29 15:48:59 Quit HDBZW char=e added=78166 dupes=1 processed=78167 searches=251340 retry=0
2023/09/29 15:48:59 key_add=1244033 key_app=124 total=1244157
2023/09/29 15:48:59 done=4000000 took 51 seconds
```

## Inserting 400.000.000 to history and hashdb
```sh
./nntp-history-test -todo 100000000
CPU=4/12 | useHashDB: true | jobs=4 | todo=100000000 | total=400000000 | keyalgo=11 | keylen=6 | BatchSize=1024
2023/10/03 02:50:30 his.History_DBZinit() boltInitChan=4 boltSyncChan=1
2023/10/03 02:50:30 History: HF='history/history.dat' DB='hashdb/history.dat.hash' KeyAlgo=11 KeyLen=6
...
2023/10/03 05:32:22 done=400000000 took 9712 seconds
```

## Message-ID Hash Splitting with BoltDB

This README explains the process of splitting Message-ID hashes and using BoltDB to organize data into 16 different databases based on the first character of the hash, and further dividing each database into buckets using the next 3 characters of the hash.

The remaining hash can be customized based on the "KeyLen" setting.

## Example

Suppose you have a Message-ID hash of "1a2b3c4d5e6f0..."

Using the described approach:

- The first character "1" selects the database "1".
- The next character "a" select the bucket "a" within the "1" database.
- The remaining hash "2b3c4d5e6f0..." can be used for further data organization within the "a" bucket based on the "KeyLen" setting.

