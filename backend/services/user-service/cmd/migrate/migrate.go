package migrate

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"

	"user-service/internal/ioc/config"
	datasource "user-service/internal/ioc/db/postgres"
	ioclogger "user-service/internal/ioc/logger"

	"github.com/spf13/cobra"
	"gorm.io/gorm"
)

var configFile string

var MigrateCmd = &cobra.Command{
	Use:          "migrate",
	Short:        "Apply user-service SQL migrations",
	Example:      "user-service migrate -c configs/config.yaml",
	SilenceUsage: true,
	RunE: func(cmd *cobra.Command, args []string) error {
		return run(cmd)
	},
}

func init() {
	MigrateCmd.PersistentFlags().StringVarP(&configFile, "config", "c", "configs/config.yaml", "Run migration with provided configuration file")
}

func run(cmd *cobra.Command) error {
	db, err := createMigrationDB(configFile)
	if err != nil {
		return err
	}
	if err := runMigrations(db); err != nil {
		return err
	}
	_, _ = fmt.Fprintln(cmd.OutOrStdout(), "user-service migrations applied")
	return nil
}

func createMigrationDB(configFile string) (*gorm.DB, error) {
	v, err := config.New(configFile)
	if err != nil {
		return nil, err
	}
	logOptions, err := ioclogger.NewOptions(v)
	if err != nil {
		return nil, err
	}
	log, err := ioclogger.New(logOptions)
	if err != nil {
		return nil, err
	}
	dbOptions, err := datasource.NewOptions(v, log)
	if err != nil {
		return nil, err
	}
	return datasource.New(dbOptions)
}

func runMigrations(db *gorm.DB) error {
	sqlDB, err := db.DB()
	if err == nil {
		defer sqlDB.Close()
	}

	files, err := filepath.Glob(filepath.Join("migrations", "*.sql"))
	if err != nil {
		return err
	}
	sort.Strings(files)
	for _, file := range files {
		data, err := os.ReadFile(file)
		if err != nil {
			return err
		}
		if err := db.Exec(string(data)).Error; err != nil {
			return err
		}
	}
	return nil
}
