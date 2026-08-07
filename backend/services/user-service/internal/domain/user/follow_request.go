package user

import "time"

// FollowRequest is a pending approval for following a private account. Accepting
// one turns it into a row in user_follows; rejecting or cancelling drops it.
type FollowRequest struct {
	ID          int64
	RequesterID int64
	TargetID    int64
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
}

func NewFollowRequest(id, requesterID, targetID int64) (*FollowRequest, error) {
	if id <= 0 || requesterID <= 0 || targetID <= 0 {
		return nil, ErrInvalidID
	}
	if requesterID == targetID {
		return nil, ErrCannotFollowSelf
	}
	return &FollowRequest{
		ID:          id,
		RequesterID: requesterID,
		TargetID:    targetID,
		CreatedAt:   time.Now(),
	}, nil
}
