package cli

import (
	"bufio"
	"fmt"
	"os"
	"strings"

	"github.com/antoniomiletta/pengu-db/internal/pengu"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
)

func Run(store *pengu.Store) {
	scanner := bufio.NewScanner(os.Stdin)
	root := newRootCmd(store)

	for {
		print("pengu-# ")
		if !scanner.Scan() {
			break
		}
		line := strings.TrimSpace(scanner.Text())

		if line == "" {
			continue
		}

		if line == "exit" {
			break
		}

		resetFlags(root)
		root.SetArgs(strings.Fields(line))

		if err := root.Execute(); err != nil {
			fmt.Printf("pengu-cli: %v\n", err)
		}
	}

	if err := scanner.Err(); err != nil {
		fmt.Fprintln(os.Stderr, "pengu-cli: input error: ", err)
		os.Exit(1)
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
