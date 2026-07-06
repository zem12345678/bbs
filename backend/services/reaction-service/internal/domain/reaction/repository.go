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
