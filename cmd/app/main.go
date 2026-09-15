package main

import (
	"log"

	pengudb "github.com/antoniomiletta/pengu-db"
)

func main() {
	pengu, err := pengudb.Open("./data")
	if err != nil {
		log.Fatalf("failed to open store: %v", err)
	}
	defer pengu.Close()

	pengu.Set([]byte("key"), []byte("value"))
	pengu.Set([]byte("key1"), []byte("value"))
	pengu.Set([]byte("key2"), []byte("value"))
	pengu.Set([]byte("key3"), []byte("value"))

	for _, k := range pengu.Keys() {
		println(string(k))
	}
}
