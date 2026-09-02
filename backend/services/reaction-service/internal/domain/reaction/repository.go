package reaction

import (
	"context"
	"time"
)

type Store interface {
	Like(ctx context.Context, ref EntityRef, userID int64) (count int64, changed bool, err error)
	Unlike(ctx context.Context, ref EntityRef, userID int64) (count int64, changed bool, err error)
	Favorite(ctx context.Context, ref EntityRef, userID int64) (count int64, changed bool, err error)
	Unfavorite(ctx context.Context, ref EntityRef, userID int64) (count int64, changed bool, err error)
	LikeCount(ctx context.Context, ref EntityRef) (int64, error)
	FavoriteCount(ctx context.Context, ref EntityRef) (int64, error)
	HotIDs(ctx context.Context, entityType EntityType, limit int) ([]int64, error)
}

type Favorite struct {
	ID        int64
	UserID    int64
	Entity    EntityRef
	CreatedAt time.Time
	UpdatedAt time.Time
}

type Reaction struct {
	ID        int64
	UserID    int64
	Entity    EntityRef
	Reaction  string
	CreatedAt time.Time
	UpdatedAt time.Time
}

type Like struct {
	ID        int64
	UserID    int64
	Entity    EntityRef
	CreatedAt time.Time
	UpdatedAt time.Time
}

type LikeRepository interface {
	Like(ctx context.Context, ref EntityRef, userID int64) (count int64, changed bool, err error)
	Unlike(ctx context.Context, ref EntityRef, userID int64) (count int64, changed bool, err error)
	Count(ctx context.Context, ref EntityRef) (int64, error)
	HotIDs(ctx context.Context, entityType EntityType, limit int) ([]int64, error)
	ListLikes(ctx context.Context, userID int64, entityType EntityType, limit, offset int) ([]*Like, int64, error)
}

type FavoriteRepository interface {
	Favorite(ctx context.Context, ref EntityRef, userID int64) (count int64, changed bool, err error)
	Unfavorite(ctx context.Context, ref EntityRef, userID int64) (count int64, changed bool, err error)
	Count(ctx context.Context, ref EntityRef) (int64, error)
	ListFavorites(ctx context.Context, userID int64, entityType EntityType, limit, offset int) ([]*Favorite, int64, error)
}

type ReactionRepository interface {
	CreateReaction(ctx context.Context, ref EntityRef, userID int64, reaction string) (*Reaction, bool, error)
	DeleteReaction(ctx context.Context, ref EntityRef, userID int64) (bool, error)
	ListReactions(ctx context.Context, userID int64, entityType EntityType, limit, offset int, sinceID, untilID, sinceDate, untilDate int64) ([]*Reaction, int64, error)
}

type PinRepository interface {
	Pin(ctx context.Context, ref EntityRef, userID int64) (count int64, changed bool, err error)
	Unpin(ctx context.Context, ref EntityRef, userID int64) (count int64, changed bool, err error)
	ListPins(ctx context.Context, userID int64, limit, offset int) ([]*Pin, int64, error)
}

// FavoriteKeysetRepository provides stable, exclusive relation-ID traversal
// without changing the existing offset list behavior.
type FavoriteKeysetRepository interface {
	ListFavoritesAfterID(ctx context.Context, userID int64, entityType EntityType, afterID int64, limit int) ([]*Favorite, int64, error)
}

// CollectionRepository is intentionally separate from FavoriteRepository:
// collection membership is user-curated metadata and must not alter the
// global favorite count or Redis favorite cache.
type CollectionRepository interface {
	CreateCollection(ctx context.Context, collection *Collection) error
	UpdateCollection(ctx context.Context, userID, collectionID int64, name, description string, isPublic bool) (*Collection, error)
	DeleteCollection(ctx context.Context, userID, collectionID int64) error
	ListCollections(ctx context.Context, userID int64, limit, offset int) ([]*Collection, int64, error)
	AddCollectionItem(ctx context.Context, userID, collectionID int64, entity EntityRef) (changed bool, err error)
	RemoveCollectionItem(ctx context.Context, userID, collectionID int64, entity EntityRef) (changed bool, err error)
	ListCollectionItems(ctx context.Context, userID, collectionID int64, entityType EntityType, limit, offset int) ([]*CollectionItem, int64, error)
}

// CollectionKeysetRepository provides stable, exclusive ID traversal for
// long-running consumers without changing the existing offset list behavior.
type CollectionKeysetRepository interface {
	ListCollectionsAfterID(ctx context.Context, userID, afterID int64, limit int) ([]*Collection, int64, error)
	ListCollectionItemsAfterID(ctx context.Context, userID, collectionID int64, entityType EntityType, afterID int64, limit int) ([]*CollectionItem, int64, error)
}

type PublicCollectionRepository interface {
	GetCollection(ctx context.Context, collectionID, viewerUserID int64) (*Collection, error)
	ListPublicCollectionItems(ctx context.Context, collectionID, viewerUserID int64, limit, offset int, sinceID, untilID int64) ([]*CollectionItem, int64, error)
	ListPublicCollections(ctx context.Context, userID, viewerUserID int64, limit int, sinceID, untilID int64) ([]*Collection, error)
	ListPublicCollectionsForEntity(ctx context.Context, entity EntityRef, viewerUserID int64, limit int) ([]*Collection, error)
}
