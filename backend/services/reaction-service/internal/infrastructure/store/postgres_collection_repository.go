package store

import (
	"context"
	"errors"
	"time"

	domain "reaction-service/internal/domain/reaction"

	"github.com/jackc/pgx/v5/pgconn"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

const collectionNameUniqueConstraint = "ux_favorite_collections_owner_name"

type collectionPO struct {
	ID          int64     `gorm:"primaryKey;autoIncrement"`
	UserID      int64     `gorm:"column:user_id;not null"`
	Name        string    `gorm:"size:80;not null"`
	Description string    `gorm:"type:text;not null;default:''"`
	IsPublic    bool      `gorm:"not null;default:false"`
	ItemCount   int64     `gorm:"->;-:migration"`
	CreatedAt   time.Time `gorm:"not null;default:now()"`
	UpdatedAt   time.Time `gorm:"not null;default:now()"`
}

func (collectionPO) TableName() string { return "favorite_collections" }

type collectionItemPO struct {
	ID           int64     `gorm:"primaryKey;autoIncrement"`
	CollectionID int64     `gorm:"not null"`
	EntityType   string    `gorm:"size:32;not null"`
	EntityID     int64     `gorm:"not null"`
	CreatedAt    time.Time `gorm:"not null;default:now()"`
}

func (collectionItemPO) TableName() string { return "favorite_collection_items" }

type PostgresCollectionRepository struct {
	db *gorm.DB
}

func NewPostgresCollectionRepository(db *gorm.DB) *PostgresCollectionRepository {
	return &PostgresCollectionRepository{db: db}
}

func (r *PostgresCollectionRepository) EnsureSchema(ctx context.Context) error {
	statements := []string{
		`CREATE TABLE IF NOT EXISTS favorite_collections (
  id BIGSERIAL PRIMARY KEY,
  user_id BIGINT NOT NULL,
  name VARCHAR(80) NOT NULL CHECK (char_length(name) BETWEEN 1 AND 80),
  description TEXT NOT NULL DEFAULT '' CHECK (char_length(description) <= 500),
  is_public BOOLEAN NOT NULL DEFAULT FALSE,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
)`,
		`CREATE UNIQUE INDEX IF NOT EXISTS ux_favorite_collections_owner_name
  ON favorite_collections(user_id, lower(name))`,
		`CREATE INDEX IF NOT EXISTS idx_favorite_collections_owner_created
  ON favorite_collections(user_id, created_at DESC, id DESC)`,
		`CREATE TABLE IF NOT EXISTS favorite_collection_items (
  id BIGSERIAL PRIMARY KEY,
  collection_id BIGINT NOT NULL REFERENCES favorite_collections(id) ON DELETE CASCADE,
  entity_type VARCHAR(32) NOT NULL CHECK (entity_type IN ('article', 'topic')),
  entity_id BIGINT NOT NULL,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  UNIQUE(collection_id, entity_type, entity_id)
)`,
		`CREATE INDEX IF NOT EXISTS idx_favorite_collection_items_list
  ON favorite_collection_items(collection_id, created_at DESC, id DESC)`,
		`CREATE INDEX IF NOT EXISTS idx_favorite_collection_items_entity
  ON favorite_collection_items(entity_type, entity_id)`,
	}
	for _, statement := range statements {
		if err := r.db.WithContext(ctx).Exec(statement).Error; err != nil {
			return err
		}
	}
	return nil
}

func (r *PostgresCollectionRepository) CreateCollection(ctx context.Context, collection *domain.Collection) error {
	if collection == nil {
		return domain.ErrCollectionNotFound
	}
	name, description, err := domain.ValidateCollectionFields(collection.UserID, collection.Name, collection.Description)
	if err != nil {
		return err
	}
	now := time.Now()
	po := collectionPO{
		UserID: collection.UserID, Name: name, Description: description, IsPublic: collection.IsPublic,
		CreatedAt: now, UpdatedAt: now,
	}
	if err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := ensureReactionUserActive(tx, collection.UserID); err != nil {
			return err
		}
		return tx.Create(&po).Error
	}); err != nil {
		return mapCollectionWriteError(err)
	}
	*collection = *toCollectionEntity(&po)
	return nil
}

func (r *PostgresCollectionRepository) UpdateCollection(ctx context.Context, userID, collectionID int64, name, description string, isPublic bool) (*domain.Collection, error) {
	name, description, err := domain.ValidateCollectionFields(userID, name, description)
	if err != nil {
		return nil, err
	}
	if collectionID <= 0 {
		return nil, domain.ErrInvalidCollectionID
	}
	var out *domain.Collection
	err = r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := ensureReactionUserActive(tx, userID); err != nil {
			return err
		}
		result := tx.Model(&collectionPO{}).
			Where("id = ? AND user_id = ?", collectionID, userID).
			Updates(map[string]any{
				"name": name, "description": description, "is_public": isPublic, "updated_at": time.Now(),
			})
		if result.Error != nil {
			return mapCollectionWriteError(result.Error)
		}
		if result.RowsAffected == 0 {
			return domain.ErrCollectionNotFound
		}
		collection, err := getOwnedCollection(tx, userID, collectionID)
		if err != nil {
			return err
		}
		out = collection
		return nil
	})
	return out, err
}

func (r *PostgresCollectionRepository) DeleteCollection(ctx context.Context, userID, collectionID int64) error {
	if userID <= 0 {
		return domain.ErrInvalidUserID
	}
	if collectionID <= 0 {
		return domain.ErrInvalidCollectionID
	}
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := ensureReactionUserActive(tx, userID); err != nil {
			return err
		}
		result := tx.Where("id = ? AND user_id = ?", collectionID, userID).Delete(&collectionPO{})
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected == 0 {
			return domain.ErrCollectionNotFound
		}
		return nil
	})
}

func (r *PostgresCollectionRepository) ListCollections(ctx context.Context, userID int64, limit, offset int) ([]*domain.Collection, int64, error) {
	if userID <= 0 {
		return nil, 0, domain.ErrInvalidUserID
	}
	limit, offset = normalizeCollectionPage(limit, offset)
	query := r.db.WithContext(ctx).Model(&collectionPO{}).Where("user_id = ?", userID)
	var total int64
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	var rows []collectionPO
	if err := query.
		Select(`favorite_collections.*,
  (SELECT COUNT(*) FROM favorite_collection_items WHERE collection_id = favorite_collections.id) AS item_count`).
		Order("created_at DESC, id DESC").Limit(limit).Offset(offset).Find(&rows).Error; err != nil {
		return nil, 0, err
	}
	out := make([]*domain.Collection, 0, len(rows))
	for i := range rows {
		out = append(out, toCollectionEntity(&rows[i]))
	}
	return out, total, nil
}

func (r *PostgresCollectionRepository) AddCollectionItem(ctx context.Context, userID, collectionID int64, entity domain.EntityRef) (bool, error) {
	if err := validateCollectionAccess(userID, collectionID, entity); err != nil {
		return false, err
	}
	changed := false
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := ensureReactionUserActive(tx, userID); err != nil {
			return err
		}
		if err := lockOwnedCollection(tx, userID, collectionID); err != nil {
			return err
		}
		po := collectionItemPO{CollectionID: collectionID, EntityType: string(entity.Type), EntityID: entity.ID, CreatedAt: time.Now()}
		result := tx.Clauses(clause.OnConflict{
			Columns:   []clause.Column{{Name: "collection_id"}, {Name: "entity_type"}, {Name: "entity_id"}},
			DoNothing: true,
		}).Create(&po)
		if result.Error != nil {
			return result.Error
		}
		changed = result.RowsAffected > 0
		return nil
	})
	return changed, err
}

func (r *PostgresCollectionRepository) RemoveCollectionItem(ctx context.Context, userID, collectionID int64, entity domain.EntityRef) (bool, error) {
	if err := validateCollectionAccess(userID, collectionID, entity); err != nil {
		return false, err
	}
	changed := false
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := ensureReactionUserActive(tx, userID); err != nil {
			return err
		}
		if err := lockOwnedCollection(tx, userID, collectionID); err != nil {
			return err
		}
		result := tx.Where("collection_id = ? AND entity_type = ? AND entity_id = ?", collectionID, string(entity.Type), entity.ID).
			Delete(&collectionItemPO{})
		if result.Error != nil {
			return result.Error
		}
		changed = result.RowsAffected > 0
		return nil
	})
	return changed, err
}

func (r *PostgresCollectionRepository) ListCollectionItems(ctx context.Context, userID, collectionID int64, entityType domain.EntityType, limit, offset int) ([]*domain.CollectionItem, int64, error) {
	if userID <= 0 {
		return nil, 0, domain.ErrInvalidUserID
	}
	if collectionID <= 0 {
		return nil, 0, domain.ErrInvalidCollectionID
	}
	if entityType != "" && !domain.ValidCollectionEntityType(entityType) {
		return nil, 0, domain.ErrInvalidCollectionEntityType
	}
	limit, offset = normalizeCollectionPage(limit, offset)
	var out []*domain.CollectionItem
	var total int64
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := lockOwnedCollection(tx, userID, collectionID); err != nil {
			return err
		}
		query := tx.Model(&collectionItemPO{}).Where("collection_id = ?", collectionID)
		if entityType != "" {
			query = query.Where("entity_type = ?", string(entityType))
		}
		if err := query.Count(&total).Error; err != nil {
			return err
		}
		var rows []collectionItemPO
		if err := query.Order("created_at DESC, id DESC").Limit(limit).Offset(offset).Find(&rows).Error; err != nil {
			return err
		}
		out = make([]*domain.CollectionItem, 0, len(rows))
		for i := range rows {
			out = append(out, toCollectionItemEntity(&rows[i]))
		}
		return nil
	})
	return out, total, err
}

func (r *PostgresCollectionRepository) GetCollection(ctx context.Context, collectionID, viewerUserID int64) (*domain.Collection, error) {
	if collectionID <= 0 {
		return nil, domain.ErrInvalidCollectionID
	}
	if viewerUserID < 0 {
		return nil, domain.ErrInvalidUserID
	}
	query := r.db.WithContext(ctx).Model(&collectionPO{}).Where("id = ?", collectionID)
	if viewerUserID > 0 {
		query = query.Where("(is_public OR user_id = ?)", viewerUserID)
	} else {
		query = query.Where("is_public = TRUE")
	}
	var row collectionPO
	err := query.Select(`favorite_collections.*, (SELECT COUNT(*) FROM favorite_collection_items WHERE collection_id = favorite_collections.id) AS item_count`).First(&row).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, domain.ErrCollectionNotFound
	}
	if err != nil {
		return nil, err
	}
	return toCollectionEntity(&row), nil
}

func (r *PostgresCollectionRepository) ListPublicCollectionItems(ctx context.Context, collectionID, viewerUserID int64, limit, offset int) ([]*domain.CollectionItem, int64, error) {
	collection, err := r.GetCollection(ctx, collectionID, viewerUserID)
	if err != nil {
		return nil, 0, err
	}
	limit, offset = normalizeCollectionPage(limit, offset)
	query := r.db.WithContext(ctx).Model(&collectionItemPO{}).Where("collection_id = ?", collection.ID)
	var total int64
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	var rows []collectionItemPO
	if err := query.Order("created_at DESC, id DESC").Limit(limit).Offset(offset).Find(&rows).Error; err != nil {
		return nil, 0, err
	}
	out := make([]*domain.CollectionItem, 0, len(rows))
	for i := range rows {
		out = append(out, toCollectionItemEntity(&rows[i]))
	}
	return out, total, nil
}

func getOwnedCollection(db *gorm.DB, userID, collectionID int64) (*domain.Collection, error) {
	var po collectionPO
	err := db.Model(&collectionPO{}).
		Select(`favorite_collections.*,
  (SELECT COUNT(*) FROM favorite_collection_items WHERE collection_id = favorite_collections.id) AS item_count`).
		Where("id = ? AND user_id = ?", collectionID, userID).First(&po).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, domain.ErrCollectionNotFound
	}
	if err != nil {
		return nil, err
	}
	return toCollectionEntity(&po), nil
}

func lockOwnedCollection(db *gorm.DB, userID, collectionID int64) error {
	var collection collectionPO
	err := db.Model(&collectionPO{}).Clauses(clause.Locking{Strength: "UPDATE"}).
		Select("id").Where("id = ? AND user_id = ?", collectionID, userID).Take(&collection).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return domain.ErrCollectionNotFound
	}
	return err
}

func validateCollectionAccess(userID, collectionID int64, entity domain.EntityRef) error {
	if userID <= 0 {
		return domain.ErrInvalidUserID
	}
	if collectionID <= 0 {
		return domain.ErrInvalidCollectionID
	}
	return entity.ValidateForCollection()
}

func normalizeCollectionPage(limit, offset int) (int, int) {
	if limit <= 0 {
		limit = 20
	}
	if limit > 100 {
		limit = 100
	}
	if offset < 0 {
		offset = 0
	}
	return limit, offset
}

func mapCollectionWriteError(err error) error {
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) && pgErr.Code == "23505" && pgErr.ConstraintName == collectionNameUniqueConstraint {
		return domain.ErrCollectionNameExists
	}
	return err
}

func toCollectionEntity(po *collectionPO) *domain.Collection {
	if po == nil {
		return nil
	}
	return &domain.Collection{
		ID: po.ID, UserID: po.UserID, Name: po.Name, Description: po.Description,
		IsPublic: po.IsPublic, ItemCount: po.ItemCount, CreatedAt: po.CreatedAt, UpdatedAt: po.UpdatedAt,
	}
}

func toCollectionItemEntity(po *collectionItemPO) *domain.CollectionItem {
	if po == nil {
		return nil
	}
	return &domain.CollectionItem{
		ID: po.ID, CollectionID: po.CollectionID,
		Entity: domain.EntityRef{Type: domain.EntityType(po.EntityType), ID: po.EntityID}, CreatedAt: po.CreatedAt,
	}
}

var _ domain.CollectionRepository = (*PostgresCollectionRepository)(nil)
