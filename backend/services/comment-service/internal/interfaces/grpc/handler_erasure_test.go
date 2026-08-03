package grpc

import (
	"context"
	"testing"

	pb "comment-service/api/proto/commentpb"
	"comment-service/internal/application/comment/command"
	"comment-service/internal/application/comment/query"
	domain "comment-service/internal/domain/comment"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func TestRedactAccountCommentsHandler(t *testing.T) {
	repo := &erasureRepositoryStub{count: 7}
	handler := NewHandler(command.NewService(repo, nil, nil, nil), query.NewService(repo))

	response, err := handler.RedactAccountComments(context.Background(), &pb.RedactAccountCommentsRequest{
		UserId: 42, DeletionJobId: 91, PolicyVersion: 3,
	})
	if err != nil {
		t.Fatalf("RedactAccountComments() error = %v", err)
	}
	if !response.GetCompleted() || response.GetRedactedComments() != 7 {
		t.Fatalf("RedactAccountComments() response = %+v", response)
	}
	if repo.userID != 42 || repo.jobID != 91 || repo.policyVersion != 3 {
		t.Fatalf("repository request user=%d job=%d policy=%d", repo.userID, repo.jobID, repo.policyVersion)
	}

	_, err = handler.RedactAccountComments(context.Background(), &pb.RedactAccountCommentsRequest{})
	if status.Code(err) != codes.InvalidArgument {
		t.Fatalf("invalid request status = %v, want InvalidArgument", status.Code(err))
	}
}

type erasureRepositoryStub struct {
	userID        int64
	jobID         int64
	policyVersion int32
	count         int64
}

func (r *erasureRepositoryStub) RedactAccountComments(_ context.Context, userID, jobID int64, policyVersion int32) (int64, error) {
	r.userID, r.jobID, r.policyVersion = userID, jobID, policyVersion
	return r.count, nil
}

func (*erasureRepositoryStub) Save(context.Context, *domain.Comment) error { return nil }
func (*erasureRepositoryStub) FindByID(context.Context, int64) (*domain.Comment, error) {
	return nil, domain.ErrNotFound
}
func (*erasureRepositoryStub) ListByEntity(context.Context, domain.ListQuery) ([]*domain.Comment, int64, error) {
	return nil, 0, nil
}
func (*erasureRepositoryStub) ListReplies(context.Context, domain.ReplyListQuery) ([]*domain.Comment, int64, error) {
	return nil, 0, nil
}
func (*erasureRepositoryStub) ListForModeration(context.Context, domain.ModerationListQuery) ([]*domain.Comment, int64, error) {
	return nil, 0, nil
}
func (*erasureRepositoryStub) Hide(context.Context, *domain.Comment) error    { return nil }
func (*erasureRepositoryStub) Restore(context.Context, *domain.Comment) error { return nil }
func (*erasureRepositoryStub) IncrementReplyCount(context.Context, int64, int64) error {
	return nil
}
