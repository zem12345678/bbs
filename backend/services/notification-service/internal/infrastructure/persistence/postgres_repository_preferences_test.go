package persistence

import (
	"context"
	"fmt"
	"os"
	"testing"
	"time"

	domain "notification-service/internal/domain/notification"

	"github.com/jackc/pgx/v5/pgxpool"
)

func TestNotificationPreferencesPersistAndGateWrites(t *testing.T) {
	dsn := os.Getenv("BBS_NOTIFICATION_TEST_DSN")
	if dsn == "" {
		t.Skip("set BBS_NOTIFICATION_TEST_DSN to run postgres-backed notification preference tests")
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

	userID := int64(9_600_000_000) + time.Now().UnixNano()%100_000_000
	actorID := userID + 1
	jobID := userID + 2
	prefix := fmt.Sprintf("preference-%d", userID)
	t.Cleanup(func() {
		_, _ = pool.Exec(ctx, `DELETE FROM notifications WHERE user_id = $1 OR actor_id = $1 OR source_event_id LIKE $2`, userID, prefix+"%")
		_, _ = pool.Exec(ctx, `DELETE FROM notification_preferences WHERE user_id = $1`, userID)
		_, _ = pool.Exec(ctx, `DELETE FROM notification_erased_users WHERE user_id = $1`, userID)
	})

	if err := repo.ReplacePreferences(ctx, userID, []domain.NotificationPreference{
		{Type: domain.NotificationTypeComment, Enabled: false},
		{Type: domain.SystemNotificationType, Enabled: false},
	}); err != nil {
		t.Fatalf("replace preferences: %v", err)
	}
	items, err := repo.ListPreferences(ctx, userID)
	if err != nil {
		t.Fatalf("list preferences: %v", err)
	}
	if len(items) != len(domain.DefaultNotificationPreferences()) {
		t.Fatalf("stored preferences = %d, want %d", len(items), len(domain.DefaultNotificationPreferences()))
	}

	if err := repo.Create(ctx, domain.Notification{UserID: userID, Type: domain.NotificationTypeComment, Title: "disabled"}, prefix+"-comment-disabled", time.Now()); err != nil {
		t.Fatalf("create disabled comment: %v", err)
	}
	if err := repo.Create(ctx, domain.Notification{UserID: userID, Type: domain.NotificationTypeLike, Title: "enabled"}, prefix+"-like-enabled", time.Now()); err != nil {
		t.Fatalf("create enabled like: %v", err)
	}
	delivered, err := repo.CreateSystemNotifications(ctx, domain.SystemNotificationCommand{
		RecipientIDs: []int64{userID}, ActorID: actorID, Title: "disabled", Content: "disabled", IdempotencyKey: prefix + "-system-disabled",
	}, time.Now())
	if err != nil {
		t.Fatalf("create disabled system notification: %v", err)
	}
	if delivered != 0 {
		t.Fatalf("disabled system delivered = %d, want 0", delivered)
	}
	assertPostgresCount(t, ctx, pool, 0, `SELECT COUNT(*) FROM notifications WHERE source_event_id = $1`, prefix+"-comment-disabled")
	assertPostgresCount(t, ctx, pool, 1, `SELECT COUNT(*) FROM notifications WHERE source_event_id = $1`, prefix+"-like-enabled")

	if err := repo.ReplacePreferences(ctx, userID, []domain.NotificationPreference{{Type: domain.NotificationTypeComment, Enabled: true}, {Type: domain.SystemNotificationType, Enabled: true}}); err != nil {
		t.Fatalf("re-enable preferences: %v", err)
	}
	if err := repo.Create(ctx, domain.Notification{UserID: userID, Type: domain.NotificationTypeComment, Title: "enabled"}, prefix+"-comment-enabled", time.Now()); err != nil {
		t.Fatalf("create enabled comment: %v", err)
	}
	delivered, err = repo.CreateSystemNotifications(ctx, domain.SystemNotificationCommand{
		RecipientIDs: []int64{userID}, ActorID: actorID, Title: "enabled", Content: "enabled", IdempotencyKey: prefix + "-system-enabled",
	}, time.Now())
	if err != nil {
		t.Fatalf("create enabled system notification: %v", err)
	}
	if delivered != 1 {
		t.Fatalf("enabled system delivered = %d, want 1", delivered)
	}

	if err := repo.EraseUserData(ctx, userID, jobID, 1); err != nil {
		t.Fatalf("erase user data: %v", err)
	}
	assertPostgresCount(t, ctx, pool, 0, `SELECT COUNT(*) FROM notification_preferences WHERE user_id = $1`, userID)
}
