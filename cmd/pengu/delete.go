package main

import (
	"github.com/antoniomiletta/pengu-db"
	"github.com/spf13/cobra"
)

func newDeleteCmd(store *pengu.Store) *cobra.Command {
	return &cobra.Command{
		Use:  "delete",
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			key := []byte(args[0])
			return del(store, key)
		},
	}
}

func del(store *pengu.Store, key []byte) error {
	return store.Delete(key)
}
