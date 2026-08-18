package cli

import (
	"github.com/antoniomiletta/pengu-db/internal/pengu"
	"github.com/spf13/cobra"
)

func newGetCmd(store *pengu.Store) *cobra.Command {
	return &cobra.Command{
		Use:  "get",
		Args: cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			key := []byte(args[0])
			return get(store, key)
		},
	}
}

func get(store *pengu.Store, key []byte) error {
	val, err := store.Get(key)
	if err != nil {
		return err
	}
	println(string(val))

	return nil
}
