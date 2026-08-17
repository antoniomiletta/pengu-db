package main

import (
	"log"

	"github.com/antoniomiletta/pengu-db/pengu"
)

func main() {
	store, err := pengu.Open("data/data.db")
	if err != nil {
		log.Fatalf("pengu: %v", err)
	}

	store.Set([]byte("hello"), []byte("goodbye"))

	val, err := store.Get([]byte("hello"))
	println(string(val))

	store.Delete([]byte("hello"))

	println(string(val))
}
