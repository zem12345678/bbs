package migrate

import (
	"context"
	"fmt"
	"io/fs"
	"sort"
	"strings"

	"reaction-service/migrations"

	"gorm.io/gorm"
)

type Migrator struct {
	db *gorm.DB
}

func NewMigrator(db *gorm.DB) *Migrator {
	return &Migrator{db: db}
}

func (m *Migrator) Run(ctx context.Context) error {
	files, err := fs.Glob(migrations.Files, "*.sql")
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
	b, err := migrations.Files.ReadFile(file)
	if err != nil {
		return err
	}
	for _, statement := range strings.Split(string(b), ";") {
		statement = strings.TrimSpace(statement)
		if statement == "" {
			continue
		}
		if err := m.db.WithContext(ctx).Exec(statement).Error; err != nil {
			return fmt.Errorf("run migration %s: %w", file, err)
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
