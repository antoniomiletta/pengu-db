package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/antoniomiletta/pengu-db/cmd/cli"
	"github.com/antoniomiletta/pengu-db/internal/pengu"
)

func main() {
	// TODO: clean up temp files/multiple log files from compaction failure.
	path := fmt.Sprintf("%s/%s.log", "data", pengu.StampedLogFile())

	store, err := pengu.Open(path)
	if err != nil {
		log.Fatalf("failed to open store: %v", err)
	}
	defer store.Close()

	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()

	cli.Run(ctx, store)
}
