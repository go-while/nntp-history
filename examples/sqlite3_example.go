package main

import (
	"log"
	"os"

	history "github.com/go-while/nntp-history"
)

func main() {
	// Create a temporary directory for testing
	testDir := "/tmp/nntp-history-sqlite3-test"
	os.MkdirAll(testDir, 0755)
	defer os.RemoveAll(testDir)

	// Initialize HISTORY with SQLite3
	his := &history.HISTORY{
		DIR: testDir,
	}

	// Initialize SQLite3 backend
	log.Printf("Initializing SQLite3 backend...")
	err := his.InitializeDatabase(false) // false = SQLite3, true = MySQL
	if err != nil {
		log.Fatalf("Failed to initialize SQLite3: %v", err)
	}

	log.Printf("SQLite3 backend initialized successfully!")
	log.Printf("Database location: %s/hashdb.sqlite3", testDir)

	// Test basic functionality
	if his.UsingSQLite3() {
		log.Printf("✓ SQLite3 backend is active")
	} else {
		log.Printf("✗ SQLite3 backend is not active")
	}

	// Clean shutdown
	log.Printf("Test completed successfully!")
}
