package migrate

import (
	"fmt"

	"chat-service/internal/ioc/config"
	datasource "chat-service/internal/ioc/db/postgres"
	ioclogger "chat-service/internal/ioc/logger"
	"chat-service/internal/migrations"

	"github.com/spf13/cobra"
)

var configFile string

var MigrateCmd = &cobra.Command{
	Use:          "migrate",
	Short:        "Run chat PostgreSQL migrations",
	Example:      "chat-service migrate -c configs/config.yaml",
	SilenceUsage: true,
	RunE: func(cmd *cobra.Command, args []string) error {
		v, err := config.New(configFile)
		if err != nil {
			return err
		}
		logOptions, err := ioclogger.NewOptions(v)
		if err != nil {
			return err
		}
		log, err := ioclogger.New(logOptions)
		if err != nil {
			return err
		}
		dbOptions, err := datasource.NewOptions(v, log)
		if err != nil {
			return err
		}
		pool, err := datasource.NewPool(cmd.Context(), dbOptions)
		if err != nil {
			return err
		}
		defer pool.Close()
		if err := migrations.Run(cmd.Context(), pool); err != nil {
			return err
		}
		_, _ = fmt.Fprintln(cmd.OutOrStdout(), "chat-service database migration completed")
		return nil
	},
}

func init() {
	MigrateCmd.PersistentFlags().StringVarP(&configFile, "config", "c", "configs/config.yaml", "Run migration with provided configuration file")
}
