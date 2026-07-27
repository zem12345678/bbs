package migrations

import (
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"io/fs"
	"sort"
	"strings"

	embeddedmigrations "chat-service/migrations"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

func Run(ctx context.Context, pool *pgxpool.Pool) error {
	conn, err := pool.Acquire(ctx)
	if err != nil {
		return fmt.Errorf("acquire chat migration connection: %w", err)
	}
	defer conn.Release()

	const lockKey = "chat-service:migrations"
	if _, err := conn.Exec(ctx, `SELECT pg_advisory_lock(hashtext($1))`, lockKey); err != nil {
		return fmt.Errorf("lock chat migrations: %w", err)
	}
	defer func() { _, _ = conn.Exec(context.Background(), `SELECT pg_advisory_unlock(hashtext($1))`, lockKey) }()

	if _, err := conn.Exec(ctx, `
CREATE TABLE IF NOT EXISTS schema_migrations (
  version VARCHAR(255) PRIMARY KEY,
  checksum VARCHAR(64) NOT NULL,
  applied_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
)
`); err != nil {
		return fmt.Errorf("create chat schema_migrations: %w", err)
	}
	if _, err := conn.Exec(ctx, `ALTER TABLE schema_migrations ADD COLUMN IF NOT EXISTS checksum VARCHAR(64) NOT NULL DEFAULT ''`); err != nil {
		return fmt.Errorf("add chat migration checksum: %w", err)
	}

	files, err := migrationFiles(embeddedmigrations.Files)
	if err != nil {
		return fmt.Errorf("list chat migrations: %w", err)
	}
	for _, file := range files {
		if err := runFile(ctx, conn, embeddedmigrations.Files, file); err != nil {
			return err
		}
	}
	return nil
}

func migrationFiles(files fs.FS) ([]string, error) {
	entries, err := fs.ReadDir(files, ".")
	if err != nil {
		return nil, err
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

func runFile(ctx context.Context, conn *pgxpool.Conn, files fs.FS, version string) error {
	contents, err := fs.ReadFile(files, version)
	if err != nil {
		return fmt.Errorf("read chat migration %s: %w", version, err)
	}
	checksum := fmt.Sprintf("%x", sha256.Sum256(contents))
	var recordedChecksum string
	err = conn.QueryRow(ctx, `SELECT checksum FROM schema_migrations WHERE version = $1`, version).Scan(&recordedChecksum)
	if err == nil {
		if recordedChecksum == "" {
			if _, err := conn.Exec(ctx, `UPDATE schema_migrations SET checksum = $1 WHERE version = $2 AND checksum = ''`, checksum, version); err != nil {
				return fmt.Errorf("record existing chat migration checksum %s: %w", version, err)
			}
			return nil
		}
		if recordedChecksum != checksum {
			return fmt.Errorf("chat migration %s checksum changed after application", version)
		}
		return nil
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return fmt.Errorf("check chat migration %s: %w", version, err)
	}

	tx, err := conn.Begin(ctx)
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
	if _, err := tx.Exec(ctx, `INSERT INTO schema_migrations(version, checksum) VALUES($1, $2)`, version, checksum); err != nil {
		return fmt.Errorf("record chat migration %s: %w", version, err)
	}
	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit chat migration %s: %w", version, err)
	}
	return nil
}
