package persistence

import (
	"context"
	"os"
	"strings"
	"testing"
	"time"

	domain "notification-service/internal/domain/notification"

	"github.com/jackc/pgx/v5/pgxpool"
)

func TestCreateSystemNotificationsIsIdempotentAndAtomic(t *testing.T) {
	dsn := os.Getenv("BBS_NOTIFICATION_TEST_DSN")
	if dsn == "" {
		t.Skip("set BBS_NOTIFICATION_TEST_DSN to run postgres-backed notification repository tests")
	}

	ctx := context.Background()
	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		t.Fatalf("open postgres pool: %v", err)
	}
	defer pool.Close()
	repo := NewPostgresRepository(pool)
	if err := repo.EnsureSchema(ctx); err != nil {
		t.Fatalf("ensure schema: %v", err)
	}

	suffix := time.Now().UnixNano()
	command := domain.SystemNotificationCommand{
		RecipientIDs:   []int64{9_000_000_000 + suffix%1_000_000, 9_100_000_000 + suffix%1_000_000},
		Title:          "repository idempotency test",
		Content:        "verify one batch",
		ActorID:        7,
		IdempotencyKey: "repo-" + strings.ReplaceAll(time.Now().UTC().Format("20060102150405.000000000"), ".", ""),
	}
	sourceEventID := "admin_system:7:" + command.IdempotencyKey
	defer func() {
		_, _ = pool.Exec(ctx, `DELETE FROM notifications WHERE source_event_id = $1`, sourceEventID)
	}()

	delivered, err := repo.CreateSystemNotifications(ctx, command, time.Now())
	if err != nil {
		t.Fatalf("CreateSystemNotifications() error = %v", err)
	}
	if delivered != 2 {
		t.Fatalf("delivered = %d, want 2", delivered)
	}
	delivered, err = repo.CreateSystemNotifications(ctx, command, time.Now())
	if err != nil {
		t.Fatalf("retry CreateSystemNotifications() error = %v", err)
	}
	if delivered != 0 {
		t.Fatalf("retry delivered = %d, want 0", delivered)
	}
	var count int64
	if err := pool.QueryRow(ctx, `SELECT COUNT(*) FROM notifications WHERE source_event_id = $1`, sourceEventID).Scan(&count); err != nil {
		t.Fatalf("count idempotent notifications: %v", err)
	}
	if count != 2 {
		t.Fatalf("stored notifications = %d, want 2", count)
	}

	atomicTitle := "repository atomic failure " + command.IdempotencyKey
	failedRecipients := []int64{command.RecipientIDs[0] + 100, command.RecipientIDs[1] + 100}
	_, err = repo.CreateSystemNotifications(ctx, domain.SystemNotificationCommand{
		RecipientIDs:   failedRecipients,
		Title:          atomicTitle,
		Content:        "must not partially persist",
		ActorID:        7,
		IdempotencyKey: strings.Repeat("x", 120), // prefix makes source_event_id exceed VARCHAR(128)
	}, time.Now())
	if err == nil {
		t.Fatal("expected oversized source event id to fail")
	}
	if err := pool.QueryRow(ctx, `SELECT COUNT(*) FROM notifications WHERE title = $1`, atomicTitle).Scan(&count); err != nil {
		t.Fatalf("count failed batch notifications: %v", err)
	}
	if count != 0 {
		t.Fatalf("failed batch stored %d notifications, want 0", count)
	}
}
