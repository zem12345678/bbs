package admin

import (
	"context"
	"errors"
	"reflect"
	"testing"

	domain "admin/internal/domain/admin"
)

func TestSearchRebuildAuthorizesDedicatedActionsAndDelegates(t *testing.T) {
	authorizer := &searchRebuildAuthorizer{}
	gateway := &fakeSearchRebuildGateway{
		started: domain.SearchRebuildStatus{JobID: "job-1", State: "queued", RequestedBy: 9},
		status:  domain.SearchRebuildStatus{JobID: "job-1", State: "running", RequestedBy: 9},
	}
	service := &Service{auth: authorizer, searchRebuild: gateway}
	actor := domain.Actor{ID: 9, Username: "admin"}

	started, err := service.StartSearchRebuild(t.Context(), actor)
	if err != nil {
		t.Fatalf("StartSearchRebuild() error = %v", err)
	}
	if started.JobID != "job-1" || gateway.startedBy != actor.ID {
		t.Fatalf("started = %#v, requestedBy = %d", started, gateway.startedBy)
	}
	current, err := service.GetSearchRebuildStatus(t.Context(), actor)
	if err != nil {
		t.Fatalf("GetSearchRebuildStatus() error = %v", err)
	}
	if current.State != "running" || !gateway.statusCalled {
		t.Fatalf("status = %#v, called = %v", current, gateway.statusCalled)
	}
	if got, want := authorizer.actions, []domain.Action{domain.ActionRebuildSearch, domain.ActionViewSearchRebuild}; !reflect.DeepEqual(got, want) {
		t.Fatalf("authorized actions = %v, want %v", got, want)
	}
}

func TestSearchRebuildRejectsUnauthorizedActor(t *testing.T) {
	authorizer := &searchRebuildAuthorizer{err: domain.ErrPermissionDenied}
	gateway := &fakeSearchRebuildGateway{}
	service := &Service{auth: authorizer, searchRebuild: gateway}

	_, err := service.StartSearchRebuild(t.Context(), domain.Actor{ID: 9, Username: "moderator"})
	if !errors.Is(err, domain.ErrPermissionDenied) {
		t.Fatalf("StartSearchRebuild() error = %v, want permission denied", err)
	}
	if gateway.startedBy != 0 {
		t.Fatal("search rebuild gateway must not be called without permission")
	}
}

func TestSearchRebuildFailsWhenGatewayIsUnavailable(t *testing.T) {
	service := &Service{auth: &searchRebuildAuthorizer{}}
	_, err := service.GetSearchRebuildStatus(t.Context(), domain.Actor{ID: 9, Username: "admin"})
	if !errors.Is(err, domain.ErrSearchRebuildUnavailable) {
		t.Fatalf("GetSearchRebuildStatus() error = %v, want unavailable", err)
	}
}

type searchRebuildAuthorizer struct {
	actions []domain.Action
	err     error
}

func (a *searchRebuildAuthorizer) Authorize(_ context.Context, _ domain.Actor, action domain.Action) error {
	a.actions = append(a.actions, action)
	return a.err
}

func (*searchRebuildAuthorizer) Reload(context.Context) error { return nil }

type fakeSearchRebuildGateway struct {
	started      domain.SearchRebuildStatus
	status       domain.SearchRebuildStatus
	startedBy    int64
	statusCalled bool
	err          error
}

func (g *fakeSearchRebuildGateway) StartSearchRebuild(_ context.Context, requestedBy int64) (domain.SearchRebuildStatus, error) {
	g.startedBy = requestedBy
	return g.started, g.err
}

func (g *fakeSearchRebuildGateway) GetSearchRebuildStatus(context.Context) (domain.SearchRebuildStatus, error) {
	g.statusCalled = true
	return g.status, g.err
}
