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

func TestWebhookCRUDAndOutboxLifecycle(t *testing.T) {
	dsn := os.Getenv("BBS_NOTIFICATION_TEST_DSN")
	if dsn == "" {
		t.Skip("set BBS_NOTIFICATION_TEST_DSN to run postgres-backed webhook tests")
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
	userID := int64(9_900_000_000) + seed
	webhook, err := repo.CreateWebhook(ctx, domain.Webhook{
		UserID: userID, Name: "delivery", URL: "https://hooks.example.test/bbs", Secret: "secret", Events: []string{"note", "reply"},
	}, domain.WebhookMaxPerUser)
	if err != nil {
		t.Fatalf("create webhook: %v", err)
	}
	t.Cleanup(func() {
		_, _ = pool.Exec(context.Background(), `DELETE FROM notification_webhook_outbox WHERE user_id = $1`, userID)
		_, _ = pool.Exec(context.Background(), `DELETE FROM notification_webhooks WHERE user_id = $1`, userID)
		_, _ = pool.Exec(context.Background(), `DELETE FROM notifications WHERE user_id = $1`, userID)
		_, _ = pool.Exec(context.Background(), `DELETE FROM notification_erased_users WHERE user_id = $1`, userID)
	})
	if webhook.ID <= 0 || webhook.Events[0] != "note" || webhook.Events[1] != "reply" {
		t.Fatalf("created webhook = %+v", webhook)
	}

	items, err := repo.ListWebhooks(ctx, userID)
	if err != nil || len(items) != 1 {
		t.Fatalf("list webhooks = %+v, err=%v", items, err)
	}
	shown, err := repo.GetWebhook(ctx, userID, webhook.ID)
	if err != nil || shown.Secret != "secret" {
		t.Fatalf("show webhook = %+v, err=%v", shown, err)
	}
	updated, err := repo.UpdateWebhook(ctx, domain.Webhook{ID: webhook.ID, UserID: userID, Name: "updated", URL: webhook.URL, Secret: webhook.Secret, Events: []string{"note"}, Active: false})
	if err != nil || updated.Name != "updated" || updated.Active {
		t.Fatalf("updated webhook = %+v, err=%v", updated, err)
	}
	if err := repo.EnqueueWebhookEvent(ctx, userID, "note", "evt-direct", []byte(`{"note":{"id":"n1"}}`), time.Now().UTC()); err != nil {
		t.Fatalf("enqueue inactive webhook: %v", err)
	}
	assertWebhookCount(t, ctx, pool, 0, webhook.ID, "evt-direct")
	if _, err := repo.UpdateWebhook(ctx, domain.Webhook{ID: webhook.ID, UserID: userID, Name: updated.Name, URL: updated.URL, Secret: updated.Secret, Events: updated.Events, Active: true}); err != nil {
		t.Fatalf("reactivate webhook: %v", err)
	}
	if err := repo.EnqueueWebhookEvent(ctx, userID, "note", "evt-direct", []byte(`{"note":{"id":"n1"}}`), time.Now().UTC()); err != nil {
		t.Fatalf("enqueue active webhook: %v", err)
	}
	assertWebhookCount(t, ctx, pool, 1, webhook.ID, "evt-direct")

	deliveries, err := repo.ClaimWebhookDeliveries(ctx, 100, time.Now().UTC(), time.Minute)
	if err != nil {
		t.Fatalf("claim deliveries = %+v, err=%v", deliveries, err)
	}
	var ownDelivery domain.WebhookDelivery
	otherDeliveries := make([]domain.WebhookDelivery, 0, len(deliveries))
	for _, delivery := range deliveries {
		if delivery.WebhookID == webhook.ID && delivery.EventID == "evt-direct" {
			ownDelivery = delivery
		} else {
			otherDeliveries = append(otherDeliveries, delivery)
		}
	}
	if ownDelivery.ID <= 0 {
		t.Fatalf("claim deliveries did not include own outbox row: %+v", deliveries)
	}
	if err := repo.ReleaseWebhookDeliveries(ctx, otherDeliveries); err != nil {
		t.Fatalf("release unrelated deliveries: %v", err)
	}
	active, err := repo.IsWebhookDeliveryActive(ctx, ownDelivery)
	if err != nil || !active {
		t.Fatalf("claimed delivery active = %v, err=%v", active, err)
	}
	if err := repo.CompleteWebhookDelivery(ctx, ownDelivery, 204, time.Now().UTC()); err != nil {
		t.Fatalf("complete delivery: %v", err)
	}
	active, err = repo.IsWebhookDeliveryActive(ctx, ownDelivery)
	if err != nil || active {
		t.Fatalf("completed delivery active = %v, err=%v", active, err)
	}
	assertWebhookCount(t, ctx, pool, 1, webhook.ID, "evt-direct")
	var status int
	if err := pool.QueryRow(ctx, `SELECT latest_status FROM notification_webhooks WHERE id = $1`, webhook.ID).Scan(&status); err != nil || status != 204 {
		t.Fatalf("latest status = %d, err=%v", status, err)
	}
	if err := repo.EraseUserData(ctx, userID, userID+1000, 1); err != nil {
		t.Fatalf("erase webhook owner: %v", err)
	}
	assertWebhookCount(t, ctx, pool, 0, webhook.ID, "evt-direct")
	if _, err := repo.GetWebhook(ctx, userID, webhook.ID); err != domain.ErrWebhookNotFound {
		t.Fatalf("show erased webhook error = %v, want %v", err, domain.ErrWebhookNotFound)
	}
}

func TestNotificationTriggerBackfillsWebhookOutbox(t *testing.T) {
	dsn := os.Getenv("BBS_NOTIFICATION_TEST_DSN")
	if dsn == "" {
		t.Skip("set BBS_NOTIFICATION_TEST_DSN to run postgres-backed webhook tests")
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
	userID := int64(9_910_000_000) + seed
	item, err := repo.CreateWebhook(ctx, domain.Webhook{UserID: userID, Name: "trigger", URL: "https://hooks.example.test", Events: []string{"reply"}}, 10)
	if err != nil {
		t.Fatalf("create webhook: %v", err)
	}
	t.Cleanup(func() {
		_, _ = pool.Exec(context.Background(), `DELETE FROM notification_webhook_outbox WHERE user_id = $1`, userID)
		_, _ = pool.Exec(context.Background(), `DELETE FROM notification_webhooks WHERE user_id = $1`, userID)
		_, _ = pool.Exec(context.Background(), `DELETE FROM notifications WHERE user_id = $1`, userID)
	})
	eventID := fmt.Sprintf("webhook-trigger-%d", seed)
	if _, err := pool.Exec(ctx, `INSERT INTO notifications(user_id, type, title, content, actor_id, entity_type, entity_id, source_id, source_event_id) VALUES($1, 'comment', 'comment', 'body', 12, 'article', 12, 12, $2)`, userID, eventID); err != nil {
		t.Fatalf("insert notification: %v", err)
	}
	assertWebhookCount(t, ctx, pool, 1, item.ID, eventID)
}

func TestWebhookCreateIsRejectedAfterUserErasure(t *testing.T) {
	dsn := os.Getenv("BBS_NOTIFICATION_TEST_DSN")
	if dsn == "" {
		t.Skip("set BBS_NOTIFICATION_TEST_DSN to run postgres-backed webhook tests")
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
	userID := int64(9_920_000_000) + time.Now().UnixNano()%100_000_000
	t.Cleanup(func() {
		_, _ = pool.Exec(context.Background(), `DELETE FROM notification_webhook_outbox WHERE user_id = $1`, userID)
		_, _ = pool.Exec(context.Background(), `DELETE FROM notification_webhooks WHERE user_id = $1`, userID)
		_, _ = pool.Exec(context.Background(), `DELETE FROM notification_erased_users WHERE user_id = $1`, userID)
	})
	if _, err := repo.CreateWebhook(ctx, domain.Webhook{UserID: userID, Name: "before-erasure", URL: "https://hooks.example.test", Events: []string{"note"}}, 10); err != nil {
		t.Fatalf("create initial webhook: %v", err)
	}
	if err := repo.EraseUserData(ctx, userID, userID+1000, 1); err != nil {
		t.Fatalf("erase user: %v", err)
	}
	if _, err := repo.CreateWebhook(ctx, domain.Webhook{UserID: userID, Name: "after-erasure", URL: "https://hooks.example.test", Secret: "must-not-persist", Events: []string{"note"}}, 10); err != domain.ErrInvalidWebhook {
		t.Fatalf("create after erasure error = %v, want %v", err, domain.ErrInvalidWebhook)
	}
	var count int
	if err := pool.QueryRow(ctx, `SELECT COUNT(*) FROM notification_webhooks WHERE user_id = $1`, userID).Scan(&count); err != nil {
		t.Fatalf("count erased webhooks: %v", err)
	}
	if count != 0 {
		t.Fatalf("erased webhook count = %d, want 0", count)
	}
}

func assertWebhookCount(t *testing.T, ctx context.Context, pool *pgxpool.Pool, want int, webhookID int64, eventID string) {
	t.Helper()
	var got int
	if err := pool.QueryRow(ctx, `SELECT COUNT(*) FROM notification_webhook_outbox WHERE webhook_id = $1 AND event_id = $2`, webhookID, eventID).Scan(&got); err != nil {
		t.Fatalf("count webhook outbox: %v", err)
	}
	if got != want {
		t.Fatalf("webhook outbox count = %d, want %d", got, want)
	}
}
