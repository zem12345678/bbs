package feed

import "context"

type Item struct {
	EntityType    string
	ID            int64
	Slug          string
	Title         string
	Summary       string
	Body          string
	CoverURL      string
	Tags          []string
	AuthorID      int64
	Status        int32
	CreatedAt     int64
	UpdatedAt     int64
	PublishedAt   int64
	LikeCount     int64
	FavoriteCount int64
	CommentCount  int64
	HotScore      float64
}

type Repository interface {
	UpsertArticle(ctx context.Context, item Item) error
	UpsertTopic(ctx context.Context, item Item) error
	RemoveArticle(ctx context.Context, id int64) error
	RemoveTopic(ctx context.Context, id int64) error
	SetLikeCount(ctx context.Context, id int64, count int64) error
	SetFavoriteCount(ctx context.Context, id int64, count int64) error
	IncrementCommentCount(ctx context.Context, id int64, delta int64, activityAt int64) error
	ListLatest(ctx context.Context, limit, offset int) ([]Item, error)
	ListHot(ctx context.Context, limit, offset int) ([]Item, error)
	ListActive(ctx context.Context, limit, offset int) ([]Item, error)
}
