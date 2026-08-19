package grpc

import (
	"context"
	"testing"

	pb "search-service/api/proto/searchpb"
	"search-service/internal/application/search/command"
	"search-service/internal/application/search/query"
	domain "search-service/internal/domain/search"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func TestHandlerSearchNotesByTagReturnsTypedBBSProjections(t *testing.T) {
	repo := &tagHandlerRepository{hits: []domain.NoteLikeHit{
		{Kind: domain.NoteLikeArticle, Article: &domain.ArticleDocument{ID: 12, Title: "Article", TagNames: []string{"go"}}},
		{Kind: domain.NoteLikeTopic, Topic: &domain.TopicDocument{ID: 11, Title: "Topic", TagNames: []string{"go"}}},
	}}
	handler := NewHandler(command.NewService(repo), query.NewService(repo))
	response, err := handler.SearchNotesByTag(t.Context(), &pb.SearchNotesByTagRequest{Tag: "go"})
	if err != nil {
		t.Fatalf("SearchNotesByTag() error = %v", err)
	}
	if len(response.GetItems()) != 2 || response.GetItems()[0].GetKind() != domain.NoteLikeArticle || response.GetItems()[0].GetArticle().GetId() != 12 || response.GetItems()[0].GetTopic() != nil || response.GetItems()[1].GetKind() != domain.NoteLikeTopic || response.GetItems()[1].GetTopic().GetId() != 11 || response.GetItems()[1].GetArticle() != nil {
		t.Fatalf("response = %#v", response)
	}
}

func TestHandlerSearchNotesByTagRejectsUnsupportedFilter(t *testing.T) {
	repo := &tagHandlerRepository{}
	handler := NewHandler(command.NewService(repo), query.NewService(repo))
	value := false
	_, err := handler.SearchNotesByTag(t.Context(), &pb.SearchNotesByTagRequest{Tag: "go", Reply: &value})
	if status.Code(err) != codes.InvalidArgument {
		t.Fatalf("code = %s, want %s; err=%v", status.Code(err), codes.InvalidArgument, err)
	}
}

type tagHandlerRepository struct {
	domain.Repository
	hits []domain.NoteLikeHit
}

func (r *tagHandlerRepository) SearchByTag(context.Context, domain.SearchByTagCriteria) ([]domain.NoteLikeHit, error) {
	return r.hits, nil
}
