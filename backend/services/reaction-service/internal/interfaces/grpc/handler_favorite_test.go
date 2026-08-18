package grpc

import (
	"context"
	"testing"

	pb "reaction-service/api/proto/reactionpb"
	"reaction-service/internal/application/reaction/query"
	domain "reaction-service/internal/domain/reaction"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func TestListFavoritesUsesExclusiveAscendingIDCursor(t *testing.T) {
	repo := &favoriteKeysetRepositoryStub{rows: []*domain.Favorite{{
		ID: 11, UserID: 42, Entity: domain.EntityRef{Type: domain.EntityArticle, ID: 99},
	}}}
	h := NewHandler(nil, query.NewService(nil, nil, nil, repo, nil), nil)

	response, err := h.ListFavorites(context.Background(), &pb.ListFavoritesRequest{
		UserId: 42, EntityType: "article", Limit: 100, AfterId: 7, AscendingById: true,
	})
	if err != nil {
		t.Fatalf("ListFavorites() error = %v", err)
	}
	if len(response.GetItems()) != 1 || response.GetItems()[0].GetId() != 11 {
		t.Fatalf("items = %+v", response.GetItems())
	}
	if repo.userID != 42 || repo.entityType != domain.EntityArticle || repo.afterID != 7 || repo.limit != 100 {
		t.Fatalf("keyset request = user %d type %s after %d limit %d", repo.userID, repo.entityType, repo.afterID, repo.limit)
	}
}

func TestListFavoritesRejectsNegativeCursor(t *testing.T) {
	repo := &favoriteKeysetRepositoryStub{}
	h := NewHandler(nil, query.NewService(nil, nil, nil, repo, nil), nil)

	_, err := h.ListFavorites(context.Background(), &pb.ListFavoritesRequest{UserId: 42, AfterId: -1, AscendingById: true})
	if status.Code(err) != codes.InvalidArgument {
		t.Fatalf("status = %s, want %s", status.Code(err), codes.InvalidArgument)
	}
}

type favoriteKeysetRepositoryStub struct {
	domain.FavoriteRepository
	rows       []*domain.Favorite
	userID     int64
	entityType domain.EntityType
	afterID    int64
	limit      int
}

func (r *favoriteKeysetRepositoryStub) ListFavoritesAfterID(_ context.Context, userID int64, entityType domain.EntityType, afterID int64, limit int) ([]*domain.Favorite, int64, error) {
	r.userID, r.entityType, r.afterID, r.limit = userID, entityType, afterID, limit
	return r.rows, int64(len(r.rows)), nil
}
