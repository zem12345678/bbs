package server

import (
	"context"
	"os/signal"
	"syscall"

	"chat-service/internal/config"
	platformruntime "chat-service/internal/platform/runtime"

	"github.com/spf13/cobra"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

var configFile string

var Command = &cobra.Command{
	Use:          "server",
	Short:        "Start chat gRPC server",
	SilenceUsage: true,
	RunE: func(command *cobra.Command, args []string) error {
		cfg, err := config.Load(configFile)
		if err != nil {
			return err
		}
		logger, err := newLogger(cfg.Log.Level)
		if err != nil {
			return err
		}
		defer logger.Sync()
		ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
		defer stop()
		return platformruntime.NewServer(cfg, logger).Run(ctx)
	},
}

func init() {
	Command.Flags().StringVarP(&configFile, "config", "c", "configs/config.yaml", "chat service config file")
}

func newLogger(level string) (*zap.Logger, error) {
	config := zap.NewProductionConfig()
	if err := config.Level.UnmarshalText([]byte(level)); err != nil {
		return nil, err
	}
	config.EncoderConfig.EncodeTime = zapcore.ISO8601TimeEncoder
	return config.Build()
}
