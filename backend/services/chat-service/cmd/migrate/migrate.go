package migrate

import (
	"context"

	"chat-service/internal/config"
	"chat-service/internal/migrations"
	platformpostgres "chat-service/internal/platform/postgres"

	"github.com/spf13/cobra"
)

var configFile string

var Command = &cobra.Command{
	Use:          "migrate",
	Short:        "Run chat PostgreSQL migrations",
	SilenceUsage: true,
	RunE: func(command *cobra.Command, args []string) error {
		cfg, err := config.Load(configFile)
		if err != nil {
			return err
		}
		pool, err := platformpostgres.Open(context.Background(), cfg.Postgres.DSN, cfg.Postgres.MaxOpenConns)
		if err != nil {
			return err
		}
		defer pool.Close()
		return migrations.Run(context.Background(), pool, "migrations")
	},
}

func init() {
	Command.Flags().StringVarP(&configFile, "config", "c", "configs/config.yaml", "chat service config file")
}
