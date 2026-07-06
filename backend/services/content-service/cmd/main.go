package main

import (
	"fmt"
	"os"

	"content-service/cmd/migration"
	"content-service/cmd/server"

	"github.com/spf13/cobra"
)

func main() {
	rootCmd := &cobra.Command{
		Use:          "content-service",
		Short:        "BBS content service",
		SilenceUsage: true,
	}
	rootCmd.AddCommand(server.StartCmd, migration.MigrateCmd)

	if err := rootCmd.Execute(); err != nil {
		_, _ = fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
