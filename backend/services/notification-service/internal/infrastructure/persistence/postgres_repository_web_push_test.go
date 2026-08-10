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

func TestWebPushOutboxFollowsSubscriptionAtNotificationInsertTime(t *testing.T) {
	dsn := os.Getenv("BBS_NOTIFICATION_TEST_DSN")
	if dsn == "" {
		t.Skip("set BBS_NOTIFICATION_TEST_DSN to run postgres-backed web push tests")
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

	seed := time.Now().UnixNano() % 100_000_000
	oldUserID := int64(9_700_000_000) + seed
	newUserID := oldUserID + 1
	lateUserID := oldUserID + 2
	jobID := oldUserID + 100
	endpoint := fmt.Sprintf("https://push.example/subscription/%d", seed)
	lateEndpoint := endpoint + "/late"
	sourcePrefix := fmt.Sprintf("web-push-%d-", seed)
	t.Cleanup(func() {
		_, _ = pool.Exec(context.Background(), `DELETE FROM notifications WHERE user_id = ANY($1::BIGINT[]) OR source_event_id LIKE $2`, []int64{oldUserID, newUserID, lateUserID}, sourcePrefix+"%")
		_, _ = pool.Exec(context.Background(), `DELETE FROM web_push_subscriptions WHERE endpoint IN ($1, $2)`, endpoint, lateEndpoint)
		_, _ = pool.Exec(context.Background(), `DELETE FROM notification_erased_users WHERE user_id = ANY($1::BIGINT[])`, []int64{oldUserID, newUserID, lateUserID})
	})

	oldSubscription, err := repo.UpsertWebPushSubscription(ctx, domain.WebPushSubscription{
		UserID: oldUserID, Endpoint: endpoint, Auth: "old-auth", PublicKey: "old-key", State: domain.WebPushSubscriptionStateActive,
	}, domain.WebPushMaxSubscriptionsPerUser)
	if err != nil {
		t.Fatalf("register old owner: %v", err)
	}
	if oldSubscription.RegistrationState != "subscribed" {
		t.Fatalf("first registration state = %q", oldSubscription.RegistrationState)
	}
	existingSubscription, err := repo.UpsertWebPushSubscription(ctx, domain.WebPushSubscription{
		UserID: oldUserID, Endpoint: endpoint, Auth: "old-auth", PublicKey: "old-key", State: domain.WebPushSubscriptionStateActive,
	}, domain.WebPushMaxSubscriptionsPerUser)
	if err != nil {
		t.Fatalf("register existing subscription: %v", err)
	}
	if existingSubscription.RegistrationState != "already-subscribed" {
		t.Fatalf("existing registration state = %q", existingSubscription.RegistrationState)
	}
	oldNotificationID := insertWebPushTestNotification(t, ctx, pool, oldUserID, sourcePrefix+"old")
	assertPostgresCount(t, ctx, pool, 1, `SELECT COUNT(*) FROM web_push_outbox WHERE notification_id = $1 AND subscription_id = $2`, oldNotificationID, oldSubscription.ID)

	migrated, err := repo.UpsertWebPushSubscription(ctx, domain.WebPushSubscription{
		UserID: newUserID, Endpoint: endpoint, Auth: "new-auth", PublicKey: "new-key", State: domain.WebPushSubscriptionStateActive,
	}, domain.WebPushMaxSubscriptionsPerUser)
	if err != nil {
		t.Fatalf("migrate endpoint owner: %v", err)
	}
	if migrated.ID != oldSubscription.ID || migrated.UserID != newUserID {
		t.Fatalf("migrated subscription = %+v, old=%+v", migrated, oldSubscription)
	}
	assertPostgresCount(t, ctx, pool, 0, `SELECT COUNT(*) FROM web_push_outbox WHERE notification_id = $1`, oldNotificationID)
	newNotificationID := insertWebPushTestNotification(t, ctx, pool, newUserID, sourcePrefix+"new")
	assertPostgresCount(t, ctx, pool, 1, `SELECT COUNT(*) FROM web_push_outbox WHERE notification_id = $1 AND subscription_id = $2`, newNotificationID, migrated.ID)

	preSubscriptionNotificationID := insertWebPushTestNotification(t, ctx, pool, lateUserID, sourcePrefix+"before-subscription")
	lateSubscription, err := repo.UpsertWebPushSubscription(ctx, domain.WebPushSubscription{
		UserID: lateUserID, Endpoint: lateEndpoint, Auth: "late-auth", PublicKey: "late-key", State: domain.WebPushSubscriptionStateActive,
	}, domain.WebPushMaxSubscriptionsPerUser)
	if err != nil {
		t.Fatalf("register late subscription: %v", err)
	}
	assertPostgresCount(t, ctx, pool, 0, `SELECT COUNT(*) FROM web_push_outbox WHERE notification_id = $1`, preSubscriptionNotificationID)
	postSubscriptionNotificationID := insertWebPushTestNotification(t, ctx, pool, lateUserID, sourcePrefix+"after-subscription")
	assertPostgresCount(t, ctx, pool, 1, `SELECT COUNT(*) FROM web_push_outbox WHERE notification_id = $1 AND subscription_id = $2`, postSubscriptionNotificationID, lateSubscription.ID)

	if err := repo.EraseUserData(ctx, newUserID, jobID, 1); err != nil {
		t.Fatalf("erase migrated owner: %v", err)
	}
	assertPostgresCount(t, ctx, pool, 0, `SELECT COUNT(*) FROM web_push_subscriptions WHERE user_id = $1`, newUserID)
	assertPostgresCount(t, ctx, pool, 0, `SELECT COUNT(*) FROM web_push_outbox WHERE subscription_id = $1`, migrated.ID)
}

func TestWebPushOutboxReleaseAndCompletedRetentionCleanup(t *testing.T) {
	dsn := os.Getenv("BBS_NOTIFICATION_TEST_DSN")
	if dsn == "" {
		t.Skip("set BBS_NOTIFICATION_TEST_DSN to run postgres-backed web push tests")
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

	seed := time.Now().UnixNano() % 100_000_000
	userID := int64(9_800_000_000) + seed
	endpoint := fmt.Sprintf("https://push.example/retention/%d", seed)
	sourcePrefix := fmt.Sprintf("web-push-retention-%d-", seed)
	t.Cleanup(func() {
		_, _ = pool.Exec(context.Background(), `DELETE FROM notifications WHERE user_id = $1 OR source_event_id LIKE $2`, userID, sourcePrefix+"%")
		_, _ = pool.Exec(context.Background(), `DELETE FROM web_push_subscriptions WHERE endpoint = $1`, endpoint)
	})

	subscription, err := repo.UpsertWebPushSubscription(ctx, domain.WebPushSubscription{
		UserID: userID, Endpoint: endpoint, Auth: "retention-auth", PublicKey: "retention-key", State: domain.WebPushSubscriptionStateActive,
	}, domain.WebPushMaxSubscriptionsPerUser)
	if err != nil {
		t.Fatalf("register subscription: %v", err)
	}
	oldNotificationID := insertWebPushTestNotification(t, ctx, pool, userID, sourcePrefix+"old")
	recentNotificationID := insertWebPushTestNotification(t, ctx, pool, userID, sourcePrefix+"recent")
	pendingNotificationID := insertWebPushTestNotification(t, ctx, pool, userID, sourcePrefix+"pending")

	lookupOutboxID := func(notificationID int64) int64 {
		t.Helper()
		var outboxID int64
		if err := pool.QueryRow(ctx, `SELECT id FROM web_push_outbox WHERE notification_id = $1 AND subscription_id = $2`, notificationID, subscription.ID).Scan(&outboxID); err != nil {
			t.Fatalf("lookup outbox: %v", err)
		}
		return outboxID
	}
	oldOutboxID := lookupOutboxID(oldNotificationID)
	recentOutboxID := lookupOutboxID(recentNotificationID)
	pendingOutboxID := lookupOutboxID(pendingNotificationID)
	cutoff := time.Date(1900, 1, 2, 0, 0, 0, 0, time.UTC)
	if _, err := pool.Exec(ctx, `UPDATE web_push_outbox SET completed_at = $2, result = 'delivered', locked_at = $2 WHERE id = $1`, oldOutboxID, cutoff.Add(-24*time.Hour)); err != nil {
		t.Fatalf("complete old outbox: %v", err)
	}
	if _, err := pool.Exec(ctx, `UPDATE web_push_outbox SET completed_at = $2, result = 'delivered', locked_at = $2 WHERE id = $1`, recentOutboxID, cutoff.Add(24*time.Hour)); err != nil {
		t.Fatalf("complete recent outbox: %v", err)
	}
	if _, err := pool.Exec(ctx, `UPDATE web_push_outbox SET locked_at = NOW() WHERE id = $1`, pendingOutboxID); err != nil {
		t.Fatalf("claim pending outbox: %v", err)
	}

	if err := repo.ReleaseWebPushDeliveries(ctx, []int64{pendingOutboxID}); err != nil {
		t.Fatalf("release pending outbox: %v", err)
	}
	assertPostgresCount(t, ctx, pool, 1, `SELECT COUNT(*) FROM web_push_outbox WHERE id = $1 AND locked_at IS NULL AND completed_at IS NULL`, pendingOutboxID)

	deleted, err := repo.CleanupCompletedWebPushDeliveries(ctx, cutoff, 1000)
	if err != nil {
		t.Fatalf("cleanup completed outbox: %v", err)
	}
	if deleted != 1 {
		t.Fatalf("deleted completed outbox = %d, want 1", deleted)
	}
	assertPostgresCount(t, ctx, pool, 0, `SELECT COUNT(*) FROM web_push_outbox WHERE id = $1`, oldOutboxID)
	assertPostgresCount(t, ctx, pool, 1, `SELECT COUNT(*) FROM web_push_outbox WHERE id = $1`, recentOutboxID)
	assertPostgresCount(t, ctx, pool, 1, `SELECT COUNT(*) FROM web_push_outbox WHERE id = $1`, pendingOutboxID)
}

func insertWebPushTestNotification(t *testing.T, ctx context.Context, pool *pgxpool.Pool, userID int64, sourceEventID string) int64 {
	t.Helper()
	var notificationID int64
	if err := pool.QueryRow(ctx, `
INSERT INTO notifications(user_id, type, title, content, source_event_id)
VALUES($1, 'test', 'web push test', 'body', $2)
RETURNING id
`, userID, sourceEventID).Scan(&notificationID); err != nil {
		t.Fatalf("insert notification: %v", err)
	}
	return notificationID
}
