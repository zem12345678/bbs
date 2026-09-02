package store

import (
	"context"
	"errors"
	"time"

	domain "reaction-service/internal/domain/reaction"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type reactionPO struct {
	ID         int64     `gorm:"primaryKey"`
	UserID     int64     `gorm:"not null;index:idx_user_reactions_user_created"`
	EntityType string    `gorm:"size:32;not null;index:idx_user_reactions_entity"`
	EntityID   int64     `gorm:"not null;index:idx_user_reactions_entity"`
	Reaction   string    `gorm:"size:128;not null"`
	CreatedAt  time.Time `gorm:"not null;default:now();index:idx_user_reactions_user_created"`
	UpdatedAt  time.Time `gorm:"not null;default:now()"`
}

func (reactionPO) TableName() string { return "user_reactions" }

type PostgresReactionRepository struct {
	db *gorm.DB
}

func NewPostgresReactionRepository(db *gorm.DB) *PostgresReactionRepository {
	return &PostgresReactionRepository{db: db}
}

func (r *PostgresReactionRepository) EnsureSchema(ctx context.Context) error {
	if err := r.db.WithContext(ctx).AutoMigrate(&reactionPO{}); err != nil {
		return err
	}
	if err := r.db.WithContext(ctx).Exec(`DROP INDEX IF EXISTS ux_user_reactions_identity`).Error; err != nil {
		return err
	}
	return r.db.WithContext(ctx).Exec(`
CREATE UNIQUE INDEX IF NOT EXISTS ux_user_reactions_identity
  ON user_reactions(user_id, entity_type, entity_id)`).Error
}

func (r *PostgresReactionRepository) CreateReaction(ctx context.Context, ref domain.EntityRef, userID int64, reaction string) (*domain.Reaction, bool, error) {
	if err := ref.Validate(); err != nil {
		return nil, false, err
	}
	if userID <= 0 {
		return nil, false, domain.ErrInvalidUserID
	}
	reaction, err := domain.NormalizeReaction(reaction)
	if err != nil {
		return nil, false, err
	}
	var row reactionPO
	created := false
	err = r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := ensureReactionUserActive(tx, userID); err != nil {
			return err
		}
		queryErr := tx.Where("user_id = ? AND entity_type = ? AND entity_id = ?", userID, string(ref.Type), ref.ID).First(&row).Error
		if queryErr == nil {
			return domain.ErrReactionAlreadyExists
		}
		if !errors.Is(queryErr, gorm.ErrRecordNotFound) {
			return queryErr
		}
		now := time.Now().UTC()
		row = reactionPO{UserID: userID, EntityType: string(ref.Type), EntityID: ref.ID, Reaction: reaction, CreatedAt: now, UpdatedAt: now}
		if err := tx.Clauses(clause.OnConflict{DoNothing: true}).Create(&row).Error; err != nil {
			return err
		}
		created = row.ID != 0
		if !created {
			return tx.Where("user_id = ? AND entity_type = ? AND entity_id = ?", userID, string(ref.Type), ref.ID).First(&row).Error
		}
		return nil
	})
	if err != nil {
		return nil, false, err
	}
	return toReactionEntity(&row), created, nil
}

func (r *PostgresReactionRepository) DeleteReaction(ctx context.Context, ref domain.EntityRef, userID int64) (bool, error) {
	if err := ref.Validate(); err != nil {
		return false, err
	}
	if userID <= 0 {
		return false, domain.ErrInvalidUserID
	}
	if err := ensureReactionUserActive(r.db.WithContext(ctx), userID); err != nil {
		return false, err
	}
	result := r.db.WithContext(ctx).Where("user_id = ? AND entity_type = ? AND entity_id = ?", userID, string(ref.Type), ref.ID).Delete(&reactionPO{})
	if result.Error != nil {
		return false, result.Error
	}
	if result.RowsAffected == 0 {
		return false, domain.ErrReactionNotFound
	}
	return true, nil
}

func (r *PostgresReactionRepository) ListReactions(ctx context.Context, userID int64, entityType domain.EntityType, limit, offset int, sinceID, untilID, sinceDate, untilDate int64) ([]*domain.Reaction, int64, error) {
	if userID <= 0 {
		return nil, 0, domain.ErrInvalidUserID
	}
	if entityType != "" && !entityType.Valid() {
		return nil, 0, domain.ErrInvalidEntityType
	}
	if sinceID < 0 || untilID < 0 || sinceDate < 0 || untilDate < 0 {
		return nil, 0, domain.ErrInvalidReactionCursor
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
	query := r.db.WithContext(ctx).Model(&reactionPO{}).Where("user_id = ?", userID)
	if entityType != "" {
		query = query.Where("entity_type = ?", string(entityType))
	}
	if sinceID > 0 {
		query = query.Where("id > ?", sinceID)
	}
	if untilID > 0 {
		query = query.Where("id < ?", untilID)
	}
	if sinceDate > 0 {
		query = query.Where("created_at > ?", time.UnixMilli(sinceDate))
	}
	if untilDate > 0 {
		query = query.Where("created_at < ?", time.UnixMilli(untilDate))
	}
	var total int64
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	var rows []reactionPO
	if err := query.Order("created_at DESC, id DESC").Limit(limit).Offset(offset).Find(&rows).Error; err != nil {
		return nil, 0, err
	}
	out := make([]*domain.Reaction, 0, len(rows))
	for i := range rows {
		out = append(out, toReactionEntity(&rows[i]))
	}
	return out, total, nil
}

func toReactionEntity(row *reactionPO) *domain.Reaction {
	if row == nil {
		return nil
	}
	return &domain.Reaction{
		ID: row.ID, UserID: row.UserID,
		Entity:   domain.EntityRef{Type: domain.EntityType(row.EntityType), ID: row.EntityID},
		Reaction: row.Reaction, CreatedAt: row.CreatedAt, UpdatedAt: row.UpdatedAt,
	}
}

var _ domain.ReactionRepository = (*PostgresReactionRepository)(nil)
