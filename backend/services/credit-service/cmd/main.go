package main

import (
	"fmt"
	"os"

	"credit-service/cmd/migrate"
	"credit-service/cmd/server"

	"github.com/spf13/cobra"
)

func main() {
	rootCmd := &cobra.Command{
		Use:          "credit-service",
		Short:        "BBS credit service",
		SilenceUsage: true,
	}
	rootCmd.AddCommand(server.StartCmd, migrate.MigrateCmd)

	if err := rootCmd.Execute(); err != nil {
		_, _ = fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
