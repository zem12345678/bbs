package user

import "time"

type FollowNotify string

const (
	FollowNotifyNormal FollowNotify = "normal"
	FollowNotifyNone   FollowNotify = "none"

	DefaultFollowingLimit = 10
	MaxFollowingLimit     = 100
)

func ValidFollowNotify(notify FollowNotify) bool {
	return notify == FollowNotifyNormal || notify == FollowNotifyNone
}

type Following struct {
	ID          int64
	FollowerID  int64
	FolloweeID  int64
	WithReplies bool
	Notify      FollowNotify
	CreatedAt   time.Time
	Follower    *User
	Followee    *User
}

func NewFollowing(id, followerID, followeeID int64, withReplies bool) (*Following, error) {
	if id <= 0 || followerID <= 0 || followeeID <= 0 {
		return nil, ErrInvalidID
	}
	if followerID == followeeID {
		return nil, ErrCannotFollowSelf
	}
	return &Following{
		ID: id, FollowerID: followerID, FolloweeID: followeeID,
		WithReplies: withReplies, Notify: FollowNotifyNone, CreatedAt: time.Now(),
	}, nil
}

type FollowingPatch struct {
	WithReplies *bool
	Notify      *FollowNotify
}

func (patch FollowingPatch) Validate() error {
	if patch.Notify != nil && !ValidFollowNotify(*patch.Notify) {
		return ErrFollowNotifyInvalid
	}
	return nil
}

type FollowingQuery struct {
	UserID       int64
	ViewerID     int64
	SinceID      int64
	UntilID      int64
	Limit        int
	BirthdayMMDD string
}

type NoteNotificationSubscriber struct {
	EdgeID int64
	UserID int64
}

type NoteNotificationSubscribersQuery struct {
	FolloweeID int64
	SinceID    int64
	Limit      int
}

func (query *NoteNotificationSubscribersQuery) Normalize() error {
	if query.FolloweeID <= 0 || query.SinceID < 0 {
		return ErrInvalidID
	}
	if query.Limit == 0 {
		query.Limit = MaxFollowingLimit
	}
	if query.Limit < 1 || query.Limit > MaxFollowingLimit {
		return ErrFollowingLimitInvalid
	}
	return nil
}

func (query *FollowingQuery) Normalize() error {
	if query.UserID <= 0 || query.ViewerID < 0 || query.SinceID < 0 || query.UntilID < 0 {
		return ErrInvalidID
	}
	if query.Limit == 0 {
		query.Limit = DefaultFollowingLimit
	}
	if query.Limit < 1 || query.Limit > MaxFollowingLimit {
		return ErrFollowingLimitInvalid
	}
	if query.BirthdayMMDD != "" {
		if len(query.BirthdayMMDD) != 5 || query.BirthdayMMDD[2] != '-' {
			return ErrInvalidBirthday
		}
		if _, err := time.Parse("2006-01-02", "2000-"+query.BirthdayMMDD); err != nil {
			return ErrInvalidBirthday
		}
	}
	return nil
}
