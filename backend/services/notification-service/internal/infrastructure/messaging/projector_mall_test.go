package messaging

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"

	app "notification-service/internal/application/notification"
	domain "notification-service/internal/domain/notification"

	"github.com/segmentio/kafka-go"
)

func TestMallDigitalEntitlementRevokedPayloadContract(t *testing.T) {
	payloadJSON := []byte(`{
		"event_id":"evt-entitlement-revoked",
		"event_type":"mall.digital_entitlement.revoked.v1",
		"occurred_at_unix_ms":1784025000000,
		"entitlement_id":503,
		"order_id":8804,
		"order_no":"MO202607140001",
		"user_id":42,
		"product_id":101,
		"sku":"VIP-MONTH",
		"title":"会员月卡",
		"fulfillment_code":"BBS-VIP-503",
		"grant_type":"membership",
		"grant_key":"vip-month",
		"status":"REVOKED",
		"operator_id":"admin-7",
		"reason":"risk review"
	}`)

	var payload mallDigitalEntitlementRevokedPayload
	if err := json.Unmarshal(payloadJSON, &payload); err != nil {
		t.Fatalf("unmarshal mall entitlement revoked payload: %v", err)
	}
	if payload.EventID != "evt-entitlement-revoked" || payload.EntitlementID != 503 || payload.OrderID != 8804 || payload.UserID != 42 {
		t.Fatalf("payload identifiers = %+v", payload)
	}
	if payload.GrantType != "membership" || payload.GrantKey != "vip-month" || payload.Status != "REVOKED" {
		t.Fatalf("payload entitlement state = %+v", payload)
	}
	if payload.FulfillmentCode != "BBS-VIP-503" || payload.OperatorID != "admin-7" || payload.Reason != "risk review" {
		t.Fatalf("payload notification details = %+v", payload)
	}
}

func TestProjectorHandlesMallDigitalEntitlementRevoked(t *testing.T) {
	t.Parallel()

	repo := &mallProjectorRepo{}
	projector := NewProjector(app.NewService(repo))
	occurredAtUnixMs := int64(1784025000000)
	payload, err := json.Marshal(mallDigitalEntitlementRevokedPayload{
		EventID:          "evt-entitlement-revoked",
		OccurredAtUnixMs: occurredAtUnixMs,
		EntitlementID:    503,
		OrderID:          8804,
		OrderNo:          "MO202607140001",
		UserID:           42,
		ProductID:        101,
		SKU:              "VIP-MONTH",
		Title:            "会员月卡",
		FulfillmentCode:  "BBS-VIP-503",
		GrantType:        "membership",
		GrantKey:         "vip-month",
		Status:           "REVOKED",
		OperatorID:       "admin-7",
		Reason:           "risk review",
	})
	if err != nil {
		t.Fatalf("marshal payload: %v", err)
	}

	if err := projector.HandleMall(context.Background(), eventEnvelope{
		EventType: "mall.digital_entitlement.revoked.v1",
		Payload:   payload,
	}); err != nil {
		t.Fatalf("handle mall entitlement revoked: %v", err)
	}

	if len(repo.created) != 1 {
		t.Fatalf("created notifications = %d, want 1", len(repo.created))
	}
	item := repo.created[0]
	if item.UserID != 42 || item.Type != "mall_digital_entitlement_revoked" || item.EntityType != "mall_order" || item.EntityID != 8804 || item.SourceID != 503 {
		t.Fatalf("notification = %+v", item)
	}
	if item.Title != "数字权益已撤销" {
		t.Fatalf("title = %q", item.Title)
	}
	for _, expected := range []string{"MO202607140001", "会员月卡", "vip-month", "BBS-VIP-503", "risk review", "admin-7"} {
		if !strings.Contains(item.Content, expected) {
			t.Fatalf("content = %q, want %q", item.Content, expected)
		}
	}
	if len(repo.sourceEventIDs) != 1 || repo.sourceEventIDs[0] != "evt-entitlement-revoked" {
		t.Fatalf("source event IDs = %+v", repo.sourceEventIDs)
	}
	if len(repo.createdAt) != 1 || !repo.createdAt[0].Equal(time.UnixMilli(occurredAtUnixMs)) {
		t.Fatalf("createdAt = %+v, want %s", repo.createdAt, time.UnixMilli(occurredAtUnixMs))
	}
}

func TestProjectorHandlesMallOrderPaidKafkaEnvelopeWithDigitalEntitlements(t *testing.T) {
	t.Parallel()

	const occurredAtUnixMs = int64(1784025000000)
	message := kafka.Message{
		Value: []byte(`{
			"event_id":"evt-mall-paid-digital",
			"event_type":"mall.order.paid.v1",
			"occurred_at_unix_ms":1784025000000,
			"order_id":8802,
			"order_no":"MO202607080002",
			"user_id":42,
			"total_credits":360,
			"payment_method":"credits",
			"payment_id":501,
			"items":[{
				"product_id":101,
				"sku":"BADGE-FOUNDER",
				"title":"创始会员徽章",
				"category":"digital",
				"quantity":1,
				"unit_price_credits":360,
				"subtotal_credits":360
			}],
			"digital_entitlements":[{
				"product_id":101,
				"sku":"BADGE-FOUNDER",
				"title":"创始会员徽章",
				"quantity":1,
				"fulfillment_code":"BBS-ENTITLEMENT-8802",
				"grant_type":"badge",
				"grant_key":"badge-founder",
				"status":"ACTIVE"
			}]
		}`),
		Headers: []kafka.Header{
			{Key: "event_type", Value: []byte("mall.order.paid.v1")},
			{Key: "producer", Value: []byte("mall-service")},
		},
	}
	var env eventEnvelope
	if err := decodeKafkaEnvelope(message, &env); err != nil {
		t.Fatalf("decode mall paid Kafka envelope: %v", err)
	}

	repo := &mallProjectorRepo{}
	projector := NewProjector(app.NewService(repo))
	if err := projector.HandleMall(context.Background(), env); err != nil {
		t.Fatalf("handle mall paid event: %v", err)
	}

	if len(repo.created) != 1 {
		t.Fatalf("created notifications = %d, want 1", len(repo.created))
	}
	item := repo.created[0]
	if item.UserID != 42 || item.Type != "mall_order_paid" || item.EntityType != "mall_order" || item.EntityID != 8802 || item.SourceID != 8802 {
		t.Fatalf("notification = %+v", item)
	}
	if item.Title != "订单已支付" {
		t.Fatalf("title = %q", item.Title)
	}
	for _, expected := range []string{
		"订单 MO202607080002 已支付 360 积分",
		"支付方式：credits",
		"数字权益已发放：创始会员徽章 / 徽章 badge-founder / BBS-ENTITLEMENT-8802",
	} {
		if !strings.Contains(item.Content, expected) {
			t.Fatalf("content = %q, want %q", item.Content, expected)
		}
	}
	if len(repo.sourceEventIDs) != 1 || repo.sourceEventIDs[0] != "evt-mall-paid-digital" {
		t.Fatalf("source event IDs = %+v", repo.sourceEventIDs)
	}
	if len(repo.createdAt) != 1 || !repo.createdAt[0].Equal(time.UnixMilli(occurredAtUnixMs)) {
		t.Fatalf("createdAt = %+v, want %s", repo.createdAt, time.UnixMilli(occurredAtUnixMs))
	}
}

type mallProjectorRepo struct {
	created        []domain.Notification
	sourceEventIDs []string
	createdAt      []time.Time
	webhookEvents  []webhookEventRecord
}

type webhookEventRecord struct {
	userID    int64
	eventType string
	eventID   string
	payload   []byte
	createdAt time.Time
}

func (r *mallProjectorRepo) EnsureSchema(context.Context) error { return nil }

func (r *mallProjectorRepo) EraseUserData(context.Context, int64, int64, int32) error { return nil }

func (r *mallProjectorRepo) SaveArticle(context.Context, domain.ArticleRef, time.Time) error {
	return nil
}

func (r *mallProjectorRepo) GetArticle(context.Context, int64) (domain.ArticleRef, error) {
	return domain.ArticleRef{}, nil
}

func (r *mallProjectorRepo) SavePendingArticleNotification(context.Context, string, string, int64, int64, int64, time.Time) error {
	return nil
}

func (r *mallProjectorRepo) FlushPendingArticleNotifications(context.Context, domain.ArticleRef) error {
	return nil
}

func (r *mallProjectorRepo) SaveContent(context.Context, domain.ContentRef, time.Time) error {
	return nil
}

func (r *mallProjectorRepo) GetContent(context.Context, string, int64) (domain.ContentRef, error) {
	return domain.ContentRef{}, nil
}

func (r *mallProjectorRepo) SavePendingContentNotification(context.Context, string, string, string, int64, int64, int64, time.Time) error {
	return nil
}

func (r *mallProjectorRepo) FlushPendingContentNotifications(context.Context, domain.ContentRef) error {
	return nil
}

func (r *mallProjectorRepo) SaveComment(context.Context, domain.CommentRef, time.Time) error {
	return nil
}

func (r *mallProjectorRepo) GetComment(context.Context, int64) (domain.CommentRef, error) {
	return domain.CommentRef{}, nil
}

func (r *mallProjectorRepo) SavePendingReplyNotification(context.Context, string, int64, int64, string, int64, int64, time.Time) error {
	return nil
}

func (r *mallProjectorRepo) FlushPendingReplyNotifications(context.Context, domain.CommentRef) error {
	return nil
}

func (r *mallProjectorRepo) Create(_ context.Context, item domain.Notification, sourceEventID string, createdAt time.Time) error {
	r.created = append(r.created, item)
	r.sourceEventIDs = append(r.sourceEventIDs, sourceEventID)
	r.createdAt = append(r.createdAt, createdAt)
	return nil
}

func (*mallProjectorRepo) CreateSystemNotifications(context.Context, domain.SystemNotificationCommand, time.Time) (int32, error) {
	return 0, nil
}

func (*mallProjectorRepo) ListPreferences(context.Context, int64) ([]domain.NotificationPreference, error) {
	return nil, nil
}

func (*mallProjectorRepo) ReplacePreferences(context.Context, int64, []domain.NotificationPreference) error {
	return nil
}

func (r *mallProjectorRepo) List(context.Context, int64, int32, int32, bool) ([]domain.Notification, int64, int64, error) {
	return nil, 0, 0, nil
}

func (r *mallProjectorRepo) ListCompatibility(context.Context, domain.NotificationCompatibilityQuery) ([]domain.Notification, error) {
	return nil, nil
}

func (r *mallProjectorRepo) UnreadCount(context.Context, int64) (int64, error) { return 0, nil }
func (r *mallProjectorRepo) MarkRead(context.Context, int64, int64) error      { return nil }
func (r *mallProjectorRepo) MarkAllRead(context.Context, int64) error          { return nil }

func (*mallProjectorRepo) CreateWebhook(context.Context, domain.Webhook, int) (domain.Webhook, error) {
	return domain.Webhook{}, nil
}
func (*mallProjectorRepo) ListWebhooks(context.Context, int64) ([]domain.Webhook, error) {
	return nil, nil
}
func (*mallProjectorRepo) GetWebhook(context.Context, int64, int64) (domain.Webhook, error) {
	return domain.Webhook{}, nil
}
func (*mallProjectorRepo) UpdateWebhook(context.Context, domain.Webhook) (domain.Webhook, error) {
	return domain.Webhook{}, nil
}
func (*mallProjectorRepo) DeleteWebhook(context.Context, int64, int64) error { return nil }
func (r *mallProjectorRepo) EnqueueWebhookEvent(_ context.Context, userID int64, eventType, eventID string, payload []byte, createdAt time.Time) error {
	r.webhookEvents = append(r.webhookEvents, webhookEventRecord{userID: userID, eventType: eventType, eventID: eventID, payload: append([]byte(nil), payload...), createdAt: createdAt})
	return nil
}
func (*mallProjectorRepo) EnqueueWebhookTest(context.Context, domain.Webhook, string, string, []byte, time.Time) error {
	return nil
}
