package migrate

import (
	"context"
	"fmt"

	adminapp "admin/internal/app"
	"admin/internal/infrastructure/persistence"
	"admin/internal/ioc/config"
	datasource "admin/internal/ioc/db/postgres"
	ioclogger "admin/internal/ioc/logger"
	"admin/pkg/logger"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"gorm.io/gorm"
)

var configFile string

var MigrateCmd = &cobra.Command{
	Use:          "migrate",
	Short:        "Synchronize admin-service database schema and seed RBAC policies",
	Example:      "admin-service migrate -c configs/config.yaml",
	SilenceUsage: true,
	RunE: func(cmd *cobra.Command, args []string) error {
		return run(cmd)
	},
}

func init() {
	MigrateCmd.PersistentFlags().StringVarP(&configFile, "config", "c", "configs/config.yaml", "Run migration with provided configuration file")
}

func run(cmd *cobra.Command) error {
	log, db, v, err := createMigrationDependencies(configFile)
	if err != nil {
		return err
	}

	if log != nil {
		log.Info("start database auto migration")
	}
	repo := persistence.NewRepository(db)
	ctx := context.Background()
	if err := repo.EnsureSchema(ctx); err != nil {
		return fmt.Errorf("auto migrate admin-service schema: %w", err)
	}
	if err := repo.SeedDefaults(ctx, adminapp.BootstrapAdminPrefixes(v), v.GetString("auth.defaultAdminPassword")); err != nil {
		return fmt.Errorf("seed admin-service defaults: %w", err)
	}

	sqlDB, err := db.DB()
	if err == nil {
		_ = sqlDB.Close()
	}

	if log != nil {
		log.Info("database auto migration completed")
	}
	_, _ = fmt.Fprintln(cmd.OutOrStdout(), "admin-service database migration completed")
	return nil
}

func createMigrationDependencies(configFile string) (logger.Logger, *gorm.DB, *viper.Viper, error) {
	v, err := config.New(configFile)
	if err != nil {
		return nil, nil, nil, err
	}

	logOptions, err := ioclogger.NewOptions(v)
	if err != nil {
		return nil, nil, nil, err
	}
	log, err := ioclogger.New(logOptions)
	if err != nil {
		return nil, nil, nil, err
	}

	dbOptions, err := datasource.NewOptions(v, log)
	if err != nil {
		return nil, nil, nil, err
	}
	db, err := datasource.New(dbOptions)
	if err != nil {
		return nil, nil, nil, err
	}
	return log, db, v, nil
}
