# Pengu-db

An embedded, on-disk key-value storage engine written in Go. Built as a learning project, inspired by the [Bitcask paper](https://riak.com/assets/bitcask-intro.pdf).
It uses an append-only log on disk and an in-memory hash table for `O(1)` reads.

## How It Works

* **Writes (`Set` / `Delete`):** Every operation is strictly an append to the end of a log file.
You never overwrite existing data in place. Deletions are handled by appending a special `tombstone` record.

* **Reads (`Get`):** When the database opens, it replays the log once to build an in-memory index.
The index maps a string key to the exact byte offset of its value in the file. Reads require exactly one disk seek.
  
* **Garbage Collection (`Compact`):** Because you only append, deleting or updating a key leaves the old value on disk as "dead bytes".
`Compact()` cleans up space by writing only the active keys in the current log to a fresh file and atomically swapping them.

## Usage

```go
package main

import (
	"fmt"
	"log"

	pengudb "github.com/antoniomiletta/pengu-db"
)

func main() {
	// Open the database. Config is optional.
	pengu, err := pengudb.Open("./data")
	if err != nil {
		log.Fatal(err)
	}
	defer pengu.Close()

	// Set key-value pairs.
	pengu.Set([]byte("key"), []byte("value"))
	pengu.Set([]byte("example"), []byte("test"))
	pengu.Set([]byte("hey"), []byte("bye"))
	pengu.Set([]byte("hello"), []byte("world"))

	// Get a value.
	val, _ := pengu.Get([]byte("key"))
	fmt.Println(string(val))

	// List all active keys.
	for _, key := range pengu.Keys() {
		fmt.Println(string(key))
	}

	// Iterate over all active key-value pairs.
	pengu.Iter(func(key, val []byte) error {
		fmt.Printf("%s:%s\n", key, val)
		return nil
	})

	// Delete a key.
	pengu.Delete([]byte("key"))

	// Compact the log.
	pengu.Compact()
}
```
