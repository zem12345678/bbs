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

func TestCollectionHandlersUseExclusiveAscendingIDCursor(t *testing.T) {
	repo := &collectionRepositoryStub{
		collections: []*domain.Collection{{ID: 11, UserID: 42, Name: "Later"}},
		items: []*domain.CollectionItem{{
			ID: 21, CollectionID: 11, Entity: domain.EntityRef{Type: domain.EntityArticle, ID: 88},
		}},
	}
	h := newCollectionHandler(repo)

	collections, err := h.ListCollections(context.Background(), &pb.ListCollectionsRequest{
		UserId: 42, Limit: 10, AfterId: 7, AscendingById: true,
	})
	if err != nil {
		t.Fatalf("list collections by ID: %v", err)
	}
	if len(collections.GetItems()) != 1 || collections.GetItems()[0].GetId() != 11 || repo.collectionsAfterID != 7 {
		t.Fatalf("collections = %+v, after ID = %d", collections.GetItems(), repo.collectionsAfterID)
	}

	items, err := h.ListCollectionItems(context.Background(), &pb.ListCollectionItemsRequest{
		UserId: 42, CollectionId: 11, Limit: 10, AfterId: 17, AscendingById: true,
	})
	if err != nil {
		t.Fatalf("list collection items by ID: %v", err)
	}
	if len(items.GetItems()) != 1 || items.GetItems()[0].GetId() != 21 || repo.itemsAfterID != 17 {
		t.Fatalf("items = %+v, after ID = %d", items.GetItems(), repo.itemsAfterID)
	}
}

func TestCollectionHandlersRejectNegativeIDCursor(t *testing.T) {
	h := newCollectionHandler(&collectionRepositoryStub{})

	_, err := h.ListCollections(context.Background(), &pb.ListCollectionsRequest{
		UserId: 42, AfterId: -1, AscendingById: true,
	})
	if status.Code(err) != codes.InvalidArgument {
		t.Fatalf("negative cursor status = %s, want %s", status.Code(err), codes.InvalidArgument)
	}
}

func TestCollectionHandlerMapsLastClippedAtMilliseconds(t *testing.T) {
	lastClippedAt := time.UnixMilli(1_800_000_000_456)
	response := toCollectionPb(&domain.Collection{ID: 7, LastClippedAt: &lastClippedAt})
	if response.GetLastClippedAt() != lastClippedAt.UnixMilli() {
		t.Fatalf("last_clipped_at = %d, want %d", response.GetLastClippedAt(), lastClippedAt.UnixMilli())
	}
}

func TestListPublicCollectionsMapsItems(t *testing.T) {
	repo := &collectionRepositoryStub{publicCollections: []*domain.Collection{{ID: 7, UserID: 42, Name: "Reading", IsPublic: true}}}
	h := newCollectionHandler(repo)

	response, err := h.ListPublicCollections(context.Background(), &pb.ListPublicCollectionsRequest{UserId: 42, Limit: 10})
	if err != nil {
		t.Fatalf("list public collections: %v", err)
	}
	if response.GetTotal() != 1 || len(response.GetItems()) != 1 || response.GetItems()[0].GetId() != 7 {
		t.Fatalf("response = %+v", response)
	}

	entityResponse, err := h.ListPublicCollectionsForEntity(context.Background(), &pb.ListPublicCollectionsForEntityRequest{Entity: &pb.EntityRef{EntityType: "article", EntityId: 9}, Limit: 100})
	if err != nil {
		t.Fatalf("list public collections for entity: %v", err)
	}
	if entityResponse.GetTotal() != 1 || entityResponse.GetItems()[0].GetName() != "Reading" {
		t.Fatalf("entity response = %+v", entityResponse)
	}
}

func TestListPublicCollectionItemsForwardsExclusiveCursors(t *testing.T) {
	repo := &collectionRepositoryStub{items: []*domain.CollectionItem{{ID: 11, CollectionID: 7, Entity: domain.EntityRef{Type: domain.EntityArticle, ID: 88}}}}
	h := newCollectionHandler(repo)

	response, err := h.ListPublicCollectionItems(context.Background(), &pb.ListPublicCollectionItemsRequest{
		CollectionId: 7, Limit: 10, SinceId: 90, UntilId: 30,
	})
	if err != nil {
		t.Fatalf("list public collection items: %v", err)
	}
	if response.GetTotal() != 1 || repo.publicSinceID != 90 || repo.publicUntilID != 30 {
		t.Fatalf("response = %+v, cursors = %d/%d", response, repo.publicSinceID, repo.publicUntilID)
	}

	_, err = h.ListPublicCollectionItems(context.Background(), &pb.ListPublicCollectionItemsRequest{CollectionId: 7, SinceId: -1})
	if status.Code(err) != codes.InvalidArgument {
		t.Fatalf("negative cursor status = %s, want %s", status.Code(err), codes.InvalidArgument)
	}
}

func newCollectionHandler(repo domain.CollectionRepository) *Handler {
	cmd := command.NewService(nil, nil, nil, nil, nil, repo, nil, nil)
	qry := query.NewService(nil, nil, nil, nil, nil, repo)
	return NewHandler(cmd, qry, nil)
}

type collectionRepositoryStub struct {
	collections        []*domain.Collection
	items              []*domain.CollectionItem
	publicCollections  []*domain.Collection
	listItemsErr       error
	lastUserID         int64
	lastCollectionID   int64
	collectionsAfterID int64
	itemsAfterID       int64
	publicSinceID      int64
	publicUntilID      int64
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
	return r.collections, int64(len(r.collections)), nil
}

func (r *collectionRepositoryStub) ListCollectionsAfterID(_ context.Context, _ int64, afterID int64, _ int) ([]*domain.Collection, int64, error) {
	r.collectionsAfterID = afterID
	return r.collections, int64(len(r.collections)), nil
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

func (r *collectionRepositoryStub) ListCollectionItemsAfterID(_ context.Context, _ int64, _ int64, _ domain.EntityType, afterID int64, _ int) ([]*domain.CollectionItem, int64, error) {
	r.itemsAfterID = afterID
	return r.items, int64(len(r.items)), r.listItemsErr
}

func (r *collectionRepositoryStub) GetCollection(context.Context, int64, int64) (*domain.Collection, error) {
	return nil, nil
}

func (r *collectionRepositoryStub) ListPublicCollectionItems(_ context.Context, _ int64, _ int64, _ int, _ int, sinceID, untilID int64) ([]*domain.CollectionItem, int64, error) {
	r.publicSinceID = sinceID
	r.publicUntilID = untilID
	return r.items, int64(len(r.items)), r.listItemsErr
}

func (r *collectionRepositoryStub) ListPublicCollections(context.Context, int64, int64, int, int64, int64) ([]*domain.Collection, error) {
	return r.publicCollections, nil
}

func (r *collectionRepositoryStub) ListPublicCollectionsForEntity(context.Context, domain.EntityRef, int64, int) ([]*domain.Collection, error) {
	return r.publicCollections, nil
}
