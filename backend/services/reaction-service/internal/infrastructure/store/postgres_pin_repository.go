package store

import (
	"context"
	"errors"
	"time"

	domain "reaction-service/internal/domain/reaction"

	"gorm.io/gorm"
)

type pinPO struct {
	ID         int64     `gorm:"primaryKey"`
	UserID     int64     `gorm:"not null;uniqueIndex:idx_user_pins_user_entity;index:idx_user_pins_user_created"`
	EntityType string    `gorm:"size:32;not null;uniqueIndex:idx_user_pins_user_entity"`
	EntityID   int64     `gorm:"not null;uniqueIndex:idx_user_pins_user_entity"`
	CreatedAt  time.Time `gorm:"not null;default:now();index:idx_user_pins_user_created"`
}

func (pinPO) TableName() string { return "user_pins" }

type PostgresPinRepository struct {
	db *gorm.DB
}

func NewPostgresPinRepository(db *gorm.DB) *PostgresPinRepository {
	return &PostgresPinRepository{db: db}
}

func (r *PostgresPinRepository) EnsureSchema(ctx context.Context) error {
	statements := []string{
		`CREATE TABLE IF NOT EXISTS user_pins (
  id BIGSERIAL PRIMARY KEY,
  user_id BIGINT NOT NULL,
  entity_type VARCHAR(32) NOT NULL CHECK (entity_type IN ('article', 'topic')),
  entity_id BIGINT NOT NULL,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  UNIQUE(user_id, entity_type, entity_id)
)`,
		`CREATE INDEX IF NOT EXISTS idx_user_pins_user_created
  ON user_pins(user_id, created_at DESC, id DESC)`,
	}
	for _, statement := range statements {
		if err := r.db.WithContext(ctx).Exec(statement).Error; err != nil {
			return err
		}
	}
	return nil
}

func (r *PostgresPinRepository) Pin(ctx context.Context, ref domain.EntityRef, userID int64) (int64, bool, error) {
	if err := ref.ValidateForPin(); err != nil {
		return 0, false, err
	}
	if userID <= 0 {
		return 0, false, domain.ErrInvalidUserID
	}

	var count int64
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := ensureReactionUserActive(tx, userID); err != nil {
			return err
		}
		var existing pinPO
		err := tx.Where("user_id = ? AND entity_type = ? AND entity_id = ?", userID, string(ref.Type), ref.ID).First(&existing).Error
		switch {
		case err == nil:
			return domain.ErrAlreadyPinned
		case !errors.Is(err, gorm.ErrRecordNotFound):
			return err
		}
		if err := tx.Model(&pinPO{}).Where("user_id = ?", userID).Count(&count).Error; err != nil {
			return err
		}
		if count >= domain.MaxPinsPerUser {
			return domain.ErrPinLimitExceeded
		}
		if err := tx.Create(&pinPO{UserID: userID, EntityType: string(ref.Type), EntityID: ref.ID, CreatedAt: time.Now().UTC()}).Error; err != nil {
			return err
		}
		count++
		return nil
	})
	return count, err == nil, err
}

func (r *PostgresPinRepository) Unpin(ctx context.Context, ref domain.EntityRef, userID int64) (int64, bool, error) {
	if err := ref.ValidateForPin(); err != nil {
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
		result := tx.Where("user_id = ? AND entity_type = ? AND entity_id = ?", userID, string(ref.Type), ref.ID).Delete(&pinPO{})
		if result.Error != nil {
			return result.Error
		}
		changed = result.RowsAffected > 0
		return tx.Model(&pinPO{}).Where("user_id = ?", userID).Count(&count).Error
	})
	return count, changed, err
}

func (r *PostgresPinRepository) ListPins(ctx context.Context, userID int64, limit, offset int) ([]*domain.Pin, int64, error) {
	if userID <= 0 {
		return nil, 0, domain.ErrInvalidUserID
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

	query := r.db.WithContext(ctx).Model(&pinPO{}).Where("user_id = ?", userID)
	var total int64
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	var rows []pinPO
	if err := query.Order("created_at DESC, id DESC").Limit(limit).Offset(offset).Find(&rows).Error; err != nil {
		return nil, 0, err
	}
	out := make([]*domain.Pin, 0, len(rows))
	for index := range rows {
		out = append(out, toPinEntity(&rows[index]))
	}
	return out, total, nil
}

func toPinEntity(po *pinPO) *domain.Pin {
	if po == nil {
		return nil
	}
	return &domain.Pin{
		ID:        po.ID,
		UserID:    po.UserID,
		Entity:    domain.EntityRef{Type: domain.EntityType(po.EntityType), ID: po.EntityID},
		CreatedAt: po.CreatedAt,
	}
}
