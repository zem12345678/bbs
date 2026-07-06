package migration

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"reaction-service/internal/support/config"

	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

type Migrator struct {
	db  *gorm.DB
	dir string
}

func NewMigrator(db *gorm.DB) *Migrator {
	return &Migrator{db: db, dir: "migrations"}
}

func provideDB(cfg *config.Config) (*gorm.DB, error) {
	return gorm.Open(postgres.Open(cfg.Postgres.DSN), &gorm.Config{})
}

func (m *Migrator) Run(ctx context.Context) error {
	files, err := filepath.Glob(filepath.Join(m.dir, "*.sql"))
	if err != nil {
		return err
	}
	sort.Strings(files)
	for _, file := range files {
		if err := m.runFile(ctx, file); err != nil {
			return err
		}
	}
	return nil
}

func (m *Migrator) runFile(ctx context.Context, file string) error {
	b, err := os.ReadFile(file)
	if err != nil {
		return err
	}
	for _, statement := range strings.Split(string(b), ";") {
		statement = strings.TrimSpace(statement)
		if statement == "" {
			continue
		}
		if err := m.db.WithContext(ctx).Exec(statement).Error; err != nil {
			return fmt.Errorf("run migration %s: %w", filepath.Base(file), err)
		}
	}
	return nil
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
