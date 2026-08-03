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
	ListTopics(ctx context.Context, status Status, typ Type, tag string, authorID int64, categoryID int64, sort string, limit, offset int) ([]*Topic, int64, error)
	UpdateTopicStatus(ctx context.Context, id int64, status Status, publishedAt *time.Time) error
	AcceptTopicComment(ctx context.Context, topicID, commentID, commentAuthorID int64, updatedAt time.Time) (*Topic, bool, error)
	UnacceptTopicComment(ctx context.Context, topicID, commentID int64, updatedAt time.Time) (*Topic, bool, error)
	IncrementTopicViewCount(ctx context.Context, id int64) (int64, error)
}

type PollRepository interface {
	UpdateTopicWithPoll(ctx context.Context, t *Topic, poll *PollInput) error
	FindTopicPoll(ctx context.Context, topicID, userID int64) (*Poll, error)
	VoteTopicPoll(ctx context.Context, topicID, userID int64, choices []int32, now time.Time) (*Poll, error)
}

type QAAcceptanceOutboxEvent struct {
	EventID    string
	TopicID    int64
	MessageKey string
	Payload    []byte
	Attempt    int
}

type QAAcceptanceOutboxRepository interface {
	AcceptTopicCommentWithOutbox(ctx context.Context, topicID, commentID, commentAuthorID int64, updatedAt time.Time, event QAAcceptanceOutboxEvent) (*Topic, bool, error)
	EnsureQAAcceptanceOutboxEvent(ctx context.Context, event QAAcceptanceOutboxEvent) error
	ClaimPendingQAAcceptanceOutboxEvents(ctx context.Context, owner string, limit int, leaseDuration time.Duration) ([]QAAcceptanceOutboxEvent, error)
	MarkQAAcceptanceOutboxEventPublished(ctx context.Context, eventID, owner string) error
	MarkQAAcceptanceOutboxEventFailed(ctx context.Context, eventID, owner, message string, nextAttemptAt time.Time) error
}
