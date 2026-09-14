package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/signal"
	"sync"
	"syscall"

	"github.com/antoniomiletta/pengu-db"
)

func main() {
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()

	datadir := "data"

	if err := os.MkdirAll(datadir, 0o755); err != nil {
		log.Fatalf("failed to create path: %v", err)
	}

	latestLog := filterObsolete(datadir)
	target := fmt.Sprintf("%s/%s", datadir, latestLog)

	store, err := pengu.Open(target)
	if err != nil {
		log.Fatalf("failed to open store: %v", err)
	}
	defer store.Close()

	// TODO: cleanup leftover files from compaction crash.
	compactionWorker := pengu.NewCompactionWorker(store)

	var wg sync.WaitGroup
	defer wg.Wait()

	wg.Go(func() {
		compactionWorker.StartPoll(ctx, store)
	})

	Execute(ctx, store)
	cancel() // cancel context after repl returns so worker doesnt block shutdown
}

func filterObsolete(datadir string) string {
	logs, err := os.ReadDir(datadir)
	if err != nil {
		log.Fatalf("failed to read data directory: %v", err)
	}

	if len(logs) == 0 {
		return fmt.Sprintf("%s.log", pengu.StampedLogFile())
	}

	var latest string
	for _, log := range logs {
		if log.Name() > latest {
			latest = log.Name()
		}
	}

	return latest
}
