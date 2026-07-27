package migrate

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"errors"
	"fmt"
	"io/fs"
	"sort"
	"strings"

	"user-service/internal/ioc/config"
	datasource "user-service/internal/ioc/db/postgres"
	ioclogger "user-service/internal/ioc/logger"
	servicemigrations "user-service/migrations"

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
	if err := runMigrations(cmd.Context(), db); err != nil {
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

func runMigrations(ctx context.Context, db *gorm.DB) error {
	sqlDB, err := db.DB()
	if err == nil {
		defer sqlDB.Close()
	}

	return db.WithContext(ctx).Connection(func(conn *gorm.DB) error {
		const lockKey = "user-service:migrations"
		if err := conn.Exec(`SELECT pg_advisory_lock(hashtext(?))`, lockKey).Error; err != nil {
			return fmt.Errorf("lock user-service migrations: %w", err)
		}
		defer func() { _ = conn.Exec(`SELECT pg_advisory_unlock(hashtext(?))`, lockKey).Error }()

		if err := conn.Exec(`
CREATE TABLE IF NOT EXISTS schema_migrations (
  version VARCHAR(255) PRIMARY KEY,
  checksum VARCHAR(64) NOT NULL,
  applied_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
)`).Error; err != nil {
			return fmt.Errorf("create user schema_migrations: %w", err)
		}

		files, err := migrationFiles(servicemigrations.Files)
		if err != nil {
			return err
		}
		for _, file := range files {
			if err := runMigrationFile(ctx, conn, servicemigrations.Files, file); err != nil {
				return err
			}
		}
		return nil
	})
}

func migrationFiles(files fs.FS) ([]string, error) {
	entries, err := fs.ReadDir(files, ".")
	if err != nil {
		return nil, fmt.Errorf("list user migrations: %w", err)
	}
	result := make([]string, 0, len(entries))
	for _, entry := range entries {
		if !entry.IsDir() && strings.HasSuffix(entry.Name(), ".sql") {
			result = append(result, entry.Name())
		}
	}
	sort.Strings(result)
	return result, nil
}

func runMigrationFile(ctx context.Context, db *gorm.DB, files fs.FS, file string) error {
	data, err := fs.ReadFile(files, file)
	if err != nil {
		return fmt.Errorf("read user migration %s: %w", file, err)
	}
	checksum := fmt.Sprintf("%x", sha256.Sum256(data))
	var recordedChecksum string
	err = db.Raw(`SELECT checksum FROM schema_migrations WHERE version = ?`, file).Row().Scan(&recordedChecksum)
	if err == nil {
		if recordedChecksum != checksum {
			return fmt.Errorf("user migration %s checksum changed after application", file)
		}
		return nil
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return fmt.Errorf("check user migration %s: %w", file, err)
	}

	return db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		for _, statement := range strings.Split(string(data), ";") {
			statement = strings.TrimSpace(statement)
			if statement == "" {
				continue
			}
			if err := tx.Exec(statement).Error; err != nil {
				return fmt.Errorf("execute user migration %s: %w", file, err)
			}
		}
		if err := tx.Exec(`INSERT INTO schema_migrations(version, checksum) VALUES (?, ?)`, file, checksum).Error; err != nil {
			return fmt.Errorf("record user migration %s: %w", file, err)
		}
		return nil
	})
}
