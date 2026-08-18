package main

import (
	"log"

	"github.com/antoniomiletta/pengu-db/cmd/cli"
	"github.com/antoniomiletta/pengu-db/internal/pengu"
)

func main() {
	store, err := pengu.Open("data/data.db")
	if err != nil {
		log.Fatalf("pengu: %v", err)
	}

	cli.Run(store)
}
