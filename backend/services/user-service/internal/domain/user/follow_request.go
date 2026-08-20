package user

import "time"

// FollowRequest is a pending approval for following a private account. Accepting
// one turns it into a row in user_follows; rejecting or cancelling drops it.
type FollowRequest struct {
	ID          int64
	RequesterID int64
	TargetID    int64
	WithReplies bool
	CreatedAt   time.Time

	// Requester is populated when listing requests received by a target, and
	// Target when listing requests the actor has sent. The unused side stays nil.
	Requester *User
	Target    *User
}

// FollowRequestQuery pages the requests received by, or sent from, ActorID.
type FollowRequestQuery struct {
	ActorID  int64
	Page     int
	PageSize int
	SinceID  int64
	UntilID  int64
	Limit    int
}

func (query *FollowRequestQuery) Normalize() error {
	if query.ActorID <= 0 || query.SinceID < 0 || query.UntilID < 0 {
		return ErrInvalidID
	}
	if query.Limit > 0 || query.SinceID > 0 || query.UntilID > 0 {
		if query.Limit == 0 {
			query.Limit = DefaultFollowingLimit
		}
		if query.Limit < 1 || query.Limit > MaxFollowingLimit {
			return ErrFollowingLimitInvalid
		}
	}
	return nil
}

func NewFollowRequest(id, requesterID, targetID int64, withReplies ...bool) (*FollowRequest, error) {
	if id <= 0 || requesterID <= 0 || targetID <= 0 {
		return nil, ErrInvalidID
	}
	if requesterID == targetID {
		return nil, ErrCannotFollowSelf
	}
	includeReplies := false
	if len(withReplies) > 0 {
		includeReplies = withReplies[0]
	}
	return &FollowRequest{
		ID:          id,
		RequesterID: requesterID,
		TargetID:    targetID,
		WithReplies: includeReplies,
		CreatedAt:   time.Now(),
	}, nil
}
