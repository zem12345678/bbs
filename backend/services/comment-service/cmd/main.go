package main

import (
	"fmt"
	"os"

	"comment-service/cmd/server"

	"github.com/spf13/cobra"
)

func main() {
	rootCmd := &cobra.Command{
		Use:          "comment-service",
		Short:        "BBS comment service",
		SilenceUsage: true,
	}
	rootCmd.AddCommand(server.StartCmd)

	if err := rootCmd.Execute(); err != nil {
		_, _ = fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
