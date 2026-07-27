package notification

import (
	"context"
	"errors"
	"time"
)

const (
	SystemNotificationType              = "system"
	SystemNotificationMaxRecipients     = 1000
	SystemNotificationMaxTitleRunes     = 200
	SystemNotificationMaxContentRunes   = 5000
	SystemNotificationMaxIdempotencyKey = 95
)

var ErrInvalidSystemNotification = errors.New("invalid system notification")

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

// SystemNotificationCommand is accepted only through the internal notification RPC.
// It intentionally carries explicit recipients; broadcasts and audience filters are
// not part of the first delivery path.
type SystemNotificationCommand struct {
	RecipientIDs   []int64
	Title          string
	Content        string
	ActorID        int64
	IdempotencyKey string
}

type ArticleRef struct {
	ID       int64
	AuthorID int64
	Title    string
}

type ContentRef struct {
	EntityType string
	ID         int64
	AuthorID   int64
	Title      string
}

type CommentRef struct {
	ID         int64
	EntityType string
	EntityID   int64
	AuthorID   int64
	ParentID   int64
}

type Repository interface {
	EnsureSchema(ctx context.Context) error
	SaveArticle(ctx context.Context, article ArticleRef, publishedAt time.Time) error
	GetArticle(ctx context.Context, id int64) (ArticleRef, error)
	SavePendingArticleNotification(ctx context.Context, eventID, notificationType string, articleID, actorID, sourceID int64, createdAt time.Time) error
	FlushPendingArticleNotifications(ctx context.Context, article ArticleRef) error
	SaveContent(ctx context.Context, content ContentRef, publishedAt time.Time) error
	GetContent(ctx context.Context, entityType string, id int64) (ContentRef, error)
	SavePendingContentNotification(ctx context.Context, eventID, notificationType, entityType string, entityID, actorID, sourceID int64, createdAt time.Time) error
	FlushPendingContentNotifications(ctx context.Context, content ContentRef) error
	SaveComment(ctx context.Context, comment CommentRef, createdAt time.Time) error
	GetComment(ctx context.Context, id int64) (CommentRef, error)
	SavePendingReplyNotification(ctx context.Context, eventID string, parentCommentID, commentID int64, entityType string, entityID, actorID int64, createdAt time.Time) error
	FlushPendingReplyNotifications(ctx context.Context, parent CommentRef) error
	Create(ctx context.Context, item Notification, sourceEventID string, createdAt time.Time) error
	CreateSystemNotifications(ctx context.Context, command SystemNotificationCommand, createdAt time.Time) (int32, error)
	List(ctx context.Context, userID int64, limit, offset int32, unreadOnly bool) ([]Notification, int64, int64, error)
	UnreadCount(ctx context.Context, userID int64) (int64, error)
	MarkRead(ctx context.Context, userID, id int64) error
	MarkAllRead(ctx context.Context, userID int64) error
}
