package comment

import (
	"strings"
	"time"
)

type Comment struct {
	ID         int64
	EntityType string
	EntityID   int64
	RootID     int64
	ParentID   int64
	AuthorID   int64
	Content    string
	Status     Status
	ReplyCount int64
	LikeCount  int64
	CreatedAt  time.Time
	UpdatedAt  time.Time
	DeletedAt  *time.Time

	events []DomainEvent
}

type CreateCmd struct {
	EntityType string
	EntityID   int64
	ParentID   int64
	AuthorID   int64
	Content    string
}

func NewRoot(id int64, cmd CreateCmd) (*Comment, error) {
	now := time.Now()
	c := &Comment{
		ID:         id,
		EntityType: cmd.EntityType,
		EntityID:   cmd.EntityID,
		AuthorID:   cmd.AuthorID,
		Content:    strings.TrimSpace(cmd.Content),
		Status:     StatusVisible,
		CreatedAt:  now,
		UpdatedAt:  now,
	}
	if err := c.Validate(); err != nil {
		return nil, err
	}
	c.AddEvent(NewCommentCreatedEvent(c))
	return c, nil
}

func NewReply(id int64, cmd CreateCmd, rootID, parentID int64) (*Comment, error) {
	now := time.Now()
	c := &Comment{
		ID:         id,
		EntityType: cmd.EntityType,
		EntityID:   cmd.EntityID,
		RootID:     rootID,
		ParentID:   parentID,
		AuthorID:   cmd.AuthorID,
		Content:    strings.TrimSpace(cmd.Content),
		Status:     StatusVisible,
		CreatedAt:  now,
		UpdatedAt:  now,
	}
	if err := c.Validate(); err != nil {
		return nil, err
	}
	if c.RootID <= 0 || c.ParentID <= 0 {
		return nil, ErrInvalidParent
	}
	c.AddEvent(NewCommentCreatedEvent(c))
	return c, nil
}

func (c *Comment) Validate() error {
	if c == nil || c.ID <= 0 {
		return ErrInvalidID
	}
	if _, err := ParseEntityType(c.EntityType); err != nil {
		return err
	}
	if c.EntityID <= 0 {
		return ErrInvalidEntityID
	}
	if c.AuthorID <= 0 {
		return ErrInvalidAuthorID
	}
	if strings.TrimSpace(c.Content) == "" {
		return ErrContentRequired
	}
	if len([]rune(c.Content)) > MaxContentRunes {
		return ErrContentTooLong
	}
	if !c.Status.IsValid() {
		return ErrInvalidStatus
	}
	if c.RootID < 0 || c.ParentID < 0 {
		return ErrInvalidParent
	}
	return nil
}

func (c *Comment) IsRoot() bool {
	return c.RootID == 0 && c.ParentID == 0
}

func (c *Comment) Hide(actorID int64, moderator bool) error {
	if c.Status == StatusHidden {
		return ErrAlreadyHidden
	}
	if actorID <= 0 || (!moderator && actorID != c.AuthorID) {
		return ErrPermissionDenied
	}
	if !c.Status.CanTransitionTo(StatusHidden) {
		return ErrInvalidStatusChange
	}
	now := time.Now()
	c.Status = StatusHidden
	c.UpdatedAt = now
	c.DeletedAt = &now
	c.AddEvent(NewCommentDeletedEvent(c, actorID))
	return nil
}

func (c *Comment) AddEvent(event DomainEvent) {
	c.events = append(c.events, event)
}

func (c *Comment) Events() []DomainEvent {
	events := c.events
	c.events = nil
	return events
}
