package persistence

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

func TestClaimPendingOutboxEventsReclaimsExpiredPublishingLease(t *testing.T) {
	createdAt := time.Date(2026, 7, 19, 10, 0, 0, 0, time.UTC)
	rows := &singleRowRows{hasRow: true, scan: func(dest ...any) error {
		*dest[0].(*string) = "evt-stale-publishing"
		*dest[1].(*string) = "mall_order"
		*dest[2].(*int64) = 8801
		*dest[3].(*string) = "mall.order.paid.v1"
		*dest[4].(*string) = "42"
		*dest[5].(*string) = `{"event_id":"evt-stale-publishing"}`
		*dest[6].(*int) = 2
		*dest[7].(*time.Time) = createdAt
		return nil
	}}
	db := &outboxClaimQueryer{rows: rows}

	events, err := claimPendingOutboxEvents(context.Background(), db, "recovery-worker", 7, 45*time.Second)
	if err != nil {
		t.Fatalf("claim pending outbox events: %v", err)
	}
	if len(events) != 1 {
		t.Fatalf("claimed events = %+v, want one event", events)
	}
	if events[0].EventID != "evt-stale-publishing" || events[0].Attempt != 2 || string(events[0].Payload) != `{"event_id":"evt-stale-publishing"}` {
		t.Fatalf("claimed event = %+v", events[0])
	}
	if !rows.closed {
		t.Fatal("claimed rows must be closed")
	}
	for _, want := range []string{
		"status IN ('pending', 'failed')",
		"(next_attempt_at IS NULL OR next_attempt_at <= NOW())",
		"status = 'publishing'",
		"lease_expires_at <= NOW()",
		"FOR UPDATE SKIP LOCKED",
	} {
		if !strings.Contains(db.query, want) {
			t.Fatalf("claim query = %q, want %q", db.query, want)
		}
	}
	wantArgs := []any{7, "recovery-worker", int64(45)}
	if len(db.args) != len(wantArgs) {
		t.Fatalf("claim args = %#v, want %#v", db.args, wantArgs)
	}
	for i := range wantArgs {
		if db.args[i] != wantArgs[i] {
			t.Fatalf("claim arg %d = %#v, want %#v", i, db.args[i], wantArgs[i])
		}
	}
}

type outboxClaimQueryer struct {
	query string
	args  []any
	rows  pgx.Rows
}

func (q *outboxClaimQueryer) Exec(context.Context, string, ...any) (pgconn.CommandTag, error) {
	return pgconn.CommandTag{}, errors.New("unexpected exec")
}

func (q *outboxClaimQueryer) Query(_ context.Context, query string, args ...any) (pgx.Rows, error) {
	q.query = query
	q.args = args
	return q.rows, nil
}

func (q *outboxClaimQueryer) QueryRow(context.Context, string, ...any) pgx.Row {
	return errorRow{}
}
