package main

import (
	"fmt"
	"os"

	"reaction-service/cmd/cache"
	"reaction-service/cmd/migration"
	"reaction-service/cmd/server"

	"github.com/spf13/cobra"
)

func main() {
	rootCmd := &cobra.Command{
		Use:          "reaction-service",
		Short:        "BBS reaction service",
		SilenceUsage: true,
	}
	rootCmd.AddCommand(server.StartCmd, migration.MigrateCmd, cache.RebuildCmd)

	if err := rootCmd.Execute(); err != nil {
		_, _ = fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
