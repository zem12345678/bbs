package grpc

import (
	"context"
	"testing"
	"time"

	pb "user-service/api/proto/userpb"
	"user-service/internal/application/user/command"
	domain "user-service/internal/domain/user"
)

func TestListLoginEventsForwardsExportKeysetFields(t *testing.T) {
	repo := &loginEventKeysetHandlerRepo{}
	h := NewHandler(command.NewService(repo, nil, nil, nil, "test-secret", time.Hour, 8, nil, nil, nil), nil)

	response, err := h.ListLoginEvents(context.Background(), &pb.ListLoginEventsRequest{
		UserId: 42, Limit: 100, AfterId: 700, AscendingById: true,
	})
	if err != nil || len(response.GetItems()) != 1 || response.GetItems()[0].GetId() != "701" {
		t.Fatalf("ListLoginEvents() response = %+v, error = %v", response, err)
	}
	if repo.userID != 42 || repo.afterID != 700 || repo.limit != 100 || repo.legacyCalled {
		t.Fatalf("repository call = user %d after %d limit %d legacy %t", repo.userID, repo.afterID, repo.limit, repo.legacyCalled)
	}
}

type loginEventKeysetHandlerRepo struct {
	domain.Repository
	userID       int64
	afterID      int64
	limit        int
	legacyCalled bool
}

func (r *loginEventKeysetHandlerRepo) ListLoginEventsAfterID(_ context.Context, userID, afterID int64, limit int) ([]domain.LoginEvent, error) {
	r.userID, r.afterID, r.limit = userID, afterID, limit
	return []domain.LoginEvent{{ID: 701, UserID: userID, Success: true, CreatedAt: time.Unix(1_700_000_000, 0)}}, nil
}

func (r *loginEventKeysetHandlerRepo) ListLoginEvents(context.Context, int64, int) ([]domain.LoginEvent, error) {
	r.legacyCalled = true
	return nil, nil
}

func (*loginEventKeysetHandlerRepo) RecordSession(context.Context, domain.UserSession, domain.LoginEvent) error {
	return nil
}
func (*loginEventKeysetHandlerRepo) CreateAPIToken(context.Context, domain.UserSession) error {
	return nil
}
func (*loginEventKeysetHandlerRepo) RecordLoginEvent(context.Context, domain.LoginEvent) error {
	return nil
}
func (*loginEventKeysetHandlerRepo) ListSessions(context.Context, int64, int) ([]domain.UserSession, error) {
	return nil, nil
}
func (*loginEventKeysetHandlerRepo) ListAPITokens(context.Context, int64, int, int) ([]domain.UserSession, int64, error) {
	return nil, 0, nil
}
func (*loginEventKeysetHandlerRepo) RevokeAPIToken(context.Context, int64, string, time.Time) (domain.UserSession, error) {
	return domain.UserSession{}, nil
}
func (*loginEventKeysetHandlerRepo) GetSession(context.Context, int64, string) (domain.UserSession, error) {
	return domain.UserSession{}, nil
}
func (*loginEventKeysetHandlerRepo) RevokeSession(context.Context, int64, string, time.Time) (domain.UserSession, error) {
	return domain.UserSession{}, nil
}

var _ domain.SessionRepository = (*loginEventKeysetHandlerRepo)(nil)
var _ domain.LoginEventKeysetRepository = (*loginEventKeysetHandlerRepo)(nil)
