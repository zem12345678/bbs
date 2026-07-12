package main

import (
	"fmt"
	"os"

	"mall-service/cmd/migrate"
	"mall-service/cmd/server"

	"github.com/spf13/cobra"
)

func main() {
	rootCmd := &cobra.Command{
		Use:          "mall-service",
		Short:        "BBS mall service",
		SilenceUsage: true,
	}
	rootCmd.AddCommand(server.StartCmd, migrate.MigrateCmd)

	if err := rootCmd.Execute(); err != nil {
		_, _ = fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
