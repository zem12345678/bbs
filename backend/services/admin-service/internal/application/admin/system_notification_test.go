package admin

import (
	"context"
	"errors"
	"reflect"
	"strings"
	"testing"

	domain "admin/internal/domain/admin"
)

func TestSendSystemNotificationAuthorizesDedicatedPermissionAndDelegates(t *testing.T) {
	authorizer := &systemNotificationAuthorizer{}
	gateway := &fakeSystemNotificationGateway{delivered: 2}
	users := &systemNotificationUserGateway{existing: map[int64]struct{}{101: {}, 202: {}}}
	service := &Service{auth: authorizer, users: users, notifications: gateway}
	actor := domain.Actor{ID: 9, Username: "admin"}
	command := domain.SystemNotificationCommand{
		RecipientIDs:   []int64{101, 202},
		Title:          "系统维护",
		Content:        "今晚维护",
		IdempotencyKey: "maintenance-20260725",
	}

	delivered, err := service.SendSystemNotification(t.Context(), actor, command)
	if err != nil {
		t.Fatalf("SendSystemNotification() error = %v", err)
	}
	if delivered != 2 {
		t.Fatalf("delivered = %d, want 2", delivered)
	}
	if !reflect.DeepEqual(authorizer.actions, []domain.Action{domain.ActionSendSystemNotification}) {
		t.Fatalf("authorized actions = %v", authorizer.actions)
	}
	if !gateway.called || gateway.actorID != actor.ID || !reflect.DeepEqual(gateway.command, command) {
		t.Fatalf("gateway invocation = %#v", gateway)
	}
	if !reflect.DeepEqual(users.requests, [][]int64{{101, 202}}) {
		t.Fatalf("recipient validation requests = %v", users.requests)
	}
}

func TestSendSystemNotificationRejectsUnauthorizedActor(t *testing.T) {
	authorizer := &systemNotificationAuthorizer{err: domain.ErrPermissionDenied}
	gateway := &fakeSystemNotificationGateway{}
	service := &Service{auth: authorizer, notifications: gateway}

	_, err := service.SendSystemNotification(t.Context(), domain.Actor{ID: 9, Username: "moderator"}, domain.SystemNotificationCommand{RecipientIDs: []int64{101}})
	if !errors.Is(err, domain.ErrPermissionDenied) {
		t.Fatalf("SendSystemNotification() error = %v, want ErrPermissionDenied", err)
	}
	if gateway.called {
		t.Fatal("notification gateway must not be called without permission")
	}
}

func TestSendSystemNotificationFailsWhenInternalGatewayIsUnavailable(t *testing.T) {
	service := &Service{auth: &systemNotificationAuthorizer{}}
	_, err := service.SendSystemNotification(t.Context(), domain.Actor{ID: 9, Username: "admin"}, domain.SystemNotificationCommand{RecipientIDs: []int64{101}})
	if !errors.Is(err, domain.ErrSystemNotificationUnavailable) {
		t.Fatalf("SendSystemNotification() error = %v, want ErrSystemNotificationUnavailable", err)
	}
}

func TestSendSystemNotificationRejectsMissingRecipientsWithoutDispatch(t *testing.T) {
	gateway := &fakeSystemNotificationGateway{}
	users := &systemNotificationUserGateway{existing: map[int64]struct{}{101: {}}}
	service := &Service{auth: &systemNotificationAuthorizer{}, users: users, notifications: gateway}

	_, err := service.SendSystemNotification(t.Context(), domain.Actor{ID: 9, Username: "admin"}, domain.SystemNotificationCommand{
		RecipientIDs:   []int64{101, 404},
		Title:          "系统维护",
		Content:        "今晚维护",
		IdempotencyKey: "maintenance-20260725",
	})
	if !errors.Is(err, domain.ErrSystemNotificationRecipientsNotFound) {
		t.Fatalf("SendSystemNotification() error = %v, want missing recipient error", err)
	}
	if !strings.Contains(err.Error(), "404") {
		t.Fatalf("error = %q, want missing recipient ID", err)
	}
	if gateway.called {
		t.Fatal("notification gateway must not be called when a recipient is missing")
	}
}

func TestSendSystemNotificationDoesNotDispatchWhenRecipientValidationFails(t *testing.T) {
	gateway := &fakeSystemNotificationGateway{}
	lookupErr := errors.New("user service unavailable")
	users := &systemNotificationUserGateway{err: lookupErr}
	service := &Service{auth: &systemNotificationAuthorizer{}, users: users, notifications: gateway}

	_, err := service.SendSystemNotification(t.Context(), domain.Actor{ID: 9, Username: "admin"}, domain.SystemNotificationCommand{
		RecipientIDs:   []int64{101},
		Title:          "系统维护",
		Content:        "今晚维护",
		IdempotencyKey: "maintenance-20260725",
	})
	if !errors.Is(err, lookupErr) {
		t.Fatalf("SendSystemNotification() error = %v, want %v", err, lookupErr)
	}
	if gateway.called {
		t.Fatal("notification gateway must not be called when recipient validation fails")
	}
}

type systemNotificationAuthorizer struct {
	actions []domain.Action
	err     error
}

func (a *systemNotificationAuthorizer) Authorize(_ context.Context, _ domain.Actor, action domain.Action) error {
	a.actions = append(a.actions, action)
	return a.err
}

func (*systemNotificationAuthorizer) Reload(context.Context) error { return nil }

type fakeSystemNotificationGateway struct {
	called    bool
	actorID   int64
	command   domain.SystemNotificationCommand
	delivered int32
	err       error
}

func (g *fakeSystemNotificationGateway) DispatchSystemNotifications(_ context.Context, actorID int64, command domain.SystemNotificationCommand) (int32, error) {
	g.called = true
	g.actorID = actorID
	g.command = command
	return g.delivered, g.err
}

type systemNotificationUserGateway struct {
	UserGateway
	existing map[int64]struct{}
	err      error
	requests [][]int64
}

func (g *systemNotificationUserGateway) ExistingUserIDs(_ context.Context, ids []int64) (map[int64]struct{}, error) {
	g.requests = append(g.requests, append([]int64(nil), ids...))
	if g.err != nil {
		return nil, g.err
	}
	return g.existing, nil
}
