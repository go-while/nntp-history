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

- If the `IndexRetChan` channel is provided, it receives one of the following (int) values:
```go
  /*
  0: Indicates "not a duplicate."
  1: Indicates "duplicate."
  2: Indicates "retry later."
  */
```

## Sending History Data

- To send historical data for writing, you create a HistoryObject and send it through the channel.

## Error Handling

The `History_Boot` function performs basic error checks and logs errors if the history system is already booted.

## Contributing

Contributions to this code are welcome.

If you have suggestions for improvements or find issues, please feel free to open an issue or submit a pull request.

## License

This code is provided under the MIT License. See the [LICENSE](LICENSE) file for details.

## Benchmark with 4 parallel tests
```sh
2023/09/24 00:07:06 RUN test p=2 nntp-history done=10000/1000000 added=10000 dupes=0 cachehits=0 retry=0 adddupes=0
2023/09/24 00:07:06 RUN test p=1 nntp-history done=10000/1000000 added=10000 dupes=0 cachehits=0 retry=0 adddupes=0
2023/09/24 00:07:06 RUN test p=3 nntp-history done=10000/1000000 added=10000 dupes=0 cachehits=0 retry=0 adddupes=0
2023/09/24 00:07:06 RUN test p=4 nntp-history done=10000/1000000 added=10000 dupes=0 cachehits=0 retry=0 adddupes=0
2023/09/24 00:07:08 RUN test p=1 nntp-history done=20000/1000000 added=20000 dupes=0 cachehits=0 retry=0 adddupes=0
2023/09/24 00:07:08 RUN test p=2 nntp-history done=20000/1000000 added=20000 dupes=0 cachehits=0 retry=0 adddupes=0
2023/09/24 00:07:08 RUN test p=4 nntp-history done=20000/1000000 added=20000 dupes=0 cachehits=0 retry=0 adddupes=0
2023/09/24 00:07:08 RUN test p=3 nntp-history done=20000/1000000 added=20000 dupes=0 cachehits=0 retry=0 adddupes=0
2023/09/24 00:07:10 RUN test p=2 nntp-history done=30000/1000000 added=30000 dupes=0 cachehits=0 retry=0 adddupes=0
2023/09/24 00:07:10 RUN test p=1 nntp-history done=30000/1000000 added=30000 dupes=0 cachehits=0 retry=0 adddupes=0
2023/09/24 00:07:10 RUN test p=4 nntp-history done=30000/1000000 added=30000 dupes=0 cachehits=0 retry=0 adddupes=0
2023/09/24 00:07:10 RUN test p=3 nntp-history done=30000/1000000 added=30000 dupes=0 cachehits=0 retry=0 adddupes=0
```

## Benchmark with 6 parallel tests
```sh
2023/09/24 00:09:39 RUN test p=6 nntp-history done=10000/1000000 added=10000 dupes=0 cachehits=0 retry=0 adddupes=0
2023/09/24 00:09:39 RUN test p=1 nntp-history done=10000/1000000 added=10000 dupes=0 cachehits=0 retry=0 adddupes=0
2023/09/24 00:09:39 RUN test p=2 nntp-history done=10000/1000000 added=10000 dupes=0 cachehits=0 retry=0 adddupes=0
2023/09/24 00:09:39 RUN test p=3 nntp-history done=10000/1000000 added=10000 dupes=0 cachehits=0 retry=0 adddupes=0
2023/09/24 00:09:39 RUN test p=5 nntp-history done=10000/1000000 added=10000 dupes=0 cachehits=0 retry=0 adddupes=0
2023/09/24 00:09:39 RUN test p=4 nntp-history done=10000/1000000 added=10000 dupes=0 cachehits=0 retry=0 adddupes=0
2023/09/24 00:09:43 RUN test p=2 nntp-history done=20000/1000000 added=20000 dupes=0 cachehits=0 retry=0 adddupes=0
2023/09/24 00:09:43 RUN test p=6 nntp-history done=20000/1000000 added=20000 dupes=0 cachehits=0 retry=0 adddupes=0
2023/09/24 00:09:43 RUN test p=1 nntp-history done=20000/1000000 added=20000 dupes=0 cachehits=0 retry=0 adddupes=0
2023/09/24 00:09:43 RUN test p=5 nntp-history done=20000/1000000 added=20000 dupes=0 cachehits=0 retry=0 adddupes=0
2023/09/24 00:09:43 RUN test p=4 nntp-history done=20000/1000000 added=20000 dupes=0 cachehits=0 retry=0 adddupes=0
2023/09/24 00:09:43 RUN test p=3 nntp-history done=20000/1000000 added=20000 dupes=0 cachehits=0 retry=0 adddupes=0
2023/09/24 00:09:46 RUN test p=2 nntp-history done=30000/1000000 added=30000 dupes=0 cachehits=0 retry=0 adddupes=0
2023/09/24 00:09:46 RUN test p=6 nntp-history done=30000/1000000 added=30000 dupes=0 cachehits=0 retry=0 adddupes=0
2023/09/24 00:09:46 RUN test p=1 nntp-history done=30000/1000000 added=30000 dupes=0 cachehits=0 retry=0 adddupes=0
2023/09/24 00:09:46 RUN test p=5 nntp-history done=30000/1000000 added=30000 dupes=0 cachehits=0 retry=0 adddupes=0
2023/09/24 00:09:46 RUN test p=3 nntp-history done=30000/1000000 added=30000 dupes=0 cachehits=0 retry=0 adddupes=0
2023/09/24 00:09:46 RUN test p=4 nntp-history done=30000/1000000 added=30000 dupes=0 cachehits=0 retry=0 adddupes=0
```

## Message-ID Hash Splitting with BoltDB

This README explains the process of splitting Message-ID hashes and using BoltDB to organize data into 16 different databases based on the first character of the hash, and further dividing each database into buckets using the next 3 characters of the hash.

The remaining hash can be customized based on the "HashLen" setting.

## Example

Suppose you have a Message-ID hash of "1a2b3c4d5e6f7g8h9i0j". Using the described approach:

- The first character "1" selects the database "1".
- The next 3 characters "a2b" select the bucket "a2b" within the "1" database.
- The remaining hash "3c4d5e6f7g8h9i0j" can be used for further data organization within the "a2b" bucket based on the "HashLen" setting.

