package notification

import (
	"context"
	"time"
)

type Notification struct {
	ID         int64
	UserID     int64
	Type       string
	Title      string
	Content    string
	ActorID    int64
	EntityType string
	EntityID   int64
	SourceID   int64
	ReadAt     *time.Time
	CreatedAt  time.Time
}

type ArticleRef struct {
	ID       int64
	AuthorID int64
	Title    string
}

type Repository interface {
	EnsureSchema(ctx context.Context) error
	SaveArticle(ctx context.Context, article ArticleRef, publishedAt time.Time) error
	GetArticle(ctx context.Context, id int64) (ArticleRef, error)
	SavePendingArticleNotification(ctx context.Context, eventID, notificationType string, articleID, actorID, sourceID int64, createdAt time.Time) error
	FlushPendingArticleNotifications(ctx context.Context, article ArticleRef) error
	Create(ctx context.Context, item Notification, sourceEventID string, createdAt time.Time) error
	List(ctx context.Context, userID int64, limit, offset int32, unreadOnly bool) ([]Notification, int64, int64, error)
	UnreadCount(ctx context.Context, userID int64) (int64, error)
	MarkRead(ctx context.Context, userID, id int64) error
	MarkAllRead(ctx context.Context, userID int64) error
}
