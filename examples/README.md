# SQLite3 Sharding Examples

This directory contains examples demonstrating the different SQLite3 sharding modes available in nntp-history.

## Available Sharding Modes

| Mode | Description | Databases | Tables/DB | Total Tables | Use Case |
|------|-------------|-----------|-----------|--------------|----------|
| 0 | Single DB | 1 | 4096 | 4096 | Most use cases, simple management |
| 1 | Full Split | 4096 | 1 | 4096 | Maximum parallelism, high resources |
| 2 | 16 × 256 | 16 | 256 | 4096 | Medium-high concurrency |
| 3 | 64 × 64 | 64 | 64 | 4096 | Balanced approach |
| 4 | 128 × 32 | 128 | 32 | 4096 | High throughput scenarios |
| 5 | 512 × 8 | 512 | 8 | 4096 | Maximum table distribution |

## Running the Examples

### Sharding Demo
```bash
cd sharding_demo
go run main.go
```

This will demonstrate:
- Initialization of all 6 sharding modes
- Configuration information for each mode
- Usage patterns and recommendations

### Memory Analysis
```bash
cd memory_analysis
go run main.go
```

This will demonstrate:
- Real-time memory usage comparison
- RAM footprint for different sharding modes
- Initialization time differences
- Memory optimization recommendations

### Performance Considerations

**REAL TEST RESULTS (1 MILLION Hashes Inserted per Mode):**

| Mode | Insertion Rate (1M) | Go Heap RAM (Post GC) | SQLite Cache (Est. Max) | Initialization Time | Best For |
|------|-----------------------|-------------------------|---------------------------|---------------------|----------|
| **Mode 0** (1 DB) | ~333K/sec | ~0-1 MB | ~400 MB (Shared) | 8.55s | Read-heavy, Simplicity |
| **Mode 2** (16 DBs) | ~332K/sec | ~0-1 MB | ~112 MB (Adaptive) | **2.42s** | Balanced, Fast Init |
| **Mode 3** (64 DBs) | ~322K/sec | ~0-1 MB | ~192 MB (Adaptive) | 3.05s | Write-focused, High Concurrency |

**Key Findings (1M Hashes):**
- **Write performance is consistently high** across modes (>320K hashes/sec).
- **Go application memory usage is extremely low** (~1MB heap after GC) for all modes. SQLite manages its cache outside the Go heap.
- **Adaptive cache sizing is crucial** for multi-DB modes, keeping potential SQLite RAM usage manageable.
- **Mode 2 (16 DBs) offers the fastest initialization time** and excellent write performance, making it a strong default.
- **Mode 0 (Single DB)** is slightly faster for raw insertions in a tight loop but has significantly slower initialization. Its large shared cache is best for read-heavy workloads.

**File Descriptors:**
- Mode 0: 3-5 FDs ✅ **Recommended for simplicity or read-heavy loads**
- Mode 1: 4096+ FDs ❌ **May hit OS limits, specialized use only**
- Mode 2-5: 16-512 FDs ✅ **Manageable range, scales with DB count**

**Memory Usage (Go Heap vs. SQLite Cache):**
- The Go application's heap remains very small (e.g., ~1MB).
- SQLite's page cache is the main memory component, managed by SQLite/OS. Adaptive sizing ensures this is reasonable:
  - Mode 0: Up to ~400MB (single shared cache)
  - Mode 2: Up to ~112MB total (16 DBs * ~7MB/DB)
  - Mode 3: Up to ~192MB total (64 DBs * ~3MB/DB)
- Previous theoretical estimates were significantly higher before considering adaptive caching and SQLite's lazy allocation.

**Updated Production Recommendations (Post 1M Hash Test):**
- **General Purpose / Balanced / Fast Setup**: **Mode 2 (16 DBs)**. Offers a great balance of fast initialization, high write throughput, and efficient memory.
- **Write-Heavy / Higher Concurrency**: **Mode 3 (64 DBs)**. Provides more parallelism for writes if needed, with still manageable memory.
- **Read-Heavy / Utmost Simplicity**: **Mode 0 (1 DB)**. Best if read performance is paramount (benefits from a single large cache) and initialization time is less critical.
- **Development**: **Mode 2 (16 DBs)**. Quickest to start and provides a robust testing environment.

## Integration Examples

### Basic Usage
```go
his := &history.HISTORY{DIR: "/path/to/history"}

// Use default single database
err := his.InitializeSingleDB()

// Or use specific sharding mode
err := his.InitializeDatabaseWithSharding(false, history.SHARD_64_64)
```

### Convenience Functions
```go
// Easy initialization functions
his.InitializeSingleDB()        // Mode 0: 1 DB, 4096 tables
his.InitializeFullSplit()       // Mode 1: 4096 DBs, 1 table each
his.Initialize16DB256Tables()   // Mode 2: 16 DBs, 256 tables each
his.Initialize64DB64Tables()    // Mode 3: 64 DBs, 64 tables each
his.Initialize128DB32Tables()   // Mode 4: 128 DBs, 32 tables each
his.Initialize512DB8Tables()    // Mode 5: 512 DBs, 8 tables each
```

### Configuration Info
```go
mode, numDBs, tablesPerDB, description := his.GetShardingInfo()
fmt.Printf("Using: %s (%d DBs × %d tables)\n", description, numDBs, tablesPerDB)
```

## Choosing the Right Mode

**Mode 0 (Single DB)** - Recommended for:
- Most applications
- Simple deployment
- Limited concurrent connections
- Easy backup and maintenance

**Mode 1 (Full Split)** - Use only if:
- You have extreme parallelism requirements
- System can handle 4000+ file descriptors
- High-end hardware with abundant resources

**Mode 2-5 (Hybrid)** - Consider for:
- Medium to high concurrency applications
- Balanced resource usage
- Specific performance optimization needs
- Testing different distribution strategies

## Testing Performance

Use the [nntp-history_test](../../nntp-history_test) repository to benchmark different sharding modes:

```bash
# Test single database
./nntp-history-test -shardmode=0 -todo=1000000

# Test full split
./nntp-history-test -shardmode=1 -todo=1000000

# Test balanced approach
./nntp-history-test -shardmode=3 -todo=1000000
```

Compare throughput, latency, and resource usage to find the optimal configuration for your workload.
