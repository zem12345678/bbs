package store

import (
	"context"
	"errors"
	"time"

	domain "reaction-service/internal/domain/reaction"

	"gorm.io/gorm"
)

type favoritePO struct {
	ID         int64      `gorm:"primaryKey"`
	UserID     int64      `gorm:"not null;uniqueIndex:idx_favorites_user_entity;index:idx_favorites_user_deleted_created"`
	EntityType string     `gorm:"size:32;not null;uniqueIndex:idx_favorites_user_entity;index:idx_favorites_entity_deleted"`
	EntityID   int64      `gorm:"not null;uniqueIndex:idx_favorites_user_entity;index:idx_favorites_entity_deleted"`
	DeletedAt  *time.Time `gorm:"index:idx_favorites_user_deleted_created;index:idx_favorites_entity_deleted"`
	CreatedAt  time.Time  `gorm:"not null;default:now();index:idx_favorites_user_deleted_created"`
	UpdatedAt  time.Time  `gorm:"not null;default:now()"`
}

func (favoritePO) TableName() string {
	return "favorites"
}

type PostgresFavoriteRepository struct {
	db *gorm.DB
}

func NewPostgresFavoriteRepository(db *gorm.DB) *PostgresFavoriteRepository {
	return &PostgresFavoriteRepository{db: db}
}

func (r *PostgresFavoriteRepository) EnsureSchema(ctx context.Context) error {
	if err := r.db.WithContext(ctx).Exec(`
CREATE TABLE IF NOT EXISTS favorites (
  id BIGSERIAL PRIMARY KEY,
  user_id BIGINT NOT NULL,
  entity_type VARCHAR(32) NOT NULL,
  entity_id BIGINT NOT NULL,
  deleted_at TIMESTAMPTZ,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  UNIQUE(user_id, entity_type, entity_id)
)`).Error; err != nil {
		return err
	}
	if err := r.db.WithContext(ctx).Exec(`
CREATE INDEX IF NOT EXISTS idx_favorites_user_deleted_created
  ON favorites(user_id, deleted_at, created_at DESC)`).Error; err != nil {
		return err
	}
	return r.db.WithContext(ctx).Exec(`
CREATE INDEX IF NOT EXISTS idx_favorites_entity_deleted
  ON favorites(entity_type, entity_id, deleted_at)`).Error
}

func (r *PostgresFavoriteRepository) Favorite(ctx context.Context, ref domain.EntityRef, userID int64) (int64, bool, error) {
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
		var existing favoritePO
		err := tx.Where("user_id = ? AND entity_type = ? AND entity_id = ?", userID, string(ref.Type), ref.ID).First(&existing).Error
		switch {
		case errors.Is(err, gorm.ErrRecordNotFound):
			now := time.Now()
			po := favoritePO{UserID: userID, EntityType: string(ref.Type), EntityID: ref.ID, CreatedAt: now, UpdatedAt: now}
			if err := tx.Create(&po).Error; err != nil {
				return err
			}
			changed = true
		case err != nil:
			return err
		case existing.DeletedAt != nil:
			now := time.Now()
			if err := tx.Model(&favoritePO{}).Where("id = ?", existing.ID).Updates(map[string]any{
				"deleted_at": nil, "created_at": now, "updated_at": now,
			}).Error; err != nil {
				return err
			}
			changed = true
		}
		return countFavorites(tx, ref, &count)
	})
	return count, changed, err
}

func (r *PostgresFavoriteRepository) Unfavorite(ctx context.Context, ref domain.EntityRef, userID int64) (int64, bool, error) {
	if err := ref.Validate(); err != nil {
		return 0, false, err
	}
	if userID <= 0 {
		return 0, false, domain.ErrInvalidUserID
	}
	now := time.Now()
	result := r.db.WithContext(ctx).Model(&favoritePO{}).
		Where("user_id = ? AND entity_type = ? AND entity_id = ? AND deleted_at IS NULL", userID, string(ref.Type), ref.ID).
		Updates(map[string]any{
			"deleted_at": &now,
			"updated_at": now,
		})
	if result.Error != nil {
		return 0, false, result.Error
	}
	count, err := r.Count(ctx, ref)
	return count, result.RowsAffected > 0, err
}

func (r *PostgresFavoriteRepository) Count(ctx context.Context, ref domain.EntityRef) (int64, error) {
	if err := ref.Validate(); err != nil {
		return 0, err
	}
	var count int64
	err := r.db.WithContext(ctx).Model(&favoritePO{}).
		Where("entity_type = ? AND entity_id = ? AND deleted_at IS NULL", string(ref.Type), ref.ID).
		Count(&count).Error
	return count, err
}

func countFavorites(db *gorm.DB, ref domain.EntityRef, count *int64) error {
	return db.Model(&favoritePO{}).
		Where("entity_type = ? AND entity_id = ? AND deleted_at IS NULL", string(ref.Type), ref.ID).
		Count(count).Error
}

func (r *PostgresFavoriteRepository) ListFavorites(ctx context.Context, userID int64, entityType domain.EntityType, limit, offset int) ([]*domain.Favorite, int64, error) {
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

	query := r.db.WithContext(ctx).Model(&favoritePO{}).Where("user_id = ? AND deleted_at IS NULL", userID)
	if entityType != "" {
		query = query.Where("entity_type = ?", string(entityType))
	}
	var total int64
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	var rows []favoritePO
	if err := query.Order("created_at DESC, id DESC").Limit(limit).Offset(offset).Find(&rows).Error; err != nil {
		return nil, 0, err
	}
	out := make([]*domain.Favorite, 0, len(rows))
	for i := range rows {
		out = append(out, toFavoriteEntity(&rows[i]))
	}
	return out, total, nil
}

func toFavoriteEntity(po *favoritePO) *domain.Favorite {
	if po == nil {
		return nil
	}
	return &domain.Favorite{
		ID:        po.ID,
		UserID:    po.UserID,
		Entity:    domain.EntityRef{Type: domain.EntityType(po.EntityType), ID: po.EntityID},
		CreatedAt: po.CreatedAt,
		UpdatedAt: po.UpdatedAt,
	}
}
