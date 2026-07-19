package persistence

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/jackc/pgx/v5"
)

func TestCountPendingOutboxEventsIncludesExpiredPublishingLease(t *testing.T) {
	db := &outboxPendingCountQueryer{row: outboxPendingCountRow{count: 1}}

	count, err := countPendingOutboxEvents(context.Background(), db)
	if err != nil {
		t.Fatalf("count pending outbox events: %v", err)
	}
	if count != 1 {
		t.Fatalf("pending outbox count = %d, want 1", count)
	}
	if len(db.args) != 0 {
		t.Fatalf("count args = %#v, want none", db.args)
	}
	for _, want := range []string{
		"status IN ('pending', 'failed')",
		"(next_attempt_at IS NULL OR next_attempt_at <= NOW())",
		"status = 'publishing'",
		"lease_expires_at <= NOW()",
	} {
		if !strings.Contains(db.query, want) {
			t.Fatalf("count query = %q, want %q", db.query, want)
		}
	}
}

type outboxPendingCountQueryer struct {
	query string
	args  []any
	row   pgx.Row
}

func (q *outboxPendingCountQueryer) QueryRow(_ context.Context, query string, args ...any) pgx.Row {
	q.query = query
	q.args = args
	return q.row
}

type outboxPendingCountRow struct {
	count int
}

func (r outboxPendingCountRow) Scan(dest ...any) error {
	if len(dest) != 1 {
		return errors.New("unexpected scan destination count")
	}
	count, ok := dest[0].(*int)
	if !ok {
		return errors.New("unexpected scan destination")
	}
	*count = r.count
	return nil
}
