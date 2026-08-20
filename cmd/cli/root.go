package cli

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/antoniomiletta/pengu-db/internal/pengu"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
)

func Run(ctx context.Context, store *pengu.Store) {
	root := newRootCmd(store)
	scanner := bufio.NewScanner(os.Stdin)

	// Scan in parallel so it doesn't block ctx.Done() until enter pressed.
	inputCh := make(chan string)
	go func() {
		for scanner.Scan() {
			inputCh <- scanner.Text()
		}

		if err := scanner.Err(); err != nil {
			return
		}
	}()

	for {
		fmt.Print("pengu-# ")

		select {
		case <-ctx.Done():
			return
		case line := <-inputCh:
			if line == "" {
				continue
			}

			if line == "exit" {
				break
			}

			resetFlags(root)
			root.SetArgs(strings.Fields(line))

			if err := root.ExecuteContext(ctx); err != nil {
				fmt.Printf("pengu-cli: %v\n", err)
			}
		}
	}
}

func newRootCmd(store *pengu.Store) *cobra.Command {
	root := &cobra.Command{
		Use:           "repl",
		SilenceUsage:  true,
		SilenceErrors: true,
	}

	root.AddCommand(newSetCmd(store))
	root.AddCommand(newGetCmd(store))
	root.AddCommand(newDeleteCmd(store))
	root.AddCommand(newDisplayCmd(store))

	return root
}

func resetFlags(cmd *cobra.Command) {
	cmd.Flags().VisitAll(func(f *pflag.Flag) {
		f.Value.Set(f.DefValue)
		f.Changed = false
	})
	for _, c := range cmd.Commands() {
		resetFlags(c)
	}
}
