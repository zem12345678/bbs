package server

import (
	"fmt"

	"admin/internal/ioc/application"

	"github.com/spf13/cobra"
)

var (
	configFile string
	serverApp  *application.Application

	StartCmd = &cobra.Command{
		Use:          "server",
		Short:        "Start admin gRPC server",
		Example:      "admin-service server -c configs/config.yaml",
		SilenceUsage: true,
		PreRunE: func(cmd *cobra.Command, args []string) error {
			tip()
			return setup()
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

func setup() error {
	var err error
	serverApp, err = CreateApp(configFile)
	if err != nil {
		return fmt.Errorf("create admin application: %w", err)
	}
	return nil
}

func run() error {
	if err := serverApp.Start(); err != nil {
		return err
	}
	serverApp.AwaitSignal()
	return nil
}
