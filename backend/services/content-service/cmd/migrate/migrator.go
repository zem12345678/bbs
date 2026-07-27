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

	servicemigrations "content-service/migrations"

	"gorm.io/gorm"
)

type Migrator struct {
	db *gorm.DB
}

func NewMigrator(db *gorm.DB) *Migrator {
	return &Migrator{db: db}
}

func (m *Migrator) Run(ctx context.Context) error {
	return m.db.WithContext(ctx).Connection(func(db *gorm.DB) error {
		const lockKey = "content-service:migrations"
		if err := db.Exec(`SELECT pg_advisory_lock(hashtext(?))`, lockKey).Error; err != nil {
			return fmt.Errorf("lock content-service migrations: %w", err)
		}
		defer func() { _ = db.Exec(`SELECT pg_advisory_unlock(hashtext(?))`, lockKey).Error }()

		if err := db.Exec(`
CREATE TABLE IF NOT EXISTS schema_migrations (
  version VARCHAR(255) PRIMARY KEY,
  checksum VARCHAR(64) NOT NULL,
  applied_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
)`).Error; err != nil {
			return fmt.Errorf("create content schema_migrations: %w", err)
		}

		files, err := migrationFiles(servicemigrations.Files)
		if err != nil {
			return err
		}
		for _, file := range files {
			if err := runFile(ctx, db, servicemigrations.Files, file); err != nil {
				return err
			}
		}
		return nil
	})
}

func migrationFiles(files fs.FS) ([]string, error) {
	entries, err := fs.ReadDir(files, ".")
	if err != nil {
		return nil, fmt.Errorf("list content migrations: %w", err)
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

func runFile(ctx context.Context, db *gorm.DB, files fs.FS, file string) error {
	b, err := fs.ReadFile(files, file)
	if err != nil {
		return fmt.Errorf("read content migration %s: %w", file, err)
	}
	checksum := fmt.Sprintf("%x", sha256.Sum256(b))
	var recordedChecksum string
	err = db.Raw(`SELECT checksum FROM schema_migrations WHERE version = ?`, file).Row().Scan(&recordedChecksum)
	if err == nil {
		if recordedChecksum != checksum {
			return fmt.Errorf("content migration %s checksum changed after application", file)
		}
		return nil
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return fmt.Errorf("check content migration %s: %w", file, err)
	}

	return db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		for _, statement := range strings.Split(string(b), ";") {
			statement = strings.TrimSpace(statement)
			if statement == "" {
				continue
			}
			if err := tx.Exec(statement).Error; err != nil {
				return fmt.Errorf("execute content migration %s: %w", file, err)
			}
		}
		if err := tx.Exec(`INSERT INTO schema_migrations(version, checksum) VALUES (?, ?)`, file, checksum).Error; err != nil {
			return fmt.Errorf("record content migration %s: %w", file, err)
		}
		return nil
	})
}

func (m *Migrator) Close() {
	if m.db == nil {
		return
	}
	sqlDB, err := m.db.DB()
	if err == nil {
		_ = sqlDB.Close()
	}
}
