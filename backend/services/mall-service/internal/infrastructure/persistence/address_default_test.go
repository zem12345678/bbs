package persistence

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

func TestLockUserAddressesUsesDedicatedAdvisoryLock(t *testing.T) {
	db := &addressDefaultQueryer{}

	if err := lockUserAddresses(context.Background(), db, 7); err != nil {
		t.Fatalf("lockUserAddresses() error = %v", err)
	}

	if db.execCalls != 1 {
		t.Fatalf("Exec() calls = %d, want 1", db.execCalls)
	}
	if !strings.Contains(db.query, "pg_advisory_xact_lock") {
		t.Fatalf("lock query = %q, want advisory lock", db.query)
	}
	wantArg := addressAdvisoryLockBase + int64(7)
	if len(db.args) != 1 || db.args[0] != wantArg {
		t.Fatalf("lock args = %+v, want [%d]", db.args, wantArg)
	}
}

func TestAddressSchemaNormalizesAndConstrainsDefaultAddress(t *testing.T) {
	cleanupIndex := -1
	uniqueIndex := -1
	for i, statement := range schemaStatements {
		normalized := strings.Join(strings.Fields(statement), " ")
		if strings.Contains(normalized, "ROW_NUMBER() OVER (PARTITION BY user_id ORDER BY updated_at DESC, id DESC)") &&
			strings.Contains(normalized, "WHERE is_default = true") &&
			strings.Contains(normalized, "r.rn > 1") {
			cleanupIndex = i
		}
		if strings.Contains(normalized, "CREATE UNIQUE INDEX IF NOT EXISTS idx_mall_addresses_one_default_per_user") &&
			strings.Contains(normalized, "ON mall_addresses (user_id)") &&
			strings.Contains(normalized, "WHERE is_default = true") {
			uniqueIndex = i
		}
	}
	if cleanupIndex < 0 {
		t.Fatal("schemaStatements missing default address cleanup")
	}
	if uniqueIndex < 0 {
		t.Fatal("schemaStatements missing one-default-address unique index")
	}
	if cleanupIndex > uniqueIndex {
		t.Fatalf("address cleanup statement index=%d after unique index=%d", cleanupIndex, uniqueIndex)
	}
}

type addressDefaultQueryer struct {
	query     string
	args      []any
	execCalls int
}

func (q *addressDefaultQueryer) Exec(_ context.Context, query string, args ...any) (pgconn.CommandTag, error) {
	q.execCalls++
	q.query = query
	q.args = args
	return pgconn.CommandTag{}, nil
}

func (q *addressDefaultQueryer) Query(context.Context, string, ...any) (pgx.Rows, error) {
	return nil, nil
}

func (q *addressDefaultQueryer) QueryRow(context.Context, string, ...any) pgx.Row {
	return addressDefaultScanRow{}
}

type addressDefaultScanRow struct{}

func (addressDefaultScanRow) Scan(...any) error {
	return errors.New("unexpected scan")
}
