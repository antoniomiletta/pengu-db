package cli

import (
	"github.com/antoniomiletta/pengu-db/internal/pengu"
	"github.com/spf13/cobra"

	"bufio"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"
)

func newDisplayCmd(store *pengu.Store) *cobra.Command {
	cmd := &cobra.Command{
		Use:  "dr",
		Args: cobra.MinimumNArgs(0),
		RunE: func(cmd *cobra.Command, args []string) error {
			alive, _ := cmd.Flags().GetBool("alive")
			if alive {
				return displayAlive(store)
			}
			return display(store)
		},
	}

	cmd.Flags().BoolP("alive", "a", false, "display alive")
	return cmd
}

func displayAlive(store *pengu.Store) error {
	// Print header
	fmt.Printf("%-20s | %-20s | %s\n", "KEY", "VALUE", "SIZE")
	fmt.Println(strings.Repeat("-", 50))

	var count int

	displayFn := func(key, val []byte) error {
		displayKey := fmt.Sprintf("%q", key)
		displayVal := fmt.Sprintf("%q", val)

		// Truncate
		if len(displayKey) > 20 {
			displayKey = displayKey[:17] + "..."
		}
		if len(displayVal) > 20 {
			displayVal = displayVal[:17] + "..."
		}

		sizeStr := fmt.Sprintf("%d B", len(val))

		// Print row
		fmt.Printf("%-20s | %-20s | %s\n", displayKey, displayVal, sizeStr)

		count++
		return nil
	}

	if err := store.Iter(displayFn); err != nil {
		return err
	}

	// Print footer
	fmt.Println(strings.Repeat("-", 50))
	fmt.Printf("Total Active Keys: %d\n", count)

	return nil
}

func display(store *pengu.Store) error {
	// TODO: check for a better approach than opening a new file handle
	file, err := os.Open(store.Journal.Path)
	if err != nil {
		return fmt.Errorf("failed to open file for debugging: %w", err)
	}
	defer file.Close()

	// Print header
	fmt.Printf("%-10s | %-6s | %-4s | %-20s | %-20s | %s\n",
		"OFFSET", "ACTION", "CRC", "KEY", "VALUE", "SIZE")
	fmt.Println(strings.Repeat("-", 80))

	reader := bufio.NewReader(file)
	var offset int64 = 0
	var count int

	for {
		rec, n, err := pengu.Decode(reader)
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			fmt.Printf("\n[!] Corrupted record at offset %d: %v\n", offset, err)
			break
		}

		action := "SET"
		if rec.Typ == pengu.TypeTombstone {
			action = "DELETE"
		}

		displayKey := fmt.Sprintf("%q", rec.Key)
		displayVal := fmt.Sprintf("%q", rec.Val)
		sizeStr := fmt.Sprintf("%d B", len(rec.Val))

		// Truncate
		if rec.Typ == pengu.TypeTombstone {
			displayVal = "<tombstone>"
			sizeStr = "0 B"
		} else if len(displayVal) > 20 {
			displayVal = displayVal[:17] + "..."
		}

		if len(displayKey) > 20 {
			displayKey = displayKey[:17] + "..."
		}

		// Print row
		fmt.Printf("%-10d | %-6s | %-4s | %-20s | %-20s | %s\n",
			offset, action, "OK", displayKey, displayVal, sizeStr)

		offset += int64(n)
		count++
	}

	// 4. Print the footer
	fmt.Println(strings.Repeat("-", 80))
	fmt.Printf("Total Records: %d | Total File Size: %d bytes\n", count, offset)

	return nil
}
