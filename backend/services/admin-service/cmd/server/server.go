package server

import (
	"context"
	"fmt"

	"github.com/spf13/cobra"
)

var (
	configFile string
	serverApp  *App

	StartCmd = &cobra.Command{
		Use:          "server",
		Short:        "Start admin gRPC server",
		Example:      "admin-service server -c configs/config.yaml",
		SilenceUsage: true,
		PreRun: func(cmd *cobra.Command, args []string) {
			tip()
			setup()
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			return run()
		},
	}
)

func init() {
	StartCmd.PersistentFlags().StringVarP(&configFile, "config", "c", "configs/config.yaml", "Start server with provided configuration file")
}

func tip() {
	fmt.Printf("%s\n", "starting admin server")
}

func setup() {
	var err error
	serverApp, err = InitializeServerApp(context.Background(), configFile)
	if err != nil {
		panic(err)
	}
}

func run() error {
	if err := serverApp.Start(); err != nil {
		return err
	}
	serverApp.AwaitSignal()
	return nil
}
