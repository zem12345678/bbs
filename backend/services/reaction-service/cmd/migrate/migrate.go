package migrate

import (
	"context"
	"fmt"

	"reaction-service/internal/ioc/config"
	datasource "reaction-service/internal/ioc/db/postgres"
	ioclogger "reaction-service/internal/ioc/logger"
	"reaction-service/pkg/logger"

	"github.com/spf13/cobra"
	"gorm.io/gorm"
)

var configFile string

var MigrateCmd = &cobra.Command{
	Use:          "migrate",
	Short:        "Synchronize reaction-service database schema",
	Example:      "reaction-service migrate -c configs/config.yaml",
	SilenceUsage: true,
	RunE: func(cmd *cobra.Command, args []string) error {
		return run(cmd)
	},
}

func init() {
	MigrateCmd.PersistentFlags().StringVarP(&configFile, "config", "c", "configs/config.yaml", "Run migration with provided configuration file")
}

func run(cmd *cobra.Command) error {
	log, db, err := createMigrationDependencies(configFile)
	if err != nil {
		return err
	}
	migrator := NewMigrator(db)
	defer migrator.Close()

	if log != nil {
		log.Info("start database migration")
	}
	if err := migrator.Run(context.Background()); err != nil {
		return err
	}
	if log != nil {
		log.Info("database migration completed")
	}
	_, _ = fmt.Fprintln(cmd.OutOrStdout(), "reaction-service database migration completed")
	return nil
}

func createMigrationDependencies(configFile string) (logger.Logger, *gorm.DB, error) {
	v, err := config.New(configFile)
	if err != nil {
		return nil, nil, err
	}

	logOptions, err := ioclogger.NewOptions(v)
	if err != nil {
		return nil, nil, err
	}
	log, err := ioclogger.New(logOptions)
	if err != nil {
		return nil, nil, err
	}

	dbOptions, err := datasource.NewOptions(v, log)
	if err != nil {
		return nil, nil, err
	}
	db, err := datasource.New(dbOptions)
	if err != nil {
		return nil, nil, err
	}
	return log, db, nil
}
