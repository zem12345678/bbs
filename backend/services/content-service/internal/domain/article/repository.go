package article

import (
	"context"
	"time"
)

type TagStats struct {
	Name  string
	Count int64
}

type Repository interface {
	Create(ctx context.Context, a *Article) error
	Update(ctx context.Context, a *Article) error
	FindBySlug(ctx context.Context, slug string) (*Article, error)
	FindByID(ctx context.Context, id int64) (*Article, error)
	List(ctx context.Context, status Status, tag string, authorID int64, sort string, limit, offset int) ([]*Article, int64, error)
	ListTags(ctx context.Context, status Status, query string, limit int) ([]TagStats, error)
	UpdateStatus(ctx context.Context, id int64, status Status, publishedAt *time.Time) error
	FeedByTime(ctx context.Context, limit, offset int) ([]*Article, error)
	FindByIDs(ctx context.Context, ids []int64) (map[int64]*Article, error)
	IncrementViewCount(ctx context.Context, id int64) (int64, error)
}

// KeysetRepository is the stable, ID-ordered read path used by exports.
// It remains optional so offset-based callers and their fakes are unchanged.
type KeysetRepository interface {
	ListAfterID(ctx context.Context, status Status, authorID, afterID int64, limit int) ([]*Article, int64, error)
}

type Cache interface {
	Get(ctx context.Context, slug string) (*Article, bool)
	Set(ctx context.Context, a *Article)
	Del(ctx context.Context, slug string)
}
