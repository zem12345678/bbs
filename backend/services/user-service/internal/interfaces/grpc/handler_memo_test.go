package grpc

import (
	"context"
	"testing"
	"time"

	pb "user-service/api/proto/userpb"
	"user-service/internal/application/user/command"
	domain "user-service/internal/domain/user"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type handlerMemoRepo struct {
	domain.Repository
	memo string
}

func (r *handlerMemoRepo) FindByID(_ context.Context, id int64) (*domain.User, error) {
	if id != 2 {
		return nil, domain.ErrNotFound
	}
	return &domain.User{ID: 2}, nil
}

func (r *handlerMemoRepo) UpdateUserMemo(_ context.Context, _, _ int64, memo string) error {
	r.memo = memo
	return nil
}

func (r *handlerMemoRepo) GetUserMemo(_ context.Context, _, _ int64) (string, error) {
	return r.memo, nil
}

func TestUserMemoHandlersRoundTripAndMapMissingTarget(t *testing.T) {
	repo := &handlerMemoRepo{}
	handler := NewHandler(command.NewService(repo, nil, nil, nil, "test-secret", time.Hour, 8, nil, nil, nil), nil)

	updated, err := handler.UpdateUserMemo(context.Background(), &pb.UpdateUserMemoRequest{UserId: 1, TargetUserId: 2, Memo: " teammate "})
	if err != nil || !updated.GetSuccess() || repo.memo != "teammate" {
		t.Fatalf("UpdateUserMemo() response = %+v, memo = %q, error = %v", updated, repo.memo, err)
	}
	got, err := handler.GetUserMemo(context.Background(), &pb.GetUserMemoRequest{UserId: 1, TargetUserId: 2})
	if err != nil || got.GetMemo() != "teammate" {
		t.Fatalf("GetUserMemo() response = %+v, error = %v", got, err)
	}
	_, err = handler.UpdateUserMemo(context.Background(), &pb.UpdateUserMemoRequest{UserId: 1, TargetUserId: 99, Memo: "missing"})
	if status.Code(err) != codes.NotFound {
		t.Fatalf("missing target code = %s, want NotFound", status.Code(err))
	}
}
