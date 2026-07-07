package server

import (
	"fmt"

	"credit-service/internal/ioc/application"

	"github.com/spf13/cobra"
)

var (
	configFile string
	serverApp  *application.Application
)

var StartCmd = &cobra.Command{
	Use:          "server",
	Short:        "Start credit gRPC server",
	Example:      serviceName + " server -c " + defaultConfigFile,
	SilenceUsage: true,
	PreRun: func(cmd *cobra.Command, args []string) {
		tip()
		setup()
	},
	RunE: func(cmd *cobra.Command, args []string) error {
		return run()
	},
}

func init() {
	StartCmd.PersistentFlags().StringVarP(&configFile, "config", "c", defaultConfigFile, "Start server with provided configuration file")
}

func tip() {
	fmt.Printf("%s\n", "starting "+serviceLabel+" server")
}

func setup() {
	var err error
	serverApp, err = CreateApp(configFile)
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
