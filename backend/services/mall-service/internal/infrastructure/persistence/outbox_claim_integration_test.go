package persistence

import (
	"context"
	"encoding/json"
	"os"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

func TestClaimPendingOutboxEventsPostgresReclaimsExpiredPublishingLease(t *testing.T) {
	ctx, pool := openOutboxSmokePool(t)
	defer pool.Close()

	eventID := "mall-outbox-smoke-" + uuid.NewString()
	payload, err := json.Marshal(map[string]string{"event_id": eventID})
	if err != nil {
		t.Fatalf("marshal outbox payload: %v", err)
	}
	now := time.Now().UTC()
	createdAt := time.Date(2000, 1, 1, 0, 0, 0, 0, time.UTC)
	_, err = pool.Exec(ctx, `
		INSERT INTO mall_outbox_events (
		  event_id, aggregate_type, aggregate_id, event_type, message_key, payload_json,
		  status, attempts, lease_owner, lease_expires_at, last_error, next_attempt_at,
		  published_at, created_at, updated_at
		) VALUES ($1, 'outbox_smoke', 1, 'mall.outbox.smoke.v1', '1', $2::jsonb,
		  'publishing', 1, 'crashed-worker', $3, '', NULL, NULL, $4, $5)`,
		eventID,
		string(payload),
		now.Add(-time.Minute),
		createdAt,
		now,
	)
	if err != nil {
		t.Fatalf("insert stale publishing event: %v", err)
	}
	defer func() {
		_, _ = pool.Exec(context.Background(), "DELETE FROM mall_outbox_events WHERE event_id = $1", eventID)
	}()

	events, err := NewPostgresRepository(pool).ClaimPendingOutboxEvents(ctx, "outbox-smoke-recovery", 1, time.Minute)
	if err != nil {
		t.Fatalf("claim stale publishing event: %v", err)
	}
	if len(events) != 1 || events[0].EventID != eventID || events[0].Attempt != 2 {
		t.Fatalf("claimed events = %+v, want recovered %q with attempt 2", events, eventID)
	}

	var attempt int
	var leaseOwner string
	var leaseExpiresAt time.Time
	if err := pool.QueryRow(ctx, `
		SELECT attempts, lease_owner, lease_expires_at
		FROM mall_outbox_events
		WHERE event_id = $1`, eventID).Scan(&attempt, &leaseOwner, &leaseExpiresAt); err != nil {
		t.Fatalf("load recovered event: %v", err)
	}
	if attempt != 2 || leaseOwner != "outbox-smoke-recovery" || !leaseExpiresAt.After(now) {
		t.Fatalf("recovered event attempt=%d leaseOwner=%q leaseExpiresAt=%s", attempt, leaseOwner, leaseExpiresAt)
	}
}

func TestCountPendingOutboxEventsPostgresIncludesExpiredPublishingLease(t *testing.T) {
	ctx, pool := openOutboxSmokePool(t)
	defer pool.Close()

	tx, err := pool.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.RepeatableRead})
	if err != nil {
		t.Fatalf("begin transaction: %v", err)
	}
	defer func() {
		_ = tx.Rollback(context.Background())
	}()

	before, err := countPendingOutboxEvents(ctx, tx)
	if err != nil {
		t.Fatalf("count pending outbox events before insert: %v", err)
	}
	eventID := "mall-outbox-count-smoke-" + uuid.NewString()
	payload, err := json.Marshal(map[string]string{"event_id": eventID})
	if err != nil {
		t.Fatalf("marshal outbox payload: %v", err)
	}
	_, err = tx.Exec(ctx, `
		INSERT INTO mall_outbox_events (
		  event_id, aggregate_type, aggregate_id, event_type, message_key, payload_json,
		  status, attempts, lease_owner, lease_expires_at, last_error, next_attempt_at,
		  published_at, created_at, updated_at
		) VALUES ($1, 'outbox_smoke', 1, 'mall.outbox.smoke.v1', '1', $2::jsonb,
		  'publishing', 1, 'crashed-worker', NOW() - INTERVAL '1 minute', '', NULL, NULL, NOW(), NOW())`,
		eventID,
		string(payload),
	)
	if err != nil {
		t.Fatalf("insert stale publishing event: %v", err)
	}

	after, err := countPendingOutboxEvents(ctx, tx)
	if err != nil {
		t.Fatalf("count pending outbox events after insert: %v", err)
	}
	if after != before+1 {
		t.Fatalf("pending outbox count after stale publishing insert = %d, want %d", after, before+1)
	}
}

func openOutboxSmokePool(t *testing.T) (context.Context, *pgxpool.Pool) {
	t.Helper()
	if os.Getenv("BBS_MALL_PG_SMOKE") != "1" {
		t.Skip("set BBS_MALL_PG_SMOKE=1 to run PostgreSQL outbox recovery smoke test")
	}

	dsn := os.Getenv("BBS_MALL_PG_DSN")
	if dsn == "" {
		dsn = "postgres://bbs_mall_app:local_mall_pass@127.0.0.1:5432/bbs?sslmode=disable&search_path=bbs_mall"
	}
	ctx := context.Background()
	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		t.Fatalf("open postgres: %v", err)
	}
	return ctx, pool
}
