package query

import (
	"context"
	"errors"

	domain "user-service/internal/domain/user"
)

type FollowRequestsResult struct {
	Items []*domain.FollowRequest
	Total int64
}

// IsFollowRequestPending reports whether actorID already has a request waiting
// on targetID. Missing requests are the normal false case.
func (s *Service) IsFollowRequestPending(ctx context.Context, actorID, targetID int64) (bool, error) {
	if actorID <= 0 || targetID <= 0 {
		return false, domain.ErrInvalidID
	}
	if actorID == targetID {
		return false, nil
	}
	repo, ok := s.repo.(domain.FollowRequestRepository)
	if !ok {
		return false, domain.ErrFollowRequestRepositoryUnavailable
	}
	if _, err := repo.GetFollowRequest(ctx, actorID, targetID); err != nil {
		if errors.Is(err, domain.ErrFollowRequestNotFound) {
			return false, nil
		}
		return false, err
	}
	return true, nil
}

// ListReceivedFollowRequests pages the approvals waiting on the actor.
func (s *Service) ListReceivedFollowRequests(ctx context.Context, q domain.FollowRequestQuery) (FollowRequestsResult, error) {
	repo, err := s.followRequestRepository(q.ActorID)
	if err != nil {
		return FollowRequestsResult{}, err
	}
	if _, err := s.repo.FindByID(ctx, q.ActorID); err != nil {
		return FollowRequestsResult{}, err
	}
	items, total, err := repo.ListReceivedFollowRequests(ctx, q)
	if err != nil {
		return FollowRequestsResult{}, err
	}
	return FollowRequestsResult{Items: items, Total: total}, nil
}

// ListSentFollowRequests pages the approvals the actor is waiting on.
func (s *Service) ListSentFollowRequests(ctx context.Context, q domain.FollowRequestQuery) (FollowRequestsResult, error) {
	repo, err := s.followRequestRepository(q.ActorID)
	if err != nil {
		return FollowRequestsResult{}, err
	}
	if _, err := s.repo.FindByID(ctx, q.ActorID); err != nil {
		return FollowRequestsResult{}, err
	}
	items, total, err := repo.ListSentFollowRequests(ctx, q)
	if err != nil {
		return FollowRequestsResult{}, err
	}
	return FollowRequestsResult{Items: items, Total: total}, nil
}

func (s *Service) followRequestRepository(actorID int64) (domain.FollowRequestRepository, error) {
	if actorID <= 0 {
		return nil, domain.ErrInvalidID
	}
	repo, ok := s.repo.(domain.FollowRequestRepository)
	if !ok {
		return nil, domain.ErrFollowRequestRepositoryUnavailable
	}
	return repo, nil
}
