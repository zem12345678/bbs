package main

import (
	"fmt"
	"os"

	"api-gateway/cmd/server"

	"github.com/spf13/cobra"
)

func main() {
	rootCmd := &cobra.Command{
		Use:          "api-gateway",
		Short:        "BBS API gateway",
		SilenceUsage: true,
	}
	rootCmd.AddCommand(server.StartCmd)

	if err := rootCmd.Execute(); err != nil {
		_, _ = fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
