package query

import (
	"context"
	"errors"

	domain "comment-service/internal/domain/comment"
)

const (
	maxConversationLimit  = 100
	maxConversationOffset = 10000
)

func (s *Service) Conversation(ctx context.Context, q domain.ConversationQuery) ([]*domain.Comment, error) {
	if q.CommentID <= 0 {
		return nil, domain.ErrInvalidID
	}
	if q.Limit < 1 || q.Limit > maxConversationLimit {
		return nil, domain.ErrConversationLimitInvalid
	}
	if q.Offset < 0 || q.Offset > maxConversationOffset {
		return nil, domain.ErrConversationOffsetInvalid
	}

	target, err := s.repo.FindByID(ctx, q.CommentID)
	if err != nil {
		return nil, err
	}
	if target == nil {
		return nil, domain.ErrNotFound
	}
	if target.Status != domain.StatusVisible {
		return []*domain.Comment{}, nil
	}
	if target.ParentID == 0 {
		if !target.IsRoot() {
			return nil, domain.ErrInvalidParent
		}
		return []*domain.Comment{}, nil
	}
	if target.RootID <= 0 {
		return nil, domain.ErrInvalidParent
	}

	items := make([]*domain.Comment, 0, q.Limit)
	seen := map[int64]struct{}{target.ID: {}}
	parentID := target.ParentID
	skipped := 0
	for parentID != 0 && len(items) < q.Limit {
		if _, exists := seen[parentID]; exists {
			return nil, domain.ErrInvalidParent
		}
		seen[parentID] = struct{}{}

		parent, findErr := s.repo.FindByID(ctx, parentID)
		if findErr != nil {
			if errors.Is(findErr, domain.ErrNotFound) {
				return nil, domain.ErrInvalidParent
			}
			return nil, findErr
		}
		if parent == nil {
			return nil, domain.ErrInvalidParent
		}
		if parent.Status != domain.StatusVisible {
			break
		}
		if parent.EntityType != target.EntityType || parent.EntityID != target.EntityID {
			return nil, domain.ErrInvalidParent
		}
		if parent.IsRoot() {
			if parent.ID != target.RootID {
				return nil, domain.ErrInvalidParent
			}
		} else if parent.RootID != target.RootID || parent.ParentID <= 0 {
			return nil, domain.ErrInvalidParent
		}

		if skipped < q.Offset {
			skipped++
		} else {
			items = append(items, parent)
		}
		parentID = parent.ParentID
	}

	return items, nil
}
