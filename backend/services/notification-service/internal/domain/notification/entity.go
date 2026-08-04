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

const (
	NotificationTypeFollow                        = "follow"
	NotificationTypeComment                       = "comment"
	NotificationTypeReply                         = "reply"
	NotificationTypeLike                          = "like"
	NotificationTypeFavorite                      = "favorite"
	NotificationTypeQAAnswerAccepted              = "qa_answer_accepted"
	NotificationTypeMallRefundApproved            = "mall_refund_approved"
	NotificationTypeMallRefundRejected            = "mall_refund_rejected"
	NotificationTypeMallDigitalEntitlementRevoked = "mall_digital_entitlement_revoked"
	NotificationTypeMallOrderPaid                 = "mall_order_paid"
	NotificationTypeMallOrderShipped              = "mall_order_shipped"
	NotificationTypeMallOrderCompleted            = "mall_order_completed"
	NotificationTypeMallReviewPublished           = "mall_review_published"
	NotificationTypeMallReviewHidden              = "mall_review_hidden"
)

var (
	ErrInvalidSystemNotification      = errors.New("invalid system notification")
	ErrInvalidUserErasure             = errors.New("invalid user erasure")
	ErrInvalidNotificationPreferences = errors.New("invalid notification preferences")
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

type NotificationPreference struct {
	Type    string
	Enabled bool
}

// DefaultNotificationPreferences defines the user-visible in-app notification
// types. Missing database rows intentionally resolve to enabled so existing
// users keep receiving every notification after this feature is deployed.
func DefaultNotificationPreferences() []NotificationPreference {
	types := []string{
		SystemNotificationType,
		NotificationTypeFollow,
		NotificationTypeComment,
		NotificationTypeReply,
		NotificationTypeLike,
		NotificationTypeFavorite,
		NotificationTypeQAAnswerAccepted,
		NotificationTypeMallRefundApproved,
		NotificationTypeMallRefundRejected,
		NotificationTypeMallDigitalEntitlementRevoked,
		NotificationTypeMallOrderPaid,
		NotificationTypeMallOrderShipped,
		NotificationTypeMallOrderCompleted,
		NotificationTypeMallReviewPublished,
		NotificationTypeMallReviewHidden,
	}
	preferences := make([]NotificationPreference, 0, len(types))
	for _, notificationType := range types {
		preferences = append(preferences, NotificationPreference{Type: notificationType, Enabled: true})
	}
	return preferences
}

func NormalizeNotificationPreferences(items []NotificationPreference) ([]NotificationPreference, error) {
	overrides := make(map[string]bool, len(items))
	for _, item := range items {
		if !isSupportedNotificationPreferenceType(item.Type) {
			return nil, ErrInvalidNotificationPreferences
		}
		if _, exists := overrides[item.Type]; exists {
			return nil, ErrInvalidNotificationPreferences
		}
		overrides[item.Type] = item.Enabled
	}
	preferences := DefaultNotificationPreferences()
	for index := range preferences {
		if enabled, exists := overrides[preferences[index].Type]; exists {
			preferences[index].Enabled = enabled
		}
	}
	return preferences, nil
}

func isSupportedNotificationPreferenceType(notificationType string) bool {
	for _, preference := range DefaultNotificationPreferences() {
		if preference.Type == notificationType {
			return true
		}
	}
	return false
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
	EraseUserData(ctx context.Context, userID, deletionJobID int64, policyVersion int32) error
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
	ListPreferences(ctx context.Context, userID int64) ([]NotificationPreference, error)
	ReplacePreferences(ctx context.Context, userID int64, preferences []NotificationPreference) error
	List(ctx context.Context, userID int64, limit, offset int32, unreadOnly bool) ([]Notification, int64, int64, error)
	UnreadCount(ctx context.Context, userID int64) (int64, error)
	MarkRead(ctx context.Context, userID, id int64) error
	MarkAllRead(ctx context.Context, userID int64) error
}
