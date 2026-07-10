package topic

import (
	"context"
	"time"
)

type Repository interface {
	CreateTopic(ctx context.Context, t *Topic) error
	UpdateTopic(ctx context.Context, t *Topic) error
	FindTopicBySlug(ctx context.Context, slug string) (*Topic, error)
	FindTopicByID(ctx context.Context, id int64) (*Topic, error)
	ListTopics(ctx context.Context, status Status, typ Type, tag string, authorID int64, categoryID int64, limit, offset int) ([]*Topic, error)
	UpdateTopicStatus(ctx context.Context, id int64, status Status, publishedAt *time.Time) error
	IncrementTopicViewCount(ctx context.Context, id int64) (int64, error)
}
