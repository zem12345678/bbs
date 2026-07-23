package migrations

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/jackc/pgx/v5/pgxpool"
)

func Run(ctx context.Context, pool *pgxpool.Pool, directory string) error {
	if _, err := pool.Exec(ctx, `
CREATE TABLE IF NOT EXISTS schema_migrations (
  version VARCHAR(255) PRIMARY KEY,
  applied_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
)
`); err != nil {
		return fmt.Errorf("create chat schema_migrations: %w", err)
	}

	files, err := filepath.Glob(filepath.Join(directory, "*.sql"))
	if err != nil {
		return fmt.Errorf("list chat migrations: %w", err)
	}
	sort.Strings(files)
	for _, file := range files {
		if err := runFile(ctx, pool, file); err != nil {
			return err
		}
	}
	return nil
}

func runFile(ctx context.Context, pool *pgxpool.Pool, path string) error {
	version := filepath.Base(path)
	var applied bool
	if err := pool.QueryRow(ctx, `
SELECT EXISTS(SELECT 1 FROM schema_migrations WHERE version = $1)
`, version).Scan(&applied); err != nil {
		return fmt.Errorf("check chat migration %s: %w", version, err)
	}
	if applied {
		return nil
	}

	contents, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("read chat migration %s: %w", version, err)
	}
	tx, err := pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin chat migration %s: %w", version, err)
	}
	defer func() { _ = tx.Rollback(ctx) }()
	for _, statement := range strings.Split(string(contents), ";") {
		if statement = strings.TrimSpace(statement); statement == "" {
			continue
		}
		if _, err := tx.Exec(ctx, statement); err != nil {
			return fmt.Errorf("execute chat migration %s: %w", version, err)
		}
	}
	if _, err := tx.Exec(ctx, `INSERT INTO schema_migrations(version) VALUES($1)`, version); err != nil {
		return fmt.Errorf("record chat migration %s: %w", version, err)
	}
	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit chat migration %s: %w", version, err)
	}
	return nil
}
