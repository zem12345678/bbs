package server

import (
	"fmt"

	"chat-service/internal/ioc/application"

	"github.com/spf13/cobra"
)

var (
	configFile string
	serverApp  *application.Application
)

var StartCmd = &cobra.Command{
	Use:          "server",
	Short:        "Start chat gRPC server",
	Example:      "chat-service server -c configs/config.yaml",
	SilenceUsage: true,
	PreRunE: func(cmd *cobra.Command, args []string) error {
		tip(cmd)
		return setup()
	},
	RunE: func(cmd *cobra.Command, args []string) error {
		return run()
	},
}

func init() {
	StartCmd.PersistentFlags().StringVarP(&configFile, "config", "c", defaultConfigFile, "Start server with provided configuration file")
}

func tip(cmd *cobra.Command) {
	_, _ = fmt.Fprintf(cmd.OutOrStdout(), "starting %s server\n", serviceLabel)
}

func setup() error {
	var err error
	serverApp, err = CreateApp(configFile)
	return err
}

func run() error {
	if err := serverApp.Start(); err != nil {
		return err
	}
	serverApp.AwaitSignal()
	return nil
}
