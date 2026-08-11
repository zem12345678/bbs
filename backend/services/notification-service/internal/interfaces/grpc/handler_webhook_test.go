package grpc

import (
	"context"
	"testing"
	"time"

	pb "notification-service/api/proto/notificationpb"
	app "notification-service/internal/application/notification"
	domain "notification-service/internal/domain/notification"
)

func TestWebhookHandlerRoundTripAndTestQueue(t *testing.T) {
	repo := &webhookHandlerRepository{}
	service := app.NewService(repo).SetWebhookConfig(domain.WebhookConfig{Enabled: true, ServerURL: "https://bbs.example.test"})
	handler := NewHandler(service)

	created, err := handler.CreateWebhook(t.Context(), &pb.CreateWebhookRequest{UserId: 42, Name: "Delivery", Url: "https://hooks.example.test/bbs", Secret: "shared", On: []string{"note", "reply"}})
	if err != nil {
		t.Fatalf("CreateWebhook() error = %v", err)
	}
	if created.GetWebhook().GetId() != 101 || created.GetWebhook().GetUserId() != 42 || created.GetWebhook().GetSecret() != "shared" {
		t.Fatalf("created webhook = %#v", created.GetWebhook())
	}

	listed, err := handler.ListWebhooks(t.Context(), &pb.ListWebhooksRequest{UserId: 42})
	if err != nil || len(listed.GetItems()) != 1 {
		t.Fatalf("ListWebhooks() = %#v, err=%v", listed, err)
	}
	shown, err := handler.ShowWebhook(t.Context(), &pb.ShowWebhookRequest{UserId: 42, WebhookId: 101})
	if err != nil || shown.GetWebhook().GetName() != "Delivery" {
		t.Fatalf("ShowWebhook() = %#v, err=%v", shown, err)
	}
	updated, err := handler.UpdateWebhook(t.Context(), &pb.UpdateWebhookRequest{UserId: 42, WebhookId: 101, Active: false, ActiveSet: true})
	if err != nil || updated.GetWebhook().GetActive() || updated.GetWebhook().GetSecret() != "shared" {
		t.Fatalf("UpdateWebhook() = %#v, err=%v", updated, err)
	}
	cleared, err := handler.UpdateWebhook(t.Context(), &pb.UpdateWebhookRequest{UserId: 42, WebhookId: 101, Secret: "", SecretSet: true})
	if err != nil || cleared.GetWebhook().GetSecret() != "" {
		t.Fatalf("clear webhook secret = %#v, err=%v", cleared, err)
	}
	tested, err := handler.TestWebhook(t.Context(), &pb.TestWebhookRequest{UserId: 42, WebhookId: 101, Type: "reaction"})
	if err != nil || !tested.GetSuccess() || repo.testEventType != "reaction" || len(repo.testPayload) == 0 {
		t.Fatalf("TestWebhook() = %#v, event=%q, payload=%q, err=%v", tested, repo.testEventType, repo.testPayload, err)
	}
	deleted, err := handler.DeleteWebhook(t.Context(), &pb.DeleteWebhookRequest{UserId: 42, WebhookId: 101})
	if err != nil || !deleted.GetSuccess() || !repo.deleted {
		t.Fatalf("DeleteWebhook() = %#v, deleted=%t, err=%v", deleted, repo.deleted, err)
	}
}

type webhookHandlerRepository struct {
	domain.Repository
	item          domain.Webhook
	testEventType string
	testPayload   []byte
	deleted       bool
}

func (r *webhookHandlerRepository) CreateWebhook(_ context.Context, item domain.Webhook, _ int) (domain.Webhook, error) {
	now := time.Now().UTC()
	item.ID = 101
	item.Active = true
	item.CreatedAt = now
	item.UpdatedAt = now
	r.item = item
	return item, nil
}
func (r *webhookHandlerRepository) ListWebhooks(context.Context, int64) ([]domain.Webhook, error) {
	return []domain.Webhook{r.item}, nil
}
func (r *webhookHandlerRepository) GetWebhook(_ context.Context, userID, webhookID int64) (domain.Webhook, error) {
	if r.item.UserID != userID || r.item.ID != webhookID || r.deleted {
		return domain.Webhook{}, domain.ErrWebhookNotFound
	}
	return r.item, nil
}
func (r *webhookHandlerRepository) UpdateWebhook(_ context.Context, item domain.Webhook) (domain.Webhook, error) {
	r.item = item
	return item, nil
}
func (r *webhookHandlerRepository) DeleteWebhook(context.Context, int64, int64) error {
	r.deleted = true
	return nil
}
func (*webhookHandlerRepository) EnqueueWebhookEvent(context.Context, int64, string, string, []byte, time.Time) error {
	return nil
}
func (r *webhookHandlerRepository) EnqueueWebhookTest(_ context.Context, _ domain.Webhook, eventType, _ string, payload []byte, _ time.Time) error {
	r.testEventType = eventType
	r.testPayload = append([]byte(nil), payload...)
	return nil
}
