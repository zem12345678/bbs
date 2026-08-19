package grpc

import (
	"context"
	"testing"

	pb "notification-service/api/proto/notificationpb"
	app "notification-service/internal/application/notification"
	domain "notification-service/internal/domain/notification"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func TestFlushDelegatesAuthenticatedUser(t *testing.T) {
	repo := &flushHandlerRepository{}
	handler := NewHandler(app.NewService(repo))

	response, err := handler.Flush(t.Context(), &pb.FlushRequest{UserId: 42})
	if err != nil {
		t.Fatalf("Flush() error = %v", err)
	}
	if !response.GetSuccess() || repo.userID != 42 {
		t.Fatalf("response = %#v, repository user ID = %d", response, repo.userID)
	}
}

func TestFlushRejectsInvalidUser(t *testing.T) {
	repo := &flushHandlerRepository{}
	handler := NewHandler(app.NewService(repo))

	response, err := handler.Flush(t.Context(), &pb.FlushRequest{})
	if status.Code(err) != codes.InvalidArgument {
		t.Fatalf("Flush() error = %v, want InvalidArgument", err)
	}
	if response != nil || repo.called {
		t.Fatalf("response = %#v, repository called = %v", response, repo.called)
	}
}

type flushHandlerRepository struct {
	domain.Repository
	userID int64
	called bool
}

func (r *flushHandlerRepository) Flush(_ context.Context, userID int64) error {
	r.called = true
	r.userID = userID
	return nil
}
