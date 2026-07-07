package main

import (
	"fmt"
	"os"

	"search-service/cmd/server"

	"github.com/spf13/cobra"
)

func main() {
	rootCmd := &cobra.Command{
		Use:          "search-service",
		Short:        "BBS search service",
		SilenceUsage: true,
	}
	rootCmd.AddCommand(server.StartCmd)

	if err := rootCmd.Execute(); err != nil {
		_, _ = fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
