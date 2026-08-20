package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/antoniomiletta/pengu-db/cmd/cli"
	"github.com/antoniomiletta/pengu-db/internal/pengu"
)

func main() {
	store, err := pengu.Open("data/data.db")
	if err != nil {
		log.Fatalf("pengu: %v", err)
	}
	defer store.Close()

	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()

	cli.Run(ctx, store)
}
