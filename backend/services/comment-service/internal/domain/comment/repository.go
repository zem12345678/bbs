package comment

import "context"

type ListQuery struct {
	EntityType string
	EntityID   int64
	Page       int
	PageSize   int
}

type ReplyListQuery struct {
	RootID   int64
	Page     int
	PageSize int
}

type ConversationQuery struct {
	CommentID int64
	Limit     int
	Offset    int
}

type ModerationListQuery struct {
	EntityType string
	EntityID   int64
	AuthorID   int64
	Status     int32
	Page       int
	PageSize   int
}

type Repository interface {
	Save(ctx context.Context, c *Comment) error
	FindByID(ctx context.Context, id int64) (*Comment, error)
	ListByEntity(ctx context.Context, q ListQuery) ([]*Comment, int64, error)
	ListReplies(ctx context.Context, q ReplyListQuery) ([]*Comment, int64, error)
	ListForModeration(ctx context.Context, q ModerationListQuery) ([]*Comment, int64, error)
	Hide(ctx context.Context, c *Comment) error
	Restore(ctx context.Context, c *Comment) error
	IncrementReplyCount(ctx context.Context, rootID int64, delta int64) error
	RedactAccountComments(ctx context.Context, userID, deletionJobID int64, policyVersion int32) (int64, error)
}
