package persistence

import (
	"context"
	"errors"

	domain "user-service/internal/domain/user"

	"gorm.io/gorm"
)

func toFollowingEntity(row followPO) *domain.Following {
	return &domain.Following{
		ID: row.ID, FollowerID: row.FollowerID, FolloweeID: row.FolloweeID,
		WithReplies: row.WithReplies, Notify: domain.FollowNotify(row.Notify), CreatedAt: row.CreatedAt,
	}
}

func (r *Repo) GetFollowing(ctx context.Context, followerID, followeeID int64) (*domain.Following, error) {
	if followerID <= 0 || followeeID <= 0 || followerID == followeeID {
		return nil, domain.ErrInvalidID
	}
	var row followPO
	err := r.db.WithContext(ctx).Where("follower_id = ? AND followee_id = ?", followerID, followeeID).First(&row).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, domain.ErrNotFollowing
	}
	if err != nil {
		return nil, err
	}
	edges := []*domain.Following{toFollowingEntity(row)}
	if err := r.hydrateFollowings(ctx, edges); err != nil {
		return nil, err
	}
	return edges[0], nil
}

func (r *Repo) UpdateFollowing(ctx context.Context, followerID, followeeID int64, patch domain.FollowingPatch) (*domain.Following, error) {
	if followerID <= 0 || followeeID <= 0 || followerID == followeeID {
		return nil, domain.ErrInvalidID
	}
	if err := patch.Validate(); err != nil {
		return nil, err
	}
	updates := followingUpdates(patch)
	if len(updates) > 0 {
		result := r.db.WithContext(ctx).Model(&followPO{}).
			Where("follower_id = ? AND followee_id = ?", followerID, followeeID).
			Updates(updates)
		if result.Error != nil {
			return nil, result.Error
		}
		if result.RowsAffected == 0 {
			return nil, domain.ErrNotFollowing
		}
	}
	return r.GetFollowing(ctx, followerID, followeeID)
}

func (r *Repo) UpdateAllFollowings(ctx context.Context, followerID int64, patch domain.FollowingPatch) error {
	if followerID <= 0 {
		return domain.ErrInvalidID
	}
	if err := patch.Validate(); err != nil {
		return err
	}
	updates := followingUpdates(patch)
	if len(updates) == 0 {
		return nil
	}
	return r.db.WithContext(ctx).Model(&followPO{}).Where("follower_id = ?", followerID).Updates(updates).Error
}

func followingUpdates(patch domain.FollowingPatch) map[string]any {
	updates := make(map[string]any, 2)
	if patch.WithReplies != nil {
		updates["with_replies"] = *patch.WithReplies
	}
	if patch.Notify != nil {
		updates["notify"] = string(*patch.Notify)
	}
	return updates
}

func (r *Repo) ListFollowerEdges(ctx context.Context, query domain.FollowingQuery) ([]*domain.Following, error) {
	return r.listFollowingEdges(ctx, query, "followee_id")
}

func (r *Repo) ListFollowingEdges(ctx context.Context, query domain.FollowingQuery) ([]*domain.Following, error) {
	return r.listFollowingEdges(ctx, query, "follower_id")
}

func (r *Repo) ListNoteNotificationSubscribers(ctx context.Context, query domain.NoteNotificationSubscribersQuery) ([]domain.NoteNotificationSubscriber, error) {
	if err := query.Normalize(); err != nil {
		return nil, err
	}
	var rows []struct {
		EdgeID int64 `gorm:"column:edge_id"`
		UserID int64 `gorm:"column:user_id"`
	}
	db := r.db.WithContext(ctx).Table("user_follows AS follows").
		Select("follows.id AS edge_id, follows.follower_id AS user_id").
		Joins("JOIN users AS followers ON followers.id = follows.follower_id").
		Where("follows.followee_id = ?", query.FolloweeID).
		Where("follows.notify = ?", string(domain.FollowNotifyNormal)).
		Where("follows.id > ?", query.SinceID).
		Where("followers.status = ?", int32(domain.StatusActive)).
		Where("followers.account_state = ?", string(domain.AccountStateActive)).
		Where("NOT EXISTS (SELECT 1 FROM user_blocks AS blocked_by_follower WHERE blocked_by_follower.actor_id = follows.follower_id AND blocked_by_follower.target_id = follows.followee_id)").
		Where("NOT EXISTS (SELECT 1 FROM user_blocks AS blocked_by_followee WHERE blocked_by_followee.actor_id = follows.followee_id AND blocked_by_followee.target_id = follows.follower_id)").
		Where("NOT EXISTS (SELECT 1 FROM user_mutes AS muted_by_follower WHERE muted_by_follower.actor_id = follows.follower_id AND muted_by_follower.target_id = follows.followee_id)").
		Order("follows.id ASC").Limit(query.Limit)
	if err := db.Scan(&rows).Error; err != nil {
		return nil, err
	}
	items := make([]domain.NoteNotificationSubscriber, 0, len(rows))
	for _, row := range rows {
		items = append(items, domain.NoteNotificationSubscriber{EdgeID: row.EdgeID, UserID: row.UserID})
	}
	return items, nil
}

func (r *Repo) listFollowingEdges(ctx context.Context, query domain.FollowingQuery, ownerColumn string) ([]*domain.Following, error) {
	if err := query.Normalize(); err != nil {
		return nil, err
	}
	var owner userPO
	if err := r.db.WithContext(ctx).Where("id = ?", query.UserID).First(&owner).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, domain.ErrNotFound
		}
		return nil, err
	}
	viewerID := query.ViewerID
	visibility := owner.FollowingVisibility
	if ownerColumn == "followee_id" {
		visibility = owner.FollowersVisibility
	}
	if visibility == string(domain.UserVisibilityPrivate) && viewerID != query.UserID {
		return nil, domain.ErrFollowingListForbidden
	}
	if visibility == string(domain.UserVisibilityFollowers) && viewerID != query.UserID {
		if viewerID <= 0 {
			return nil, domain.ErrFollowingListForbidden
		}
		var exists int64
		if err := r.db.WithContext(ctx).Table("user_follows").Where("follower_id = ? AND followee_id = ?", viewerID, query.UserID).Count(&exists).Error; err != nil {
			return nil, err
		}
		if exists == 0 {
			return nil, domain.ErrFollowingListForbidden
		}
	}
	db := r.db.WithContext(ctx).Where(ownerColumn+" = ?", query.UserID)
	if query.SinceID > 0 {
		db = db.Where("id > ?", query.SinceID)
	}
	if query.UntilID > 0 {
		db = db.Where("id < ?", query.UntilID)
	}
	if query.BirthdayMMDD != "" && ownerColumn == "follower_id" {
		db = db.Where("followee_id IN (SELECT id FROM users WHERE substring(birthday, 6, 5) = ?)", query.BirthdayMMDD)
	}
	order := "id DESC"
	if query.SinceID > 0 && query.UntilID == 0 {
		order = "id ASC"
	}
	var rows []followPO
	if err := db.Order(order).Limit(query.Limit).Find(&rows).Error; err != nil {
		return nil, err
	}
	edges := make([]*domain.Following, 0, len(rows))
	for _, row := range rows {
		edges = append(edges, toFollowingEntity(row))
	}
	if err := r.hydrateFollowings(ctx, edges); err != nil {
		return nil, err
	}
	return edges, nil
}

func (r *Repo) hydrateFollowings(ctx context.Context, edges []*domain.Following) error {
	ids := make([]int64, 0, len(edges)*2)
	seen := make(map[int64]struct{}, len(edges)*2)
	for _, edge := range edges {
		for _, id := range []int64{edge.FollowerID, edge.FolloweeID} {
			if _, exists := seen[id]; !exists {
				seen[id] = struct{}{}
				ids = append(ids, id)
			}
		}
	}
	if len(ids) == 0 {
		return nil
	}
	var users []userPO
	if err := r.db.WithContext(ctx).Where("id IN ?", ids).Find(&users).Error; err != nil {
		return err
	}
	byID := make(map[int64]*domain.User, len(users))
	for index := range users {
		byID[users[index].ID] = toEntity(&users[index])
	}
	for _, edge := range edges {
		edge.Follower = byID[edge.FollowerID]
		edge.Followee = byID[edge.FolloweeID]
	}
	return nil
}
