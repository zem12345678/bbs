package server

import (
	"context"
	"fmt"

	"github.com/spf13/cobra"
)

var (
	configFile string
	serverApp  *App
)

var StartCmd = &cobra.Command{
	Use:          "server",
	Short:        "Start reaction gRPC server",
	Example:      "reaction-service server -c configs/config.yaml",
	SilenceUsage: true,
	PreRun: func(cmd *cobra.Command, args []string) {
		_, _ = fmt.Fprintln(cmd.OutOrStdout(), "starting reaction server")
	},
	RunE: func(cmd *cobra.Command, args []string) error {
		var err error
		serverApp, err = InitializeServerApp(context.Background(), configFile)
		if err != nil {
			return err
		}
		defer serverApp.Close()
		if err := serverApp.Start(); err != nil {
			return err
		}
		serverApp.AwaitSignal()
		return nil
	},
}

func init() {
	StartCmd.PersistentFlags().StringVarP(&configFile, "config", "c", "configs/config.yaml", "Start server with provided configuration file")
}
