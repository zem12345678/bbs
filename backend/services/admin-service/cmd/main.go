package main

import (
	"fmt"
	"os"

	"admin/cmd/migration"
	"admin/cmd/server"

	"github.com/spf13/cobra"
)

func main() {
	rootCmd := &cobra.Command{
		Use:          "admin-service",
		Short:        "BBS admin service",
		SilenceUsage: true,
	}
	rootCmd.AddCommand(server.StartCmd, migration.MigrateCmd)

	if err := rootCmd.Execute(); err != nil {
		_, _ = fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
