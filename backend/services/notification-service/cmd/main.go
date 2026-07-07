package main

import (
	"fmt"
	"os"

	"notification-service/cmd/migrate"
	"notification-service/cmd/server"

	"github.com/spf13/cobra"
)

func main() {
	rootCmd := &cobra.Command{
		Use:          "notification-service",
		Short:        "BBS notification service",
		SilenceUsage: true,
	}
	rootCmd.AddCommand(server.StartCmd, migrate.MigrateCmd)

	if err := rootCmd.Execute(); err != nil {
		_, _ = fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
