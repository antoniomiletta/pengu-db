package cli

import (
	"github.com/antoniomiletta/pengu-db/internal/pengu"
	"github.com/spf13/cobra"
)

func newSetCmd(store *pengu.Store) *cobra.Command {
	return &cobra.Command{
		Use:  "set",
		Args: cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			key := args[0]
			val := args[1]

			return set(store, []byte(key), []byte(val))
		},
	}

}

func set(store *pengu.Store, key, val []byte) error {
	return store.Set([]byte(key), []byte(val))
}
