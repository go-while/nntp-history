# SQLite3 RocksDB-Optimized Backend for NNTP History

This implementation provides a SQLite3 backend for the NNTP History system with RocksDB-style optimizations.

## Features

- **Always Available**: SQLite3 support is always compiled in
- **Connection Pooling**: Efficient connection pool management
- **RocksDB-style Optimizations**: WAL mode, memory mapping, large cache sizes
- **4096 Table Sharding**: Uses 3-character hex prefixes for optimal distribution
- **Batch Operations**: Optimized for high-throughput operations

## Performance Optimizations

The SQLite3 implementation includes the following optimizations:

### WAL Mode (Write-Ahead Logging)
- Enables concurrent readers and writers
- Better performance than traditional rollback journal

### Memory Optimizations
- 256MB memory mapping (`mmap_size=268435456`)
- Large page cache (`cache_size=100000`)
- Temporary tables in memory (`temp_store=memory`)

### Database Structure
- WITHOUT ROWID tables for better performance
- Indexed hash columns for fast lookups
- 4096 tables (s000 to sfff) for optimal distribution

## Building

```bash
# Simple build - SQLite3 is always included
go build

# Or using the Makefile
make build
```

## Usage Example

```go
// Initialize SQLite3 backend
his := &HISTORY{DIR: "/path/to/history"}
err := his.InitializeDatabase(false) // false = use SQLite3, true = use MySQL
if err != nil {
    log.Fatal(err)
}

// The system will automatically create the database and tables
// Location: /path/to/history/hashdb.sqlite3
```

## Configuration

The SQLite3 backend can be configured via the `SQLite3Opts` struct:

```go
opts := &SQLite3Opts{
    dbPath:   "/path/to/hashdb.sqlite3",
    maxOpen:  8,  // Connection pool size
    initOpen: 2,  // Initial connections
    timeout:  30, // Connection timeout in seconds
}
```

## Database Schema

Each of the 4096 tables follows this structure:

```sql
CREATE TABLE IF NOT EXISTS s000 (
    h CHAR(7) NOT NULL PRIMARY KEY,  -- Hash suffix (after 3-char prefix)
    o TEXT                           -- Comma-separated offsets
) WITHOUT ROWID;
```

## Performance Monitoring

The implementation includes automatic optimization:

- **Periodic VACUUM**: Runs incremental vacuum every 5 minutes
- **WAL Checkpointing**: Automatic checkpoint management
- **Statistics**: Built-in performance monitoring

## Comparison with MySQL RocksDB

| Feature | MySQL RocksDB | SQLite3 Optimized |
|---------|---------------|-------------------|
| Concurrent Connections | 64 | 8 |
| Storage Engine | RocksDB | SQLite3 + WAL |
| Memory Usage | Higher | Lower |
| Setup Complexity | Complex | Simple |
| File-based | No | Yes |
| Backup | mysqldump | File copy |

## File Structure

```
history/
├── hashdb.sqlite3          # Main SQLite3 database
├── hashdb.sqlite3-wal      # WAL file (created automatically)
└── hashdb.sqlite3-shm      # Shared memory file
```

## Dependencies

SQLite3 support requires:

```go
_ "github.com/mattn/go-sqlite3"  // SQLite3 driver
```

Install with:
```bash
go get github.com/mattn/go-sqlite3
```

## Error Handling

The implementation provides comprehensive error handling:

- Connection pool exhaustion
- Database corruption detection
- Automatic retry mechanisms
- Graceful degradation

## Thread Safety

All operations are thread-safe through:

- Connection pooling with mutexes
- Read-write locks for critical sections
- Channel-based communication

This SQLite3 implementation provides a robust, high-performance alternative to MySQL RocksDB while maintaining the same API compatibility.
