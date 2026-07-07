package main

import (
	"fmt"
	"os"

	"feed-service/cmd/server"

	"github.com/spf13/cobra"
)

func main() {
	rootCmd := &cobra.Command{
		Use:          "feed-service",
		Short:        "BBS feed service",
		SilenceUsage: true,
	}
	rootCmd.AddCommand(server.StartCmd)

	if err := rootCmd.Execute(); err != nil {
		_, _ = fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
