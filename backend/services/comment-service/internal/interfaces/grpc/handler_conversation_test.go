package grpc

import (
	"context"
	"testing"

	pb "comment-service/api/proto/commentpb"
	commentquery "comment-service/internal/application/comment/query"
	domain "comment-service/internal/domain/comment"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func TestGetCommentConversationMapsRequestAndResponse(t *testing.T) {
	repo := &handlerConversationRepo{comments: map[int64]*domain.Comment{
		1: {ID: 1, EntityType: string(domain.EntityArticle), EntityID: 10, AuthorID: 20, Content: "root", Status: domain.StatusVisible},
		2: {ID: 2, EntityType: string(domain.EntityArticle), EntityID: 10, RootID: 1, ParentID: 1, AuthorID: 21, Content: "reply", Status: domain.StatusVisible},
		3: {ID: 3, EntityType: string(domain.EntityArticle), EntityID: 10, RootID: 1, ParentID: 2, AuthorID: 22, Content: "nested", Status: domain.StatusVisible},
	}}
	handler := NewHandler(nil, commentquery.NewService(repo))

	got, err := handler.GetCommentConversation(context.Background(), &pb.GetCommentConversationRequest{CommentId: 3, Limit: 1, Offset: 1})
	if err != nil {
		t.Fatalf("GetCommentConversation() error = %v", err)
	}
	if got.GetTotal() != 1 || len(got.GetItems()) != 1 || got.GetItems()[0].GetId() != 1 {
		t.Fatalf("response = %+v, want root only", got)
	}
}

func TestGetCommentConversationMapsErrors(t *testing.T) {
	handler := NewHandler(nil, commentquery.NewService(&handlerConversationRepo{comments: map[int64]*domain.Comment{}}))

	_, err := handler.GetCommentConversation(context.Background(), &pb.GetCommentConversationRequest{CommentId: 1, Limit: 0})
	if status.Code(err) != codes.InvalidArgument {
		t.Fatalf("invalid limit status = %v", status.Code(err))
	}
	_, err = handler.GetCommentConversation(context.Background(), &pb.GetCommentConversationRequest{CommentId: 1, Limit: 1})
	if status.Code(err) != codes.NotFound {
		t.Fatalf("missing comment status = %v", status.Code(err))
	}
}

type handlerConversationRepo struct {
	domain.Repository
	comments map[int64]*domain.Comment
}

func (r *handlerConversationRepo) FindByID(_ context.Context, id int64) (*domain.Comment, error) {
	comment, ok := r.comments[id]
	if !ok {
		return nil, domain.ErrNotFound
	}
	return comment, nil
}
