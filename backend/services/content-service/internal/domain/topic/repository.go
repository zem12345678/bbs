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
	ListTopics(ctx context.Context, status Status, typ Type, tag string, authorID int64, categoryID int64, sort string, limit, offset int) ([]*Topic, error)
	UpdateTopicStatus(ctx context.Context, id int64, status Status, publishedAt *time.Time) error
	AcceptTopicComment(ctx context.Context, topicID, commentID, commentAuthorID int64, updatedAt time.Time) (*Topic, bool, error)
	IncrementTopicViewCount(ctx context.Context, id int64) (int64, error)
}
