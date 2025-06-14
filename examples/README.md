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

**REAL TEST RESULTS (100K Hashes Inserted):**

| Mode | Insertion Rate | Memory Usage | Initialization | Best For |
|------|---------------|--------------|----------------|----------|
| **Mode 0** | 235,742/sec | 11 MB | 406ms | Read-heavy workloads |
| **Mode 2** | 268,958/sec | 11 MB | 177ms | Balanced read/write |
| **Mode 3** | 289,058/sec | 12 MB | 306ms | Write-heavy workloads |

**Key Findings:**
- **Multi-DB modes are faster for writes** (up to 23% improvement)
- **Memory usage is extremely low** (11-12 MB for all modes)
- **Mode 2 has fastest initialization** (177ms vs 406ms for Mode 0)
- **All modes are memory-safe** with adaptive cache sizing

**File Descriptors:**
- Mode 0: 3-5 FDs ✅ **Recommended for most cases**
- Mode 1: 4096+ FDs ❌ **May hit OS limits**
- Mode 2-5: 16-512 FDs ✅ **Manageable range**

**Memory Usage (Real vs Theoretical):**
- **Previous estimates were 20-200x too high** due to lazy cache allocation
- **Mode 0**: 11 MB actual (was predicted 402 MB)
- **Mode 2**: 11 MB actual (was predicted 6.4 GB)
- **Mode 3**: 12 MB actual (was predicted 25.6 GB)

**Production Recommendations:**
- **Write-heavy**: Mode 3 (289K hashes/sec)
- **Balanced**: Mode 2 (269K hashes/sec, fastest init)
- **Read-heavy**: Mode 0 (shared cache benefit)
- **Development**: Mode 2 (fast setup, good performance)
- Mode 2-5: Balanced memory usage across multiple caches

**Performance:**
- Mode 0: Best for most workloads, simple management
- Mode 1: Best for extreme parallelism with sufficient resources
- Mode 2-5: Balanced approaches for different load patterns

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
