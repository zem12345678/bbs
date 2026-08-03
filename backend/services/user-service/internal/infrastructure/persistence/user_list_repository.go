package persistence

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	domain "user-service/internal/domain/user"

	"github.com/jackc/pgx/v5/pgconn"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type userListPO struct {
	ID        int64     `gorm:"primaryKey"`
	OwnerID   int64     `gorm:"not null;index"`
	Name      string    `gorm:"size:100;not null"`
	IsPublic  bool      `gorm:"not null"`
	CreatedAt time.Time `gorm:"not null"`
	UpdatedAt time.Time `gorm:"not null"`
}

func (userListPO) TableName() string {
	return "user_lists"
}

type userListMembershipPO struct {
	ListID    int64     `gorm:"primaryKey"`
	UserID    int64     `gorm:"primaryKey"`
	CreatedAt time.Time `gorm:"not null"`
}

func (userListMembershipPO) TableName() string {
	return "user_list_memberships"
}

type userListFavoritePO struct {
	ListID    int64     `gorm:"primaryKey"`
	UserID    int64     `gorm:"primaryKey"`
	CreatedAt time.Time `gorm:"not null"`
}

func (userListFavoritePO) TableName() string {
	return "user_list_favorites"
}

type userListViewPO struct {
	ID            int64
	OwnerID       int64
	Name          string
	IsPublic      bool
	MemberCount   int64
	FavoriteCount int64
	IsFavorited   bool
	CreatedAt     time.Time
	UpdatedAt     time.Time
}

const userListViewColumns = `
	l.id,
	l.owner_id,
	l.name,
	l.is_public,
	l.created_at,
	l.updated_at,
	(SELECT COUNT(*) FROM user_list_memberships AS m WHERE m.list_id = l.id) AS member_count,
	(SELECT COUNT(*) FROM user_list_favorites AS f WHERE f.list_id = l.id) AS favorite_count,
	EXISTS (
		SELECT 1 FROM user_list_favorites AS vf
		WHERE vf.list_id = l.id AND vf.user_id = ?
	) AS is_favorited`

var _ domain.UserListRepository = (*Repo)(nil)

func (r *Repo) CreateUserList(ctx context.Context, list *domain.UserList) error {
	if err := list.Validate(); err != nil {
		return err
	}
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := lockUserListOwner(tx, list.OwnerID); err != nil {
			return err
		}
		if err := ensureUserExists(tx, list.OwnerID); err != nil {
			return err
		}
		var count int64
		if err := tx.Model(&userListPO{}).Where("owner_id = ?", list.OwnerID).Count(&count).Error; err != nil {
			return err
		}
		if count >= domain.MaxUserListsPerOwner {
			return domain.ErrUserListLimitReached
		}
		return mapUserListWriteError(tx.Create(toUserListPO(list)).Error)
	})
}

func (r *Repo) UpdateUserList(ctx context.Context, list *domain.UserList) error {
	if err := list.Validate(); err != nil {
		return err
	}
	res := r.db.WithContext(ctx).Model(&userListPO{}).
		Where("id = ? AND owner_id = ?", list.ID, list.OwnerID).
		Updates(map[string]any{
			"name":       list.Name,
			"is_public":  list.IsPublic,
			"updated_at": list.UpdatedAt,
		})
	if res.Error != nil {
		return mapUserListWriteError(res.Error)
	}
	if res.RowsAffected == 0 {
		return domain.ErrUserListNotFound
	}
	return nil
}

func (r *Repo) DeleteUserList(ctx context.Context, ownerID, listID int64) error {
	if ownerID <= 0 || listID <= 0 {
		return domain.ErrInvalidID
	}
	res := r.db.WithContext(ctx).
		Where("id = ? AND owner_id = ?", listID, ownerID).
		Delete(&userListPO{})
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		return domain.ErrUserListNotFound
	}
	return nil
}

func (r *Repo) GetUserList(ctx context.Context, viewerID, listID int64) (*domain.UserList, error) {
	if viewerID < 0 || listID <= 0 {
		return nil, domain.ErrInvalidID
	}
	var row userListViewPO
	err := r.db.WithContext(ctx).Table("user_lists AS l").
		Select(userListViewColumns, viewerID).
		Where("l.id = ?", listID).
		Where("l.is_public = TRUE OR l.owner_id = ?", viewerID).
		Take(&row).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, domain.ErrUserListNotFound
	}
	if err != nil {
		return nil, err
	}
	return toUserList(&row), nil
}

func (r *Repo) ListUserLists(ctx context.Context, q domain.UserListsQuery) ([]*domain.UserList, int64, error) {
	if q.ViewerID < 0 || q.OwnerID <= 0 {
		return nil, 0, domain.ErrInvalidID
	}
	normalizeList(&q.Page, &q.PageSize)
	base := r.db.WithContext(ctx).Table("user_lists AS l").
		Where("l.owner_id = ?", q.OwnerID).
		Where("l.is_public = TRUE OR l.owner_id = ?", q.ViewerID)
	var total int64
	if err := base.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	var rows []userListViewPO
	err := base.Select(userListViewColumns, q.ViewerID).
		Order("l.created_at DESC, l.id DESC").
		Limit(q.PageSize).
		Offset((q.Page - 1) * q.PageSize).
		Scan(&rows).Error
	if err != nil {
		return nil, 0, err
	}
	items := make([]*domain.UserList, 0, len(rows))
	for i := range rows {
		items = append(items, toUserList(&rows[i]))
	}
	return items, total, nil
}

func (r *Repo) ListFavoriteUserLists(ctx context.Context, q domain.UserListFavoritesQuery) ([]*domain.UserList, int64, error) {
	if q.UserID <= 0 {
		return nil, 0, domain.ErrInvalidID
	}
	normalizeList(&q.Page, &q.PageSize)
	base := r.db.WithContext(ctx).Table("user_lists AS l").
		Joins("JOIN user_list_favorites AS favorite ON favorite.list_id = l.id").
		Where("favorite.user_id = ?", q.UserID).
		Where("l.is_public = TRUE")
	var total int64
	if err := base.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	var rows []userListViewPO
	err := base.Select(userListViewColumns, q.UserID).
		Order("favorite.created_at DESC, favorite.list_id DESC").
		Limit(q.PageSize).
		Offset((q.Page - 1) * q.PageSize).
		Scan(&rows).Error
	if err != nil {
		return nil, 0, err
	}
	items := make([]*domain.UserList, 0, len(rows))
	for i := range rows {
		items = append(items, toUserList(&rows[i]))
	}
	return items, total, nil
}

func (r *Repo) AddUserListMember(ctx context.Context, ownerID int64, membership domain.UserListMembership) error {
	if ownerID <= 0 || membership.ListID <= 0 || membership.UserID <= 0 {
		return domain.ErrInvalidID
	}
	if membership.CreatedAt.IsZero() {
		membership.CreatedAt = time.Now()
	}
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := lockOwnedUserList(tx, ownerID, membership.ListID); err != nil {
			return err
		}
		if err := ensureUserExists(tx, membership.UserID); err != nil {
			return err
		}
		var existing int64
		if err := tx.Model(&userListMembershipPO{}).
			Where("list_id = ? AND user_id = ?", membership.ListID, membership.UserID).
			Count(&existing).Error; err != nil {
			return err
		}
		if existing > 0 {
			return domain.ErrUserListMemberExists
		}
		var count int64
		if err := tx.Model(&userListMembershipPO{}).
			Where("list_id = ?", membership.ListID).
			Count(&count).Error; err != nil {
			return err
		}
		if count >= domain.MaxUserListMembers {
			return domain.ErrUserListMemberLimitReached
		}
		blocked, err := userBlocks(tx, membership.UserID, ownerID)
		if err != nil {
			return err
		}
		if blocked {
			return domain.ErrUserListMemberBlocked
		}
		row := userListMembershipPO{
			ListID:    membership.ListID,
			UserID:    membership.UserID,
			CreatedAt: membership.CreatedAt,
		}
		return mapUserListWriteError(tx.Create(&row).Error)
	})
}

func (r *Repo) RemoveUserListMember(ctx context.Context, ownerID, listID, userID int64) error {
	if ownerID <= 0 || listID <= 0 || userID <= 0 {
		return domain.ErrInvalidID
	}
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := lockOwnedUserList(tx, ownerID, listID); err != nil {
			return err
		}
		res := tx.Where("list_id = ? AND user_id = ?", listID, userID).
			Delete(&userListMembershipPO{})
		if res.Error != nil {
			return res.Error
		}
		if res.RowsAffected == 0 {
			return domain.ErrUserListMemberNotFound
		}
		return nil
	})
}

func (r *Repo) ListUserListMembers(ctx context.Context, q domain.UserListMembersQuery) ([]*domain.User, int64, error) {
	if q.ViewerID < 0 || q.ListID <= 0 {
		return nil, 0, domain.ErrInvalidID
	}
	normalizeList(&q.Page, &q.PageSize)
	visible := r.db.WithContext(ctx).Table("user_lists AS l").
		Where("l.id = ?", q.ListID).
		Where("l.is_public = TRUE OR l.owner_id = ?", q.ViewerID)
	var visibleCount int64
	if err := visible.Count(&visibleCount).Error; err != nil {
		return nil, 0, err
	}
	if visibleCount == 0 {
		return nil, 0, domain.ErrUserListNotFound
	}
	var total int64
	if err := r.db.WithContext(ctx).Model(&userListMembershipPO{}).
		Where("list_id = ?", q.ListID).
		Count(&total).Error; err != nil {
		return nil, 0, err
	}
	var rows []userPO
	err := r.db.WithContext(ctx).Table("users").
		Joins("JOIN user_list_memberships AS m ON m.user_id = users.id").
		Where("m.list_id = ?", q.ListID).
		Order("m.created_at DESC, m.user_id DESC").
		Limit(q.PageSize).
		Offset((q.Page - 1) * q.PageSize).
		Find(&rows).Error
	if err != nil {
		return nil, 0, err
	}
	return toEntities(rows), total, nil
}

func (r *Repo) CopyUserList(ctx context.Context, sourceListID int64, target *domain.UserList) error {
	if sourceListID <= 0 {
		return domain.ErrInvalidID
	}
	if err := target.Validate(); err != nil {
		return err
	}
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := lockUserListOwner(tx, target.OwnerID); err != nil {
			return err
		}
		if err := ensureUserExists(tx, target.OwnerID); err != nil {
			return err
		}
		var source userListPO
		err := tx.Clauses(clause.Locking{Strength: "SHARE"}).
			Where("id = ? AND is_public = TRUE", sourceListID).
			Take(&source).Error
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return domain.ErrUserListNotFound
		}
		if err != nil {
			return err
		}
		var count int64
		if err := tx.Model(&userListPO{}).Where("owner_id = ?", target.OwnerID).Count(&count).Error; err != nil {
			return err
		}
		if count >= domain.MaxUserListsPerOwner {
			return domain.ErrUserListLimitReached
		}
		var members []userListMembershipPO
		if err := tx.Where("list_id = ?", sourceListID).
			Order("created_at ASC, user_id ASC").
			Find(&members).Error; err != nil {
			return err
		}
		if len(members) > domain.MaxUserListMembers {
			return domain.ErrUserListMemberLimitReached
		}
		if len(members) > 0 {
			memberIDs := make([]int64, 0, len(members))
			for _, member := range members {
				memberIDs = append(memberIDs, member.UserID)
			}
			var blockedCount int64
			if err := tx.Model(&blockPO{}).
				Where("actor_id IN ? AND target_id = ?", memberIDs, target.OwnerID).
				Count(&blockedCount).Error; err != nil {
				return err
			}
			if blockedCount > 0 {
				return domain.ErrUserListMemberBlocked
			}
		}
		if err := mapUserListWriteError(tx.Create(toUserListPO(target)).Error); err != nil {
			return err
		}
		if len(members) == 0 {
			return nil
		}
		copiedAt := target.CreatedAt
		rows := make([]userListMembershipPO, 0, len(members))
		for _, member := range members {
			rows = append(rows, userListMembershipPO{
				ListID:    target.ID,
				UserID:    member.UserID,
				CreatedAt: copiedAt,
			})
		}
		return mapUserListWriteError(tx.Create(&rows).Error)
	})
}

func (r *Repo) FavoriteUserList(ctx context.Context, favorite domain.UserListFavorite) error {
	if favorite.ListID <= 0 || favorite.UserID <= 0 {
		return domain.ErrInvalidID
	}
	if favorite.CreatedAt.IsZero() {
		favorite.CreatedAt = time.Now()
	}
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := ensureUserExists(tx, favorite.UserID); err != nil {
			return err
		}
		var count int64
		if err := tx.Model(&userListPO{}).
			Where("id = ? AND is_public = TRUE", favorite.ListID).
			Count(&count).Error; err != nil {
			return err
		}
		if count == 0 {
			return domain.ErrUserListNotFound
		}
		row := userListFavoritePO{
			ListID:    favorite.ListID,
			UserID:    favorite.UserID,
			CreatedAt: favorite.CreatedAt,
		}
		return mapUserListWriteError(tx.Create(&row).Error)
	})
}

func (r *Repo) UnfavoriteUserList(ctx context.Context, userID, listID int64) error {
	if userID <= 0 || listID <= 0 {
		return domain.ErrInvalidID
	}
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var count int64
		if err := tx.Model(&userListPO{}).
			Where("id = ? AND is_public = TRUE", listID).
			Count(&count).Error; err != nil {
			return err
		}
		if count == 0 {
			return domain.ErrUserListNotFound
		}
		res := tx.Where("list_id = ? AND user_id = ?", listID, userID).
			Delete(&userListFavoritePO{})
		if res.Error != nil {
			return res.Error
		}
		if res.RowsAffected == 0 {
			return domain.ErrUserListFavoriteNotFound
		}
		return nil
	})
}

func toUserListPO(list *domain.UserList) *userListPO {
	return &userListPO{
		ID:        list.ID,
		OwnerID:   list.OwnerID,
		Name:      list.Name,
		IsPublic:  list.IsPublic,
		CreatedAt: list.CreatedAt,
		UpdatedAt: list.UpdatedAt,
	}
}

func toUserList(row *userListViewPO) *domain.UserList {
	return &domain.UserList{
		ID:            row.ID,
		OwnerID:       row.OwnerID,
		Name:          row.Name,
		IsPublic:      row.IsPublic,
		MemberCount:   row.MemberCount,
		FavoriteCount: row.FavoriteCount,
		IsFavorited:   row.IsFavorited,
		CreatedAt:     row.CreatedAt,
		UpdatedAt:     row.UpdatedAt,
	}
}

func lockUserListOwner(tx *gorm.DB, ownerID int64) error {
	return tx.Exec("SELECT pg_advisory_xact_lock(hashtextextended(?, 0))", fmt.Sprintf("user-list-owner:%d", ownerID)).Error
}

func lockOwnedUserList(tx *gorm.DB, ownerID, listID int64) error {
	var list userListPO
	err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
		Where("id = ? AND owner_id = ?", listID, ownerID).
		Take(&list).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return domain.ErrUserListNotFound
	}
	return err
}

func ensureUserExists(tx *gorm.DB, userID int64) error {
	var count int64
	if err := tx.Model(&userPO{}).Where("id = ?", userID).Count(&count).Error; err != nil {
		return err
	}
	if count == 0 {
		return domain.ErrNotFound
	}
	return nil
}

func userBlocks(tx *gorm.DB, actorID, targetID int64) (bool, error) {
	var count int64
	err := tx.Model(&blockPO{}).
		Where("actor_id = ? AND target_id = ?", actorID, targetID).
		Count(&count).Error
	return count > 0, err
}

func mapUserListWriteError(err error) error {
	if err == nil {
		return nil
	}
	var pgErr *pgconn.PgError
	if !errors.As(err, &pgErr) {
		return err
	}
	switch pgErr.Code {
	case "23503":
		return domain.ErrNotFound
	case "23505":
		switch pgErr.ConstraintName {
		case "uk_user_lists_owner_name_ci":
			return domain.ErrUserListNameExists
		case "user_list_memberships_pkey":
			return domain.ErrUserListMemberExists
		case "user_list_favorites_pkey":
			return domain.ErrUserListFavoriteExists
		}
		if strings.Contains(pgErr.Detail, "owner_id") && strings.Contains(pgErr.Detail, "lower(name)") {
			return domain.ErrUserListNameExists
		}
	}
	return err
}
