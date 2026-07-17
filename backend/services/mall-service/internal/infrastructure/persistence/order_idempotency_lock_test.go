package persistence

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

func TestLockOrderIdempotencyKeyUsesUserScopedAdvisoryLock(t *testing.T) {
	db := &orderIdempotencyLockQueryer{}

	if err := lockOrderIdempotencyKey(context.Background(), db, 7, " checkout-1 "); err != nil {
		t.Fatalf("lockOrderIdempotencyKey() error = %v", err)
	}

	if db.execCalls != 1 {
		t.Fatalf("Exec() calls = %d, want 1", db.execCalls)
	}
	for _, want := range []string{
		"pg_advisory_xact_lock",
		"hashtextextended",
		"CONCAT($1::BIGINT::text, ':', $2::TEXT)",
	} {
		if !strings.Contains(db.query, want) {
			t.Fatalf("lock query = %q, want %q", db.query, want)
		}
	}
	wantArgs := []any{int64(7), "checkout-1"}
	if len(db.args) != len(wantArgs) {
		t.Fatalf("lock args = %+v, want %+v", db.args, wantArgs)
	}
	for i := range wantArgs {
		if db.args[i] != wantArgs[i] {
			t.Fatalf("lock arg %d = %#v, want %#v", i, db.args[i], wantArgs[i])
		}
	}
}

func TestLockOrderIdempotencyKeySkipsInvalidInput(t *testing.T) {
	db := &orderIdempotencyLockQueryer{}

	if err := lockOrderIdempotencyKey(context.Background(), db, 0, "checkout-1"); err != nil {
		t.Fatalf("lockOrderIdempotencyKey() user error = %v", err)
	}
	if err := lockOrderIdempotencyKey(context.Background(), db, 7, " "); err != nil {
		t.Fatalf("lockOrderIdempotencyKey() key error = %v", err)
	}
	if db.execCalls != 0 {
		t.Fatalf("Exec() calls = %d, want 0", db.execCalls)
	}
}

type orderIdempotencyLockQueryer struct {
	query     string
	args      []any
	execCalls int
}

func (q *orderIdempotencyLockQueryer) Exec(_ context.Context, query string, args ...any) (pgconn.CommandTag, error) {
	q.execCalls++
	q.query = query
	q.args = args
	return pgconn.CommandTag{}, nil
}

func (q *orderIdempotencyLockQueryer) Query(context.Context, string, ...any) (pgx.Rows, error) {
	return nil, nil
}

func (q *orderIdempotencyLockQueryer) QueryRow(context.Context, string, ...any) pgx.Row {
	return orderIdempotencyLockScanRow{}
}

type orderIdempotencyLockScanRow struct{}

func (orderIdempotencyLockScanRow) Scan(...any) error {
	return errors.New("unexpected scan")
}
