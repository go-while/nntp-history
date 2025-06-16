package main

import (
	"fmt"
	"log"

	history "github.com/go-while/nntp-history"
)

func main() {
	// Example 1: Initialize with single database (Mode 0 - Default)
	fmt.Println("=== Example 1: Single Database (1 DB, 4096 tables) ===")
	his1 := &history.HISTORY{DIR: "/tmp/history_single"}
	err := his1.InitializeSingleDB()
	if err != nil {
		log.Printf("Error initializing single DB: %v", err)
	} else {
		mode, numDBs, tablesPerDB, desc := his1.GetShardingInfo()
		fmt.Printf("Initialized: Mode %d - %s\n", mode, desc)
		fmt.Printf("Databases: %d, Tables per DB: %d\n\n", numDBs, tablesPerDB)
	}

	// Example 2: Initialize with full split (Mode 1)
	fmt.Println("=== Example 2: Full Split (4096 separate databases) ===")
	his2 := &history.HISTORY{DIR: "/tmp/history_split"}
	err = his2.InitializeFullSplit()
	if err != nil {
		log.Printf("Error initializing full split: %v", err)
	} else {
		mode, numDBs, tablesPerDB, desc := his2.GetShardingInfo()
		fmt.Printf("Initialized: Mode %d - %s\n", mode, desc)
		fmt.Printf("Databases: %d, Tables per DB: %d\n\n", numDBs, tablesPerDB)
	}

	// Example 3: Initialize with 16 DBs, 256 tables each (Mode 2)
	fmt.Println("=== Example 3: Hybrid Approach (16 DBs, 256 tables each) ===")
	err = his3.Initialize16DB256Tables()
	if err != nil {
		log.Printf("Error initializing 16DB256Tables: %v", err)
	} else {
		mode, numDBs, tablesPerDB, desc := his3.GetShardingInfo()
		fmt.Printf("Initialized: Mode %d - %s\n", mode, desc)
		fmt.Printf("Databases: %d, Tables per DB: %d\n\n", numDBs, tablesPerDB)
	}

	// Example 4: Initialize with 64 DBs, 64 tables each (Mode 3)
	fmt.Println("=== Example 4: Balanced Approach (64 DBs, 64 tables each) ===")
	his4 := &history.HISTORY{DIR: "/tmp/history_64x64"}
	err = his4.Initialize64DB64Tables()
	if err != nil {
		log.Printf("Error initializing 64DB64Tables: %v", err)
	} else {
		mode, numDBs, tablesPerDB, desc := his4.GetShardingInfo()
		fmt.Printf("Initialized: Mode %d - %s\n", mode, desc)
		fmt.Printf("Databases: %d, Tables per DB: %d\n\n", numDBs, tablesPerDB)
	}

	// Example 5: Initialize with custom sharding mode
	fmt.Println("=== Example 5: Custom Sharding Mode (128 DBs, 32 tables each) ===")
	his5 := &history.HISTORY{DIR: "/tmp/history_custom"}
	err = his5.InitializeDatabaseWithSharding(false, history.SHARD_128_32)
	if err != nil {
		log.Printf("Error initializing custom sharding: %v", err)
	} else {
		mode, numDBs, tablesPerDB, desc := his5.GetShardingInfo()
		fmt.Printf("Initialized: Mode %d - %s\n", mode, desc)
		fmt.Printf("Databases: %d, Tables per DB: %d\n\n", numDBs, tablesPerDB)
	}

	// Example 6: Show all available sharding modes
	fmt.Println("=== All Available Sharding Modes ===")
	for mode := 0; mode <= 5; mode++ {
		numDBs, tablesPerDB, desc := history.GetShardConfig(mode)
		fmt.Printf("Mode %d: %s (%d DBs × %d tables = %d total tables)\n",
			mode, desc, numDBs, tablesPerDB, numDBs*tablesPerDB)
	}

	fmt.Println("\n=== Usage Notes ===")
	fmt.Println("• Mode 0: Best for most use cases, simple management")
	fmt.Println("• Mode 1: Maximum parallelism, high resource usage")
	fmt.Println("• Mode 2-5: Balanced approaches for different workloads")
	fmt.Println("• Choose based on your concurrency needs and system limits")
}
