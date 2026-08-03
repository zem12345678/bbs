package grpc

import (
	"context"
	"errors"
	"testing"

	pb "feed-service/api/proto/feedpb"
	"feed-service/internal/application/feed/command"
	domain "feed-service/internal/domain/feed"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type handlerPurgeRepository struct {
	domain.Repository
	purgedItems int64
	err         error
	userID      int64
	called      bool
}

func (r *handlerPurgeRepository) PurgeByAuthor(_ context.Context, userID int64) (int64, error) {
	r.called = true
	r.userID = userID
	return r.purgedItems, r.err
}

func TestPurgeAccountFeed(t *testing.T) {
	repo := &handlerPurgeRepository{purgedItems: 5}
	h := NewHandler(nil, command.NewService(repo))

	response, err := h.PurgeAccountFeed(context.Background(), &pb.PurgeAccountFeedRequest{
		UserId:        41,
		DeletionJobId: 73,
		PolicyVersion: 2,
	})
	if err != nil {
		t.Fatalf("PurgeAccountFeed() error = %v", err)
	}
	if !response.GetCompleted() || response.GetPurgedItems() != 5 {
		t.Fatalf("PurgeAccountFeed() response = %+v, want completed with 5 purged items", response)
	}
	if !repo.called || repo.userID != 41 {
		t.Fatalf("PurgeByAuthor() called = %v with user ID %d, want true with 41", repo.called, repo.userID)
	}
}

func TestPurgeAccountFeedStatusCodes(t *testing.T) {
	tests := []struct {
		name string
		repo *handlerPurgeRepository
		req  *pb.PurgeAccountFeedRequest
		code codes.Code
	}{
		{
			name: "invalid request",
			repo: &handlerPurgeRepository{},
			req:  &pb.PurgeAccountFeedRequest{UserId: 41, DeletionJobId: 73},
			code: codes.InvalidArgument,
		},
		{
			name: "repository failure",
			repo: &handlerPurgeRepository{err: errors.New("redis unavailable")},
			req:  &pb.PurgeAccountFeedRequest{UserId: 41, DeletionJobId: 73, PolicyVersion: 2},
			code: codes.Internal,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			h := NewHandler(nil, command.NewService(tt.repo))
			_, err := h.PurgeAccountFeed(context.Background(), tt.req)
			if got := status.Code(err); got != tt.code {
				t.Fatalf("PurgeAccountFeed() status = %s, want %s", got, tt.code)
			}
			if tt.code == codes.InvalidArgument && tt.repo.called {
				t.Fatal("PurgeByAuthor() called for invalid request")
			}
		})
	}
}
