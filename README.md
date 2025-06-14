# nntp-history - High-Performance NNTP History Management

nntp-history is a Go module designed for managing and storing NNTP (Network News Transfer Protocol) history records efficiently with support for both MySQL RocksDB and SQLite3.

It provides a scalable way to store and retrieve historical message records with duplicate detection and high-throughput capabilities.

This module is suitable for building applications related to Usenet news servers or any system that requires managing message history efficiently.

## 🚀 Features

- **Dual Backend Support**: MySQL RocksDB and SQLite3 with RocksDB-style optimizations
- **High Performance**: Optimized for millions of records with minimal latency
- **Duplicate Detection**: Efficient hash-based duplicate checking with L1 cache
- **Connection Pooling**: Efficient database connection management
- **Concurrent Processing**: Multi-threaded design for high throughput
- **Memory Mapped I/O**: SQLite3 backend uses memory mapping for performance

## 📦 Installation

```sh
go get github.com/go-while/nntp-history
```

## 🔧 Database Backends

### SQLite3 (Recommended for most use cases)
- **File-based**: Simple deployment and backup
- **RocksDB Optimizations**: WAL mode, memory mapping, large caches
- **4096 Table Sharding**: Optimal distribution using 3-character hex prefixes
- **Always Available**: No build tags required

### MySQL RocksDB (Enterprise use cases)
- **Distributed**: Suitable for cluster environments
- **High Concurrency**: Supports more concurrent connections
- **Complex Setup**: Requires MySQL with RocksDB storage engine

## 🏗️ Quick Start

### Initialize with SQLite3 (Default)
```go
import "github.com/go-while/nntp-history"

his := &history.HISTORY{DIR: "/path/to/history"}
err := his.InitializeDatabase(false) // false = SQLite3
if err != nil {
    log.Fatal(err)
}
```

### Initialize with MySQL RocksDB
```go
his := &history.HISTORY{DIR: "/path/to/history"}
err := his.InitializeDatabase(true) // true = MySQL
if err != nil {
    log.Fatal(err)
}
```

## 📊 Performance Comparison & Sharding

The `nntp-history` module supports multiple SQLite3 sharding strategies to optimize for different workloads. Detailed benchmarks with 1 million hash insertions show:

| Sharding Mode | Insertion Rate (1M) | Go Heap RAM (Post GC) | SQLite Cache (Est. Max Potential) | Initialization Time | Key Characteristics |\n|-----------------|-----------------------|-------------------------|-----------------------------------|---------------------|-----------------------|\n| **Mode 0** (1 DB, 4096 tables) | ~333K/sec | ~0-1 MB | ~400 MB (Shared) | 8.55s | Simplicity, best for read-heavy (large shared cache) |\n| **Mode 2** (16 DBs, 256 tables) | ~332K/sec | ~0-1 MB | ~112 MB (Adaptive) | **2.42s** | **Recommended Default**: Fast init, balanced R/W, good concurrency |\n| **Mode 3** (64 DBs, 64 tables) | ~322K/sec | ~0-1 MB | ~192 MB (Adaptive) | 3.05s | Write-focused, higher concurrency needs |\n

**Key Takeaways from 1M Hash Tests:**
- **High Write Performance**: All tested modes exceed 320,000 inserts/second.
- **Low Go Application RAM**: Go's heap usage is minimal (~1MB after GC) due to SQLite's memory management.
- **Adaptive SQLite Cache**: SQLite's page cache is managed efficiently. Multi-DB modes use adaptive sizing (e.g., Mode 2 estimates up to 112MB total cache if fully utilized, Mode 3 up to 192MB). This is OS-managed memory, not Go heap.
- **Fast Initialization for Multi-DB**: Mode 2 (16 DBs) initializes significantly faster than Mode 0 (single DB with many tables).

**Recommendation:**
- **Mode 2 (16 DBs, 256 tables each)** is recommended for most use cases, offering a strong balance of fast startup, high throughput, and efficient memory utilization.
- Use Mode 0 if simplicity is paramount or for predominantly read-heavy workloads that benefit from a single, large shared cache.
- Consider Mode 3 or other multi-DB options for very specific high-concurrency write scenarios.

Refer to `examples/README.md` and `sqlite-sharded-integration-log.md` for more detailed analysis.

## 📁 Related Repositories

- **Test Suite**: [nntp-history_test](https://github.com/go-while/nntp-history_test) - Comprehensive testing and benchmarking
- **Examples**: See `examples/` directory for usage examples

## 🧪 Testing & Benchmarking

For comprehensive testing and benchmarking, use the dedicated test repository:
**[nntp-history_test](https://github.com/go-while/nntp-history_test)**

### Quick Benchmark Examples

```bash
# Clone the test repository
git clone https://github.com/go-while/nntp-history_test
cd nntp-history_test

# Build and run basic test
./build.sh
./nntp-history-test -todo=100000

# Performance test with SQLite3
./nntp-history-test -backend=sqlite3 -todo=1000000

# High-volume duplicate detection test
./nntp-history-test -todo=1000000 -DBG_BS_LOG=true
```

### Performance Notes
- Performance varies based on hardware, database configuration, and workload patterns
- Both SQLite3 and MySQL RocksDB backends are optimized for high-throughput duplicate detection
- Run your own benchmarks with the test repository for accurate measurements on your system
- See the [nntp-history_test](../nntp-history_test) repository for comprehensive testing tools

## � Storage Size Estimates

Based on the actual code structure, here are precise storage calculations for planning:

### 📄 **history.dat File Size**

Each record in history.dat has a fixed format:
```
{sha256hash}\ttimestamp~expires~date\tstoragetoken\n
```

**Per record breakdown:**
- `{` + `}` = 2 bytes (hash wrapper)
- SHA256 hash = 64 bytes
- `\t` = 1 byte (tab)
- Arrival timestamp = 10 bytes (zero-padded)
- `~` = 1 byte
- Expires = 10 bytes (zero-padded or `----------`)
- `~` = 1 byte
- Date = 10 bytes (zero-padded)
- `\t` = 1 byte (tab)
- StorageToken = 1 byte (typically "F")
- `\n` = 1 byte (newline)

**Total per record: 102 bytes**

### 🗄️ **Database Size (SQLite3 with keylen=7)**

**Table Structure:**
- 4096 tables (s000 to sfff) using 3-character hex prefixes
- Each table stores: 7-char key + comma-separated offsets
- Distribution: ~244 records per table (1M ÷ 4096)

**Storage Components:**
- Keys storage: ~7 bytes per record
- Offsets storage: ~8 bytes per record (low collision rate)
- SQLite overhead: ~12 bytes per record
- Table/index overhead: ~5 MB total

### 📊 **Size Summary**

| Records | history.dat | SQLite3 DB | Total Storage |
|---------|-------------|-------------|---------------|
| **1M** | **102 MB** | **32 MB** | **134 MB** |
| **10M** | **1.02 GB** | **320 MB** | **1.34 GB** |
| **100M** | **10.2 GB** | **3.2 GB** | **13.4 GB** |

### 🔍 **Key Performance Factors**

- **keylen=7**: Uses first 10 chars (3 for table + 7 for key)
- **Low Collision Rate**: 10-char prefixes provide excellent distribution
- **Efficient Storage**: SQLite3 WITHOUT ROWID tables minimize overhead
- **Linear Scalability**: Storage grows predictably with record count

## �🔨 Building

```bash
# Simple build (SQLite3 included by default)
go build

# Using Makefile
make build

# Get dependencies
make deps
```

## 📖 Documentation

- **SQLite3 Backend**: See [SQLITE3_README.md](SQLITE3_README.md) for detailed SQLite3 documentation
- **API Reference**: See inline documentation in source files

## Code Hints

## BootHistory Function

The `BootHistory` function in this Go code is responsible for initializing and booting a history management system.

It provides essential configuration options and prepares the system for historical data storage and retrieval.

## Usage

To use the `BootHistory` function, follow these steps:

1. Call the `BootHistory` function with the desired configuration options.

2. The history management system will be initialized and ready for use.

## history.History.WriterChan

- `history.History.WriterChan` is a Go channel used for sending and processing historical data entries.

- It is primarily responsible for writing data to a historical data storage system, using a HashDB (SQLite3/MySQL) to avoid duplicate entries.

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

- When the offset is set to -1, the channel performs a check for duplicate hashes but does not add the hash to the database.

- When the offset is set to a value greater than zero, the channel functions as a mechanism for adding these message-ID hashes to the database.

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

# Message-ID Hash Distribution with SQLite3

## KeyAlgo (`HashShort`)

- The standard key algorithm used is `HashShort` (-keyalgo=11).

## Database Organization

To improve processing speed and optimize data storage, we organize data using SQLite3 table sharding based on hash prefixes:

- We create 4096 separate tables (s000 to sfff), each corresponding to a 3-character hexadecimal prefix.

- This provides optimal distribution and parallel access for high-performance operations.

## Key Structure

The hash distribution uses a sophisticated sharding approach:

- The first 3 characters of the hash determine the table (s000-sfff)
- The next 7 characters (or `KeyLen` setting) are used as the key within that table
- The recommended `KeyLen` is 7. The minimum `KeyLen` is 1. The maximum `KeyLen` is the length of the hash -3.
- Reasonable values for `KeyLen` range from 4 to 7. Use higher values if you expect more than 100M messages.
- Choose wisely. You can not change `KeyLen` after database creation.
```sh

*** These are outdated benchmarks from the previous BoltDB implementation ***
*** SQLite3 backend provides better performance and simpler management ***

# With SQLite3 sharding (4096 tables), collision rates are significantly lower:
# - keylen=7: Optimal balance of performance and storage efficiency
# - keylen=4-6: Higher collision rates, more multi-offset storage
# - keylen=8+: Near-zero collisions, maximum storage usage

# Run your own benchmarks with nntp-history_test for current performance metrics

```

## Example

Let's illustrate this approach with an example:

Suppose you have a Message-ID hash of "1a2b3c4d5e6f0...":

- We use the first 3 characters "1a2" to select the SQLite3 table "s1a2"
- The next 7 characters "b3c4d5e" (based on `KeyLen=7`) are used as the key within that table
- The remaining hash is stored as comma-separated offsets for that key

By following this approach, you can efficiently organize and retrieve data based on Message-ID hashes while benefiting from the performance and storage optimizations provided by SQLite3 with RocksDB-style optimizations.

Feel free to customize the `KeyLen` setting to meet your specific performance and storage requirements.

Smaller `KeyLen` values save space but may result in more collisions and multi-offset storage, while larger `KeyLen` values reduce collisions but consume more space.


## File Descriptors

SQLite3 backend uses significantly fewer file descriptors:

- Writer History.dat: 1
- SQLite3 Database: 1 (main database file)
- SQLite3 WAL: 1 (write-ahead log)
- **Total**: ~3-5 file descriptors (much more efficient than the previous 33 FDs)

```sh
# Monitor open file descriptors
ls -lha /proc/$(pidof nntp-history-test)/fd

# Count all open FDs
ls -l /proc/$(pidof nntp-history-test)/fd | wc -l

# Count open history.dat
ls -lha /proc/$(pidof nntp-history-test)/fd|grep history.dat$|wc -l

# Count open SQLite3 database files
ls -lha /proc/$(pidof nntp-history-test)/fd|grep "\.sqlite3"|wc -l
```


## SQLite3 Database Statistics

When you retrieve and examine the statistics of a SQLite3 database in Go, you can gather valuable information about the database's performance, resource usage, and structure. The statistics provide insights into how the database is operating and can be useful for optimizing your application. Here are some of the key metrics you can see from SQLite3 database statistics:

**Database Size Metrics:**
- Total database file size in bytes
- Number of tables and their individual sizes
- WAL (Write-Ahead Log) file size and checkpoint frequency
- Page count and page utilization

**Performance Metrics:**
- Cache hit/miss ratios for database pages
- Query execution times and optimization statistics
- Transaction throughput and commit rates
- Lock contention and wait times

**Storage Efficiency:**
- Table and index storage utilization
- Fragmentation levels and vacuum effectiveness
- Compression ratios (if enabled)
- Free space and page reuse statistics

**Connection and Concurrency:**
- Active connection count
- Concurrent reader/writer statistics
- Transaction isolation levels and conflicts
- Checkpoint frequency and WAL size management

**SQLite3-Specific Features:**
- Memory-mapped I/O effectiveness
- Journal mode efficiency (WAL vs. DELETE)
- PRAGMA settings impact on performance
- Query planner statistics and index usage

These statistics can help you monitor and optimize your SQLite3 database. For example, you can use them to identify performance bottlenecks, understand resource usage patterns, and assess the efficiency of your data organization. The SQLite3 backend in nntp-history is optimized with RocksDB-style settings including WAL mode, large caches, and memory mapping for maximum performance.


## Contributing

Contributions to this code are welcome.

If you have suggestions for improvements or find issues, please feel free to open an issue or submit a pull request.

## License

This code is provided under the MIT License. See the [LICENSE](LICENSE) file for details.



## Benchmark pure writes (no dupe check via database) to history file with 4K bufio.
- Not using L1Cache results in 4M lines written as there are 4 jobs running
```sh
./nntp-history-test -useHashDB=false -useL1Cache=true -todo=1000000
```

## Inserting 4,000,000 hashes (75% duplicates) to history and database
```sh
./nntp-history-test -todo=1000000 -DBG_BS_LOG=true
...
```

## Checking 4,000,000 hashes (75% duplicates) vs database
```sh
./nntp-history-test -todo=1000000 -DBG_BS_LOG=false
...
```

## Inserting 400,000,000 hashes (75% duplicates) to history and database (adaptive batchsize)
```sh
# history.DBG_BS_LOG = true // debugs BatchLOG for every batch insert!
# history.DBG_GOB_TEST = true // costly check: test decodes gob encoded data
#
./nntp-history-test -todo 100000000
...
```

## Inserting 400,000,000 hashes (75% duplicates) to history and database (NO adaptive batchsize)
```sh
# history.DBG_BS_LOG = false // debugs BatchLOG for every batch insert!
# history.DBG_GOB_TEST = false // costly check: test decodes gob encoded data
#
./nntp-history-test -todo=100000000
...
...
...
```

## Checking 400,000,000 hashes (75% duplicates) vs database
```sh
# history.DBG_BS_LOG = true // debugs BatchLOG for every batch insert!
#
./nntp-history-test -todo=100000000
...
```

## Database sizes with 25M hashes inserted
- Filesystem: ZFS@linux
- ZFS compression: lz4
- ZFS blocksize=128K

**SQLite3 Backend:**
```sh
# Database file sizes (compressed)
du -h hashdb.sqlite3     # ~2.1GB
du -h hashdb.sqlite3-wal # ~15MB
```

**MySQL RocksDB Backend:**
```sh
# Database and index sizes
SELECT table_name, data_length, index_length
FROM information_schema.tables
WHERE table_schema = 'nntp_history';
```
