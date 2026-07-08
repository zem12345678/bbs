package migrate

import (
	"context"
	"fmt"

	mallapp "mall-service/internal/app"
	"mall-service/internal/infrastructure/persistence"
	"mall-service/internal/ioc/config"
	datasource "mall-service/internal/ioc/db/postgres"
	ioclogger "mall-service/internal/ioc/logger"
	"mall-service/pkg/logger"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/spf13/cobra"
)

var configFile string

var MigrateCmd = &cobra.Command{
	Use:          "migrate",
	Short:        "Run mall-service database migrations",
	Example:      "mall-service migrate -c configs/config.yaml",
	SilenceUsage: true,
	RunE: func(cmd *cobra.Command, args []string) error {
		return run(cmd)
	},
}

func init() {
	MigrateCmd.PersistentFlags().StringVarP(&configFile, "config", "c", "configs/config.yaml", "Run migration with provided configuration file")
}

func run(cmd *cobra.Command) error {
	log, pool, err := createMigrationDependencies(configFile)
	if err != nil {
		return err
	}
	defer pool.Close()

	if log != nil {
		log.Info("start database migration")
	}
	repo := persistence.NewPostgresRepository(pool)
	if err := repo.EnsureSchema(context.Background()); err != nil {
		return err
	}
	if log != nil {
		log.Info("database migration completed")
	}
	_, _ = fmt.Fprintln(cmd.OutOrStdout(), "mall-service database migration completed")
	return nil
}

func createMigrationDependencies(configFile string) (logger.Logger, *pgxpool.Pool, error) {
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
	pool, err := mallapp.ProvidePostgresPool(context.Background(), dbOptions)
	if err != nil {
		return nil, nil, err
	}
	return log, pool, nil
}
