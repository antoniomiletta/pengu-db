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

	val, _ := pengu.Get([]byte("key"))
	println(string(val))

	pengu.Delete([]byte("key"))
}
