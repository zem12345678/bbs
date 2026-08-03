package grpc

import (
	"context"
	"testing"
	"time"

	pb "reaction-service/api/proto/reactionpb"
	"reaction-service/internal/application/reaction/command"
	"reaction-service/internal/application/reaction/query"
	domain "reaction-service/internal/domain/reaction"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func TestCollectionHandlersForwardOwnerAndPreserveChanged(t *testing.T) {
	repo := &collectionRepositoryStub{}
	h := newCollectionHandler(repo)

	created, err := h.CreateCollection(context.Background(), &pb.CreateCollectionRequest{
		UserId: 42, Name: "  Reading  ", Description: "  Later  ", IsPublic: true,
	})
	if err != nil {
		t.Fatalf("create collection: %v", err)
	}
	if created.GetCollection().GetUserId() != 42 || created.GetCollection().GetName() != "Reading" {
		t.Fatalf("created collection = %+v", created.GetCollection())
	}

	added, err := h.AddCollectionItem(context.Background(), &pb.CollectionItemRequest{
		UserId: 42, CollectionId: 7,
		Entity: &pb.EntityRef{EntityType: "article", EntityId: 99},
	})
	if err != nil {
		t.Fatalf("add collection item: %v", err)
	}
	if !added.GetChanged() || repo.lastUserID != 42 || repo.lastCollectionID != 7 {
		t.Fatalf("add response = %+v, owner = %d, collection = %d", added, repo.lastUserID, repo.lastCollectionID)
	}

	removed, err := h.RemoveCollectionItem(context.Background(), &pb.CollectionItemRequest{
		UserId: 42, CollectionId: 7,
		Entity: &pb.EntityRef{EntityType: "article", EntityId: 99},
	})
	if err != nil {
		t.Fatalf("remove collection item: %v", err)
	}
	if removed.GetChanged() {
		t.Fatal("idempotent remove should preserve changed=false")
	}
}

func TestCollectionHandlersMapValidationAndOwnershipErrors(t *testing.T) {
	repo := &collectionRepositoryStub{listItemsErr: domain.ErrCollectionNotFound}
	h := newCollectionHandler(repo)

	_, err := h.CreateCollection(context.Background(), &pb.CreateCollectionRequest{UserId: 42, Name: " "})
	if status.Code(err) != codes.InvalidArgument {
		t.Fatalf("blank name status = %s, want %s", status.Code(err), codes.InvalidArgument)
	}

	_, err = h.ListCollectionItems(context.Background(), &pb.ListCollectionItemsRequest{UserId: 42, CollectionId: 99})
	if status.Code(err) != codes.NotFound {
		t.Fatalf("foreign collection status = %s, want %s", status.Code(err), codes.NotFound)
	}
}

func TestListCollectionItemsMapsEntityAndMilliseconds(t *testing.T) {
	createdAt := time.UnixMilli(1_800_000_000_123)
	repo := &collectionRepositoryStub{items: []*domain.CollectionItem{{
		ID: 11, CollectionID: 7, Entity: domain.EntityRef{Type: domain.EntityTopic, ID: 88}, CreatedAt: createdAt,
	}}}
	h := newCollectionHandler(repo)

	response, err := h.ListCollectionItems(context.Background(), &pb.ListCollectionItemsRequest{UserId: 42, CollectionId: 7, Limit: 20})
	if err != nil {
		t.Fatalf("list collection items: %v", err)
	}
	if len(response.GetItems()) != 1 || response.GetItems()[0].GetEntity().GetEntityId() != 88 {
		t.Fatalf("items = %+v", response.GetItems())
	}
	if response.GetItems()[0].GetCreatedAt() != createdAt.UnixMilli() {
		t.Fatalf("created_at = %d", response.GetItems()[0].GetCreatedAt())
	}
}

func newCollectionHandler(repo domain.CollectionRepository) *Handler {
	cmd := command.NewService(nil, nil, nil, nil, repo, nil, nil)
	qry := query.NewService(nil, nil, nil, nil, repo)
	return NewHandler(cmd, qry)
}

type collectionRepositoryStub struct {
	items            []*domain.CollectionItem
	listItemsErr     error
	lastUserID       int64
	lastCollectionID int64
}

func (r *collectionRepositoryStub) CreateCollection(_ context.Context, collection *domain.Collection) error {
	now := time.UnixMilli(1_800_000_000_000)
	collection.ID = 7
	collection.CreatedAt = now
	collection.UpdatedAt = now
	return nil
}

func (r *collectionRepositoryStub) UpdateCollection(_ context.Context, userID, collectionID int64, name, description string, isPublic bool) (*domain.Collection, error) {
	return &domain.Collection{ID: collectionID, UserID: userID, Name: name, Description: description, IsPublic: isPublic}, nil
}

func (r *collectionRepositoryStub) DeleteCollection(context.Context, int64, int64) error { return nil }

func (r *collectionRepositoryStub) ListCollections(context.Context, int64, int, int) ([]*domain.Collection, int64, error) {
	return nil, 0, nil
}

func (r *collectionRepositoryStub) AddCollectionItem(_ context.Context, userID, collectionID int64, _ domain.EntityRef) (bool, error) {
	r.lastUserID = userID
	r.lastCollectionID = collectionID
	return true, nil
}

func (r *collectionRepositoryStub) RemoveCollectionItem(_ context.Context, userID, collectionID int64, _ domain.EntityRef) (bool, error) {
	r.lastUserID = userID
	r.lastCollectionID = collectionID
	return false, nil
}

func (r *collectionRepositoryStub) ListCollectionItems(context.Context, int64, int64, domain.EntityType, int, int) ([]*domain.CollectionItem, int64, error) {
	return r.items, int64(len(r.items)), r.listItemsErr
}
