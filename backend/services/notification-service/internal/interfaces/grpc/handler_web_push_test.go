package grpc

import (
	"context"
	"testing"
	"time"

	pb "notification-service/api/proto/notificationpb"
	app "notification-service/internal/application/notification"
	domain "notification-service/internal/domain/notification"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type webPushHandlerRepository struct {
	domain.Repository
	item domain.WebPushSubscription
}

func (r *webPushHandlerRepository) UpsertWebPushSubscription(_ context.Context, item domain.WebPushSubscription, _ int) (domain.WebPushSubscription, error) {
	item.ID = 7
	item.CreatedAt = time.Unix(100, 0)
	item.UpdatedAt = time.Unix(200, 0)
	r.item = item
	return item, nil
}

func (r *webPushHandlerRepository) GetWebPushSubscription(_ context.Context, userID int64, endpoint string) (domain.WebPushSubscription, error) {
	if r.item.UserID == userID && r.item.Endpoint == endpoint {
		return r.item, nil
	}
	return domain.WebPushSubscription{}, nil
}

func (r *webPushHandlerRepository) DeleteWebPushSubscription(_ context.Context, userID int64, endpoint string) error {
	if r.item.UserID == userID && r.item.Endpoint == endpoint {
		r.item = domain.WebPushSubscription{}
	}
	return nil
}

func TestWebPushHandlerRoundTripAndPublicConfig(t *testing.T) {
	repo := &webPushHandlerRepository{}
	handler := NewHandler(app.NewService(repo, domain.WebPushConfig{
		Enabled: true, PublicKey: "vapid-public", PrivateKey: "must-not-leak",
	}))
	config, err := handler.GetWebPushConfig(t.Context(), &pb.GetWebPushConfigRequest{})
	if err != nil || !config.GetEnabled() || config.GetPublicKey() != "vapid-public" {
		t.Fatalf("web push config = %#v, err=%v", config, err)
	}
	response, err := handler.RegisterWebPushSubscription(t.Context(), &pb.RegisterWebPushSubscriptionRequest{
		UserId: 42, Endpoint: "https://push.example/subscription", Auth: "auth", PublicKey: "p256dh", SendReadMessage: true,
	})
	if err != nil || !response.GetRegistered() || response.GetState() != "subscribed" || !response.GetSendReadMessage() {
		t.Fatalf("register response = %#v, err=%v", response, err)
	}
	loaded, err := handler.GetWebPushSubscription(t.Context(), &pb.GetWebPushSubscriptionRequest{UserId: 42, Endpoint: "https://push.example/subscription"})
	if err != nil || !loaded.GetRegistered() || loaded.GetCreatedAt() != 100_000 {
		t.Fatalf("get response = %#v, err=%v", loaded, err)
	}
	unregistered, err := handler.UnregisterWebPushSubscription(t.Context(), &pb.UnregisterWebPushSubscriptionRequest{UserId: 42, Endpoint: "https://push.example/subscription"})
	if err != nil || !unregistered.GetSuccess() {
		t.Fatalf("unregister response = %#v, err=%v", unregistered, err)
	}
}

func TestWebPushHandlerRejectsInvalidAndDisabledRegistration(t *testing.T) {
	repo := &webPushHandlerRepository{}
	handler := NewHandler(app.NewService(repo, domain.WebPushConfig{Enabled: true}))
	_, err := handler.RegisterWebPushSubscription(t.Context(), &pb.RegisterWebPushSubscriptionRequest{
		UserId: 42, Endpoint: "http://10.0.0.1/subscription", Auth: "auth", PublicKey: "p256dh",
	})
	if status.Code(err) != codes.InvalidArgument {
		t.Fatalf("invalid endpoint code = %s, err=%v", status.Code(err), err)
	}
	disabled := NewHandler(app.NewService(repo))
	_, err = disabled.RegisterWebPushSubscription(t.Context(), &pb.RegisterWebPushSubscriptionRequest{
		UserId: 42, Endpoint: "https://push.example/subscription", Auth: "auth", PublicKey: "p256dh",
	})
	if status.Code(err) != codes.FailedPrecondition {
		t.Fatalf("disabled registration code = %s, err=%v", status.Code(err), err)
	}
}
