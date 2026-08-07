package user

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

type UserEvent struct {
	baseEvent
	UserID    int64  `json:"user_id"`
	Username  string `json:"username"`
	Email     string `json:"email"`
	Nickname  string `json:"nickname"`
	Status    int32  `json:"status"`
	CreatedAt int64  `json:"created_at"`
	UpdatedAt int64  `json:"updated_at"`
	event     string
}

func NewCreatedEvent(u *User) UserEvent {
	return newUserEvent("user.created", u)
}

func NewUpdatedEvent(u *User) UserEvent {
	return newUserEvent("user.updated", u)
}

func NewDeletedEvent(u *User) UserEvent {
	return newUserEvent("user.deleted", u)
}

func newUserEvent(name string, u *User) UserEvent {
	return UserEvent{
		baseEvent: newBaseEvent(),
		UserID:    u.ID,
		Username:  u.Username,
		Email:     u.Email,
		Nickname:  u.Nickname,
		Status:    int32(u.Status),
		CreatedAt: u.CreatedAt.UnixMilli(),
		UpdatedAt: u.UpdatedAt.UnixMilli(),
		event:     name,
	}
}

func (e UserEvent) EventName() string  { return e.event }
func (e UserEvent) AggregateID() int64 { return e.UserID }

type FollowEvent struct {
	baseEvent
	FollowerID int64 `json:"follower_id"`
	FolloweeID int64 `json:"followee_id"`
	event      string
}

func NewFollowedEvent(followerID, followeeID int64) FollowEvent {
	return FollowEvent{baseEvent: newBaseEvent(), FollowerID: followerID, FolloweeID: followeeID, event: "user.followed"}
}

func NewUnfollowedEvent(followerID, followeeID int64) FollowEvent {
	return FollowEvent{baseEvent: newBaseEvent(), FollowerID: followerID, FolloweeID: followeeID, event: "user.unfollowed"}
}

// NewFollowRequestedEvent marks a pending approval against a private account.
// FolloweeID is the account that must approve, matching the followed event shape
// so downstream consumers can reuse one payload decoder.
func NewFollowRequestedEvent(requesterID, targetID int64) FollowEvent {
	return FollowEvent{baseEvent: newBaseEvent(), FollowerID: requesterID, FolloweeID: targetID, event: "user.follow_requested"}
}

func NewFollowRequestAcceptedEvent(requesterID, targetID int64) FollowEvent {
	return FollowEvent{baseEvent: newBaseEvent(), FollowerID: requesterID, FolloweeID: targetID, event: "user.follow_request_accepted"}
}
func (e FollowEvent) EventName() string  { return e.event }
func (e FollowEvent) AggregateID() int64 { return e.FollowerID }
