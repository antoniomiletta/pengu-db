package pengu

import (
	"context"
	"fmt"
	"log"
	"time"
)

type CompactionWorker struct {
	store *Store
}

func NewCompactionWorker(store *Store) *CompactionWorker {
	return &CompactionWorker{
		store: store,
	}
}

func (w *CompactionWorker) StartPoll(ctx context.Context, store *Store) {
	ticker := time.NewTicker(time.Second * 10)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			fmt.Println("Shutting down worker...")
			return
		case <-ticker.C:
			w.checkRun(w.store.Journal.endOffset, w.store.Journal.activeBytes)
		}
	}
}

const minSize = 5 * 1024 * 1024

func (w *CompactionWorker) checkRun(total, active int64) {
	if total > minSize && total > active*2 {
		if err := w.store.Compact(); err != nil {
			log.Printf("\ncompaction failed: %v", err)
		}
	}
}
