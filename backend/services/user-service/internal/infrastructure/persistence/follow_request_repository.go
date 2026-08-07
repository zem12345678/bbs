package persistence

import (
	"context"
	"errors"
	"time"

	domain "user-service/internal/domain/user"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type followRequestPO struct {
	ID          int64     `gorm:"primaryKey"`
	RequesterID int64     `gorm:"not null;index"`
	TargetID    int64     `gorm:"not null;index"`
	CreatedAt   time.Time `gorm:"index"`
}

// FollowOrRequest resolves the current privacy setting and relationship state
// under the pair lock, preventing a concurrent block or settings change from
// selecting the wrong durable outcome.
func (r *Repo) FollowOrRequest(ctx context.Context, requestID, requesterID, targetID int64) (bool, bool, error) {
	if requesterID <= 0 || targetID <= 0 || requesterID == targetID {
		return false, false, domain.ErrInvalidID
	}
	now := time.Now()
	var pending, created bool
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := lockActiveUserPair(tx, requesterID, targetID); err != nil {
			return err
		}
		if err := ensureUserPairNotBlocked(tx, requesterID, targetID); err != nil {
			return err
		}
		var target userPO
		if err := tx.First(&target, targetID).Error; err != nil {
			return err
		}
		var live int64
		if err := tx.Model(&followPO{}).Where("follower_id = ? AND followee_id = ?", requesterID, targetID).Count(&live).Error; err != nil {
			return err
		}
		if live > 0 {
			return domain.ErrAlreadyFollowing
		}
		if !target.FollowApprovalRequired {
			if err := tx.Where("requester_id = ? AND target_id = ?", requesterID, targetID).Delete(&followRequestPO{}).Error; err != nil {
				return err
			}
			res := tx.Clauses(clause.OnConflict{DoNothing: true}).Create(&followPO{FollowerID: requesterID, FolloweeID: targetID, CreatedAt: now})
			if res.Error != nil {
				return res.Error
			}
			if res.RowsAffected == 0 {
				return domain.ErrAlreadyFollowing
			}
			if err := tx.Model(&userPO{}).Where("id = ?", targetID).UpdateColumn("follower_count", gorm.Expr("follower_count + 1")).Error; err != nil {
				return err
			}
			if err := tx.Model(&userPO{}).Where("id = ?", requesterID).UpdateColumn("following_count", gorm.Expr("following_count + 1")).Error; err != nil {
				return err
			}
			created = true
			return nil
		}
		pending = true
		res := tx.Clauses(clause.OnConflict{DoNothing: true}).Create(&followRequestPO{ID: requestID, RequesterID: requesterID, TargetID: targetID, CreatedAt: now})
		if res.Error != nil {
			return mapWriteError(res.Error)
		}
		created = res.RowsAffected > 0
		return nil
	})
	return pending, created, err
}

func (followRequestPO) TableName() string {
	return "user_follow_requests"
}

func toFollowRequestEntity(p followRequestPO) *domain.FollowRequest {
	return &domain.FollowRequest{
		ID:          p.ID,
		RequesterID: p.RequesterID,
		TargetID:    p.TargetID,
		CreatedAt:   p.CreatedAt,
	}
}

func (r *Repo) CreateFollowRequest(ctx context.Context, req *domain.FollowRequest) error {
	if req == nil {
		return domain.ErrInvalidID
	}
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := lockActiveUserPair(tx, req.RequesterID, req.TargetID); err != nil {
			return err
		}
		if err := ensureUserPairNotBlocked(tx, req.RequesterID, req.TargetID); err != nil {
			return err
		}
		res := tx.Clauses(clause.OnConflict{DoNothing: true}).Create(&followRequestPO{
			ID:          req.ID,
			RequesterID: req.RequesterID,
			TargetID:    req.TargetID,
			CreatedAt:   req.CreatedAt,
		})
		if res.Error != nil {
			return mapWriteError(res.Error)
		}
		if res.RowsAffected == 0 {
			return domain.ErrFollowRequestAlreadyExists
		}
		return nil
	})
}

func (r *Repo) DeleteFollowRequest(ctx context.Context, requesterID, targetID int64) error {
	res := r.db.WithContext(ctx).
		Where("requester_id = ? AND target_id = ?", requesterID, targetID).
		Delete(&followRequestPO{})
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		return domain.ErrFollowRequestNotFound
	}
	return nil
}

// AcceptFollowRequest consumes the pending row and materialises the follow in a
// single transaction, mirroring the counter bookkeeping Repo.Follow performs.
func (r *Repo) AcceptFollowRequest(ctx context.Context, requesterID, targetID int64) error {
	now := time.Now()
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := lockActiveUserPair(tx, requesterID, targetID); err != nil {
			return err
		}
		if err := ensureUserPairNotBlocked(tx, requesterID, targetID); err != nil {
			return err
		}
		res := tx.Where("requester_id = ? AND target_id = ?", requesterID, targetID).Delete(&followRequestPO{})
		if res.Error != nil {
			return res.Error
		}
		if res.RowsAffected == 0 {
			return domain.ErrFollowRequestNotFound
		}
		created := tx.Clauses(clause.OnConflict{DoNothing: true}).Create(&followPO{
			FollowerID: requesterID,
			FolloweeID: targetID,
			CreatedAt:  now,
		})
		if created.Error != nil {
			return created.Error
		}
		if created.RowsAffected == 0 {
			// The follow already exists, so the request was redundant. Dropping it
			// is the desired end state and the counters already reflect reality.
			return nil
		}
		if err := tx.Model(&userPO{}).Where("id = ?", targetID).UpdateColumn("follower_count", gorm.Expr("follower_count + 1")).Error; err != nil {
			return err
		}
		return tx.Model(&userPO{}).Where("id = ?", requesterID).UpdateColumn("following_count", gorm.Expr("following_count + 1")).Error
	})
}

func (r *Repo) GetFollowRequest(ctx context.Context, requesterID, targetID int64) (*domain.FollowRequest, error) {
	var row followRequestPO
	err := r.db.WithContext(ctx).
		Where("requester_id = ? AND target_id = ?", requesterID, targetID).
		First(&row).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, domain.ErrFollowRequestNotFound
	}
	if err != nil {
		return nil, err
	}
	return toFollowRequestEntity(row), nil
}

func (r *Repo) ListReceivedFollowRequests(ctx context.Context, q domain.FollowRequestQuery) ([]*domain.FollowRequest, int64, error) {
	return r.listFollowRequests(ctx, q, "target_id", "requester_id")
}

func (r *Repo) ListSentFollowRequests(ctx context.Context, q domain.FollowRequestQuery) ([]*domain.FollowRequest, int64, error) {
	return r.listFollowRequests(ctx, q, "requester_id", "target_id")
}

// listFollowRequests pages one side of the pending requests and hydrates the
// counterpart user so callers can render a list without an N+1 lookup.
func (r *Repo) listFollowRequests(ctx context.Context, q domain.FollowRequestQuery, ownerColumn, otherColumn string) ([]*domain.FollowRequest, int64, error) {
	normalizeList(&q.Page, &q.PageSize)
	var total int64
	if err := r.db.WithContext(ctx).Model(&followRequestPO{}).Where(ownerColumn+" = ?", q.ActorID).Count(&total).Error; err != nil {
		return nil, 0, err
	}
	var rows []followRequestPO
	err := r.db.WithContext(ctx).
		Where(ownerColumn+" = ?", q.ActorID).
		Order("created_at DESC, id DESC").
		Limit(q.PageSize).
		Offset((q.Page - 1) * q.PageSize).
		Find(&rows).Error
	if err != nil {
		return nil, 0, err
	}
	out := make([]*domain.FollowRequest, 0, len(rows))
	ids := make([]int64, 0, len(rows))
	for _, row := range rows {
		entity := toFollowRequestEntity(row)
		out = append(out, entity)
		if otherColumn == "requester_id" {
			ids = append(ids, row.RequesterID)
		} else {
			ids = append(ids, row.TargetID)
		}
	}
	if len(ids) == 0 {
		return out, total, nil
	}
	var users []userPO
	if err := r.db.WithContext(ctx).Where("id IN ?", ids).Find(&users).Error; err != nil {
		return nil, 0, err
	}
	byID := make(map[int64]*domain.User, len(users))
	for i := range users {
		byID[users[i].ID] = toEntity(&users[i])
	}
	for _, entity := range out {
		if otherColumn == "requester_id" {
			entity.Requester = byID[entity.RequesterID]
		} else {
			entity.Target = byID[entity.TargetID]
		}
	}
	return out, total, nil
}

func (r *Repo) SetFollowApprovalRequired(ctx context.Context, userID int64, required bool) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var row userPO
		err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).First(&row, userID).Error
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return domain.ErrNotFound
		}
		if err != nil {
			return err
		}
		if err := toEntity(&row).EnsureActive(); err != nil {
			return err
		}
		return tx.Model(&userPO{}).
			Where("id = ?", userID).
			Updates(map[string]any{"follow_approval_required": required, "updated_at": time.Now()}).Error
	})
}
