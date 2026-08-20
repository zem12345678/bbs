package user

import (
	"errors"
	"testing"
)

func TestFollowingPreferencesAndQueryValidation(t *testing.T) {
	edge, err := NewFollowing(1, 2, 3, true)
	if err != nil {
		t.Fatalf("NewFollowing() error = %v", err)
	}
	if !edge.WithReplies || edge.Notify != FollowNotifyNone {
		t.Fatalf("default edge = %+v", edge)
	}
	invalid := FollowNotify("loud")
	if err := (FollowingPatch{Notify: &invalid}).Validate(); !errors.Is(err, ErrFollowNotifyInvalid) {
		t.Fatalf("invalid notify error = %v", err)
	}
	query := FollowingQuery{UserID: 2}
	if err := query.Normalize(); err != nil || query.Limit != DefaultFollowingLimit {
		t.Fatalf("normalized query = %+v, error = %v", query, err)
	}
	query = FollowingQuery{UserID: 2, Limit: MaxFollowingLimit + 1}
	if err := query.Normalize(); !errors.Is(err, ErrFollowingLimitInvalid) {
		t.Fatalf("invalid limit error = %v", err)
	}
}
