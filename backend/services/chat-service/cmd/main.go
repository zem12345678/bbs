package main

import (
	"fmt"
	"os"

	"chat-service/cmd/migrate"
	"chat-service/cmd/server"

	"github.com/spf13/cobra"
)

func main() {
	root := &cobra.Command{
		Use:          "chat-service",
		Short:        "BBS chat service",
		SilenceUsage: true,
	}
	root.AddCommand(server.Command, migrate.Command)
	if err := root.Execute(); err != nil {
		_, _ = fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
