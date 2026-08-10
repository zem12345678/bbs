package notification

import (
	"context"
	"errors"
	"testing"
	"time"

	domain "notification-service/internal/domain/notification"
)

type webPushRepositoryFake struct {
	domain.Repository
	items map[string]domain.WebPushSubscription
}

func (r *webPushRepositoryFake) UpsertWebPushSubscription(_ context.Context, item domain.WebPushSubscription, maxPerUser int) (domain.WebPushSubscription, error) {
	for endpoint, existing := range r.items {
		if existing.UserID == item.UserID && endpoint != item.Endpoint {
			count := 0
			for _, candidate := range r.items {
				if candidate.UserID == item.UserID {
					count++
				}
			}
			if count >= maxPerUser {
				return domain.WebPushSubscription{}, domain.ErrWebPushSubscriptionLimit
			}
		}
	}
	if existing, ok := r.items[item.Endpoint]; ok {
		item.ID = existing.ID
		item.CreatedAt = existing.CreatedAt
	}
	if item.ID == 0 {
		item.ID = int64(len(r.items) + 1)
		item.CreatedAt = time.Now()
	}
	item.UpdatedAt = time.Now()
	r.items[item.Endpoint] = item
	return item, nil
}

func (r *webPushRepositoryFake) GetWebPushSubscription(_ context.Context, userID int64, endpoint string) (domain.WebPushSubscription, error) {
	item := r.items[endpoint]
	if item.UserID != userID {
		return domain.WebPushSubscription{}, nil
	}
	return item, nil
}

func (r *webPushRepositoryFake) DeleteWebPushSubscription(_ context.Context, userID int64, endpoint string) error {
	if item, ok := r.items[endpoint]; ok && item.UserID == userID {
		delete(r.items, endpoint)
	}
	return nil
}

func TestWebPushSubscriptionValidationAndEndpointMigration(t *testing.T) {
	repo := &webPushRepositoryFake{items: map[string]domain.WebPushSubscription{}}
	service := NewService(repo, domain.WebPushConfig{Enabled: true, PublicKey: "public", PrivateKey: "private"})
	ctx := context.Background()

	registered, err := service.RegisterWebPushSubscription(ctx, domain.WebPushSubscription{
		UserID: 42, Endpoint: "http://127.0.0.1:8849/push", Auth: "auth", PublicKey: "p256dh",
	})
	if err != nil || registered.UserID != 42 {
		t.Fatalf("register local subscription = %+v, err=%v", registered, err)
	}
	if _, err := service.RegisterWebPushSubscription(ctx, domain.WebPushSubscription{
		UserID: 42, Endpoint: "http://10.0.0.1/push", Auth: "auth", PublicKey: "p256dh",
	}); !errors.Is(err, domain.ErrInvalidWebPushSubscription) {
		t.Fatalf("remote http endpoint error = %v", err)
	}

	migrated, err := service.RegisterWebPushSubscription(ctx, domain.WebPushSubscription{
		UserID: 99, Endpoint: "http://127.0.0.1:8849/push", Auth: "new-auth", PublicKey: "new-p256dh",
	})
	if err != nil || migrated.UserID != 99 || migrated.Auth != "new-auth" {
		t.Fatalf("endpoint migration = %+v, err=%v", migrated, err)
	}
	old, err := service.GetWebPushSubscription(ctx, 42, "http://127.0.0.1:8849/push")
	if err != nil || old.ID != 0 {
		t.Fatalf("old endpoint owner = %+v, err=%v", old, err)
	}
	current, err := service.GetWebPushSubscription(ctx, 99, "http://127.0.0.1:8849/push")
	if err != nil || current.ID == 0 {
		t.Fatalf("new endpoint owner = %+v, err=%v", current, err)
	}
}

func TestWebPushDisabledAndConfigDoesNotExposePrivateKey(t *testing.T) {
	repo := &webPushRepositoryFake{items: map[string]domain.WebPushSubscription{}}
	service := NewService(repo, domain.WebPushConfig{Enabled: false, PublicKey: "public", PrivateKey: "private"})
	if _, err := service.RegisterWebPushSubscription(context.Background(), domain.WebPushSubscription{UserID: 1, Endpoint: "https://push.example/push", Auth: "a", PublicKey: "p"}); !errors.Is(err, domain.ErrWebPushDisabled) {
		t.Fatalf("disabled registration error = %v", err)
	}
	config := service.GetWebPushConfig()
	if config.PrivateKey != "" || config.PublicKey != "public" || config.Enabled {
		t.Fatalf("public config = %+v", config)
	}
}
