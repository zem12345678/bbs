package store

import (
	"context"
	"errors"
	"time"

	domain "reaction-service/internal/domain/reaction"

	"gorm.io/gorm"
)

const (
	likeStatusInactive int16 = 0
	likeStatusActive   int16 = 1
)

type likePO struct {
	ID         int64     `gorm:"primaryKey"`
	UserID     int64     `gorm:"not null;uniqueIndex:idx_user_likes_user_entity;index:idx_user_likes_user_status_created"`
	EntityType string    `gorm:"size:32;not null;uniqueIndex:idx_user_likes_user_entity;index:idx_user_likes_entity_status"`
	EntityID   int64     `gorm:"not null;uniqueIndex:idx_user_likes_user_entity;index:idx_user_likes_entity_status"`
	Status     int16     `gorm:"not null;default:1;index:idx_user_likes_user_status_created;index:idx_user_likes_entity_status"`
	CreatedAt  time.Time `gorm:"not null;default:now();index:idx_user_likes_user_status_created"`
	UpdatedAt  time.Time `gorm:"not null;default:now()"`
}

func (likePO) TableName() string {
	return "user_likes"
}

type PostgresLikeRepository struct {
	db *gorm.DB
}

func NewPostgresLikeRepository(db *gorm.DB) *PostgresLikeRepository {
	return &PostgresLikeRepository{db: db}
}

func (r *PostgresLikeRepository) EnsureSchema(ctx context.Context) error {
	if err := r.db.WithContext(ctx).Exec(`
CREATE TABLE IF NOT EXISTS user_likes (
  id BIGSERIAL PRIMARY KEY,
  user_id BIGINT NOT NULL,
  entity_type VARCHAR(32) NOT NULL,
  entity_id BIGINT NOT NULL,
  status SMALLINT NOT NULL DEFAULT 1,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  UNIQUE(user_id, entity_type, entity_id)
)`).Error; err != nil {
		return err
	}
	if err := r.db.WithContext(ctx).Exec(`
CREATE INDEX IF NOT EXISTS idx_user_likes_entity_status
  ON user_likes(entity_type, entity_id, status)`).Error; err != nil {
		return err
	}
	return r.db.WithContext(ctx).Exec(`
CREATE INDEX IF NOT EXISTS idx_user_likes_user_status_created
  ON user_likes(user_id, status, created_at DESC)`).Error
}

func (r *PostgresLikeRepository) Like(ctx context.Context, ref domain.EntityRef, userID int64) (int64, bool, error) {
	if err := ref.Validate(); err != nil {
		return 0, false, err
	}
	if userID <= 0 {
		return 0, false, domain.ErrInvalidUserID
	}

	var count int64
	var changed bool
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := ensureReactionUserActive(tx, userID); err != nil {
			return err
		}
		var existing likePO
		err := tx.Where("user_id = ? AND entity_type = ? AND entity_id = ?", userID, string(ref.Type), ref.ID).First(&existing).Error
		switch {
		case errors.Is(err, gorm.ErrRecordNotFound):
			now := time.Now()
			po := likePO{UserID: userID, EntityType: string(ref.Type), EntityID: ref.ID, Status: likeStatusActive, CreatedAt: now, UpdatedAt: now}
			if err := tx.Create(&po).Error; err != nil {
				return err
			}
			changed = true
		case err != nil:
			return err
		case existing.Status != likeStatusActive:
			now := time.Now()
			if err := tx.Model(&likePO{}).Where("id = ?", existing.ID).Updates(map[string]any{
				"status": likeStatusActive, "created_at": now, "updated_at": now,
			}).Error; err != nil {
				return err
			}
			changed = true
		}
		return countLikes(tx, ref, &count)
	})
	return count, changed, err
}

func (r *PostgresLikeRepository) Unlike(ctx context.Context, ref domain.EntityRef, userID int64) (int64, bool, error) {
	if err := ref.Validate(); err != nil {
		return 0, false, err
	}
	if userID <= 0 {
		return 0, false, domain.ErrInvalidUserID
	}
	now := time.Now()
	result := r.db.WithContext(ctx).Model(&likePO{}).
		Where("user_id = ? AND entity_type = ? AND entity_id = ? AND status = ?", userID, string(ref.Type), ref.ID, likeStatusActive).
		Updates(map[string]any{
			"status":     likeStatusInactive,
			"updated_at": now,
		})
	if result.Error != nil {
		return 0, false, result.Error
	}
	count, err := r.Count(ctx, ref)
	return count, result.RowsAffected > 0, err
}

func (r *PostgresLikeRepository) Count(ctx context.Context, ref domain.EntityRef) (int64, error) {
	if err := ref.Validate(); err != nil {
		return 0, err
	}
	var count int64
	err := r.db.WithContext(ctx).Model(&likePO{}).
		Where("entity_type = ? AND entity_id = ? AND status = ?", string(ref.Type), ref.ID, likeStatusActive).
		Count(&count).Error
	return count, err
}

func countLikes(db *gorm.DB, ref domain.EntityRef, count *int64) error {
	return db.Model(&likePO{}).
		Where("entity_type = ? AND entity_id = ? AND status = ?", string(ref.Type), ref.ID, likeStatusActive).
		Count(count).Error
}

func (r *PostgresLikeRepository) HotIDs(ctx context.Context, entityType domain.EntityType, limit int) ([]int64, error) {
	if !entityType.Valid() {
		return nil, domain.ErrInvalidEntityType
	}
	if limit <= 0 {
		limit = 20
	}
	if limit > 100 {
		limit = 100
	}
	var rows []struct {
		EntityID int64 `gorm:"column:entity_id"`
	}
	err := r.db.WithContext(ctx).Model(&likePO{}).
		Select("entity_id").
		Where("entity_type = ? AND status = ?", string(entityType), likeStatusActive).
		Group("entity_id").
		Order("COUNT(*) DESC, MAX(updated_at) DESC, entity_id DESC").
		Limit(limit).
		Scan(&rows).Error
	if err != nil {
		return nil, err
	}
	ids := make([]int64, 0, len(rows))
	for _, row := range rows {
		ids = append(ids, row.EntityID)
	}
	return ids, nil
}

func (r *PostgresLikeRepository) ListLikes(ctx context.Context, userID int64, entityType domain.EntityType, limit, offset int) ([]*domain.Like, int64, error) {
	if userID <= 0 {
		return nil, 0, domain.ErrInvalidUserID
	}
	if entityType != "" && !entityType.Valid() {
		return nil, 0, domain.ErrInvalidEntityType
	}
	if limit <= 0 {
		limit = 20
	}
	if limit > 100 {
		limit = 100
	}
	if offset < 0 {
		offset = 0
	}

	query := r.db.WithContext(ctx).Model(&likePO{}).Where("user_id = ? AND status = ?", userID, likeStatusActive)
	if entityType != "" {
		query = query.Where("entity_type = ?", string(entityType))
	}
	var total int64
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	var rows []likePO
	if err := query.Order("created_at DESC, id DESC").Limit(limit).Offset(offset).Find(&rows).Error; err != nil {
		return nil, 0, err
	}
	out := make([]*domain.Like, 0, len(rows))
	for i := range rows {
		out = append(out, toLikeEntity(&rows[i]))
	}
	return out, total, nil
}

func toLikeEntity(po *likePO) *domain.Like {
	if po == nil {
		return nil
	}
	return &domain.Like{
		ID:        po.ID,
		UserID:    po.UserID,
		Entity:    domain.EntityRef{Type: domain.EntityType(po.EntityType), ID: po.EntityID},
		CreatedAt: po.CreatedAt,
		UpdatedAt: po.UpdatedAt,
	}
}
