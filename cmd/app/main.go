package main

import (
	"fmt"
	"log"

	pengudb "github.com/antoniomiletta/pengu-db"
)

func main() {
	// opens the db and replays the log file to build the in-memory index.
	pengu, err := pengudb.Open("./data", pengudb.StoreConfig{
		CompactionMinSize:   0,
		CompactionDeadRatio: 0,
	})
	if err != nil {
		log.Fatalf("failed to open pengu: %v", err)
	}
	defer pengu.Close()

	// appends record
	pengu.Set([]byte("key"), []byte("value"))
	pengu.Set([]byte("example"), []byte("test"))
	pengu.Set([]byte("hey"), []byte("bye"))
	pengu.Set([]byte("hello"), []byte("world"))

	// returns the value for the key. index lookup + single disk seek
	val, _ := pengu.Get([]byte("key"))
	fmt.Println("-- Get():")
	fmt.Println(string(val))

	// lists all active keys
	fmt.Println("-- Keys():")
	for _, k := range pengu.Keys() {
		fmt.Println(string(k))
	}

	// executes fn for each key-val pair mapped in the index.
	fmt.Println("-- Iter():")
	pengu.Iter(func(key, val []byte) error {
		fmt.Printf("%s:%s\n", string(key), string(val))
		return nil
	})

	// appends tombstone and removes from index.
	pengu.Delete([]byte("key"))

	// attempts to compact the log file; no-op if the file is too small
	// or the dead byte ratio does not meet the threshold.
	if err := pengu.Compact(); err != nil {
		log.Printf("failed to compact: %v", err)
	}
}
