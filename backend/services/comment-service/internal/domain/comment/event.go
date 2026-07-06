package comment

import "time"

type DomainEvent interface {
	EventName() string
	OccurredAt() time.Time
	AggregateID() int64
}

type baseEvent struct {
	occurredAt time.Time
}

func newBaseEvent() baseEvent {
	return baseEvent{occurredAt: time.Now()}
}

func (e baseEvent) OccurredAt() time.Time { return e.occurredAt }

type CommentCreatedEvent struct {
	baseEvent
	CommentID  int64  `json:"comment_id"`
	EntityType string `json:"entity_type"`
	EntityID   int64  `json:"entity_id"`
	RootID     int64  `json:"root_id"`
	ParentID   int64  `json:"parent_id"`
	AuthorID   int64  `json:"author_id"`
}

func NewCommentCreatedEvent(c *Comment) CommentCreatedEvent {
	return CommentCreatedEvent{
		baseEvent:  newBaseEvent(),
		CommentID:  c.ID,
		EntityType: c.EntityType,
		EntityID:   c.EntityID,
		RootID:     c.RootID,
		ParentID:   c.ParentID,
		AuthorID:   c.AuthorID,
	}
}

func (e CommentCreatedEvent) EventName() string  { return "comment.created" }
func (e CommentCreatedEvent) AggregateID() int64 { return e.CommentID }

type CommentDeletedEvent struct {
	baseEvent
	CommentID  int64  `json:"comment_id"`
	EntityType string `json:"entity_type"`
	EntityID   int64  `json:"entity_id"`
	RootID     int64  `json:"root_id"`
	ParentID   int64  `json:"parent_id"`
	AuthorID   int64  `json:"author_id"`
	ActorID    int64  `json:"actor_id"`
}

func NewCommentDeletedEvent(c *Comment, actorID int64) CommentDeletedEvent {
	return CommentDeletedEvent{
		baseEvent:  newBaseEvent(),
		CommentID:  c.ID,
		EntityType: c.EntityType,
		EntityID:   c.EntityID,
		RootID:     c.RootID,
		ParentID:   c.ParentID,
		AuthorID:   c.AuthorID,
		ActorID:    actorID,
	}
}

func (e CommentDeletedEvent) EventName() string  { return "comment.deleted" }
func (e CommentDeletedEvent) AggregateID() int64 { return e.CommentID }
