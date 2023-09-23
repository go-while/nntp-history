# nntp-history

nntp-history is a Go module designed for managing and storing NNTP (Network News Transfer Protocol) history records efficiently.

It provides a way to store and retrieve historical message records in a simple and scalable manner.

This module is suitable for building applications related to Usenet news servers or any system that requires managing message history efficiently.

```sh
go get github.com/go-while/nntp-history
```

# Before running the test application, you can configure its behavior by modifying the test/nntp-history-test.go file.


# Code Hints

## History_Boot Function

The `History_Boot` function in this Go code is responsible for initializing and booting a history management system.

It provides essential configuration options and prepares the system for historical data storage and retrieval.

## Usage

To use the `History_Boot` function, follow these steps:

1. Call the `History_Boot` function with the desired configuration options.

2. The history management system will be initialized and ready for use.

# history.HISTORY_WRITER_CHAN

- `HISTORY_WRITER_CHAN` is a Go channel used for sending and processing historical data entries.

- It is primarily responsible for writing data to a historical data storage system, using a HashDB (BoltDB) to avoid duplicate entries.

# history.History.IndexChan

- `history.History.IndexChan` is a Go channel used for checking message-ID hashs for duplicates in history file.

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
2023/09/23 21:38:48 RUN test p=1 nntp-history done=10000/1000000 added=20000 dupes=0 cachehits=0 retry=0 adddupes=0
2023/09/23 21:38:48 RUN test p=4 nntp-history done=10000/1000000 added=20000 dupes=0 cachehits=0 retry=0 adddupes=0
2023/09/23 21:38:48 RUN test p=2 nntp-history done=10000/1000000 added=20000 dupes=0 cachehits=0 retry=0 adddupes=0
2023/09/23 21:38:48 RUN test p=3 nntp-history done=10000/1000000 added=20000 dupes=0 cachehits=0 retry=0 adddupes=0
2023/09/23 21:38:51 RUN test p=1 nntp-history done=20000/1000000 added=40000 dupes=0 cachehits=0 retry=0 adddupes=0
2023/09/23 21:38:51 RUN test p=3 nntp-history done=20000/1000000 added=40000 dupes=0 cachehits=0 retry=0 adddupes=0
2023/09/23 21:38:51 RUN test p=4 nntp-history done=20000/1000000 added=40000 dupes=0 cachehits=0 retry=0 adddupes=0
2023/09/23 21:38:51 RUN test p=2 nntp-history done=20000/1000000 added=40000 dupes=0 cachehits=0 retry=0 adddupes=0
2023/09/23 21:38:53 RUN test p=1 nntp-history done=30000/1000000 added=60000 dupes=0 cachehits=0 retry=0 adddupes=0
2023/09/23 21:38:53 RUN test p=3 nntp-history done=30000/1000000 added=60000 dupes=0 cachehits=0 retry=0 adddupes=0
2023/09/23 21:38:53 RUN test p=4 nntp-history done=30000/1000000 added=60000 dupes=0 cachehits=0 retry=0 adddupes=0
2023/09/23 21:38:53 RUN test p=2 nntp-history done=30000/1000000 added=60000 dupes=0 cachehits=0 retry=0 adddupes=0
```
