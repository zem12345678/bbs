package persistence

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	domain "user-service/internal/domain/user"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type antennaPO struct {
	ID                             int64     `gorm:"primaryKey"`
	OwnerID                        int64     `gorm:"not null;index"`
	Name                           string    `gorm:"size:100;not null"`
	Source                         string    `gorm:"column:source;size:32;not null"`
	UserListID                     *int64    `gorm:"column:user_list_id"`
	Keywords                       []byte    `gorm:"type:jsonb;not null"`
	ExcludeKeywords                []byte    `gorm:"type:jsonb;not null"`
	Users                          []byte    `gorm:"type:jsonb;not null"`
	CaseSensitive                  bool      `gorm:"not null"`
	LocalOnly                      bool      `gorm:"not null"`
	ExcludeBots                    bool      `gorm:"not null"`
	WithReplies                    bool      `gorm:"not null"`
	WithFile                       bool      `gorm:"not null"`
	ExcludeNotesInSensitiveChannel bool      `gorm:"not null"`
	IsActive                       bool      `gorm:"not null"`
	CreatedAt                      time.Time `gorm:"not null"`
	UpdatedAt                      time.Time `gorm:"not null"`
	LastUsedAt                     time.Time `gorm:"not null"`
}

func (antennaPO) TableName() string { return "antennas" }

var _ domain.AntennaRepository = (*Repo)(nil)

func (r *Repo) CreateAntenna(ctx context.Context, antenna *domain.Antenna) error {
	if err := antenna.Validate(); err != nil {
		return err
	}
	row, err := toAntennaPO(antenna)
	if err != nil {
		return err
	}
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := lockAntennaOwner(tx, antenna.OwnerID); err != nil {
			return err
		}
		if err := ensureUserExists(tx, antenna.OwnerID); err != nil {
			return err
		}
		if err := ensureAntennaUserListVisible(tx, antenna.OwnerID, antenna.UserListID); err != nil {
			return err
		}
		var count int64
		if err := tx.Model(&antennaPO{}).Where("owner_id = ?", antenna.OwnerID).Count(&count).Error; err != nil {
			return err
		}
		if count >= domain.MaxAntennasPerOwner {
			return domain.ErrAntennaLimitReached
		}
		return tx.Create(row).Error
	})
}

func (r *Repo) UpdateAntenna(ctx context.Context, antenna *domain.Antenna) error {
	if err := antenna.Validate(); err != nil {
		return err
	}
	row, err := toAntennaPO(antenna)
	if err != nil {
		return err
	}
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var existing antennaPO
		err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
			Where("id = ? AND owner_id = ?", antenna.ID, antenna.OwnerID).Take(&existing).Error
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return domain.ErrAntennaNotFound
		}
		if err != nil {
			return err
		}
		if err := ensureAntennaUserListVisible(tx, antenna.OwnerID, antenna.UserListID); err != nil {
			return err
		}
		return tx.Model(&antennaPO{}).Where("id = ? AND owner_id = ?", antenna.ID, antenna.OwnerID).Updates(map[string]any{
			"name": row.Name, "source": row.Source, "user_list_id": row.UserListID,
			"keywords": row.Keywords, "exclude_keywords": row.ExcludeKeywords, "users": row.Users,
			"case_sensitive": row.CaseSensitive, "local_only": row.LocalOnly, "exclude_bots": row.ExcludeBots,
			"with_replies": row.WithReplies, "with_file": row.WithFile,
			"exclude_notes_in_sensitive_channel": row.ExcludeNotesInSensitiveChannel,
			"is_active":                          true, "updated_at": row.UpdatedAt, "last_used_at": row.LastUsedAt,
		}).Error
	})
}

func (r *Repo) DeleteAntenna(ctx context.Context, ownerID, antennaID int64) error {
	if ownerID <= 0 || antennaID <= 0 {
		return domain.ErrInvalidID
	}
	result := r.db.WithContext(ctx).Where("id = ? AND owner_id = ?", antennaID, ownerID).Delete(&antennaPO{})
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return domain.ErrAntennaNotFound
	}
	return nil
}

func (r *Repo) GetAntenna(ctx context.Context, ownerID, antennaID int64) (*domain.Antenna, error) {
	if ownerID <= 0 || antennaID <= 0 {
		return nil, domain.ErrInvalidID
	}
	var row antennaPO
	err := r.db.WithContext(ctx).Where("id = ? AND owner_id = ?", antennaID, ownerID).Take(&row).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, domain.ErrAntennaNotFound
	}
	if err != nil {
		return nil, err
	}
	return toAntenna(&row)
}

func (r *Repo) ListAntennas(ctx context.Context, ownerID int64) ([]*domain.Antenna, error) {
	if ownerID <= 0 {
		return nil, domain.ErrInvalidID
	}
	var rows []antennaPO
	if err := r.db.WithContext(ctx).Where("owner_id = ?", ownerID).Order("created_at DESC, id DESC").Find(&rows).Error; err != nil {
		return nil, err
	}
	items := make([]*domain.Antenna, 0, len(rows))
	for index := range rows {
		item, err := toAntenna(&rows[index])
		if err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, nil
}

func toAntennaPO(antenna *domain.Antenna) (*antennaPO, error) {
	keywords, err := json.Marshal(antenna.Keywords)
	if err != nil {
		return nil, err
	}
	excluded, err := json.Marshal(antenna.ExcludeKeywords)
	if err != nil {
		return nil, err
	}
	users, err := json.Marshal(antenna.Users)
	if err != nil {
		return nil, err
	}
	var listID *int64
	if antenna.UserListID > 0 {
		value := antenna.UserListID
		listID = &value
	}
	return &antennaPO{
		ID: antenna.ID, OwnerID: antenna.OwnerID, Name: antenna.Name, Source: antenna.Source, UserListID: listID,
		Keywords: keywords, ExcludeKeywords: excluded, Users: users, CaseSensitive: antenna.CaseSensitive,
		LocalOnly: antenna.LocalOnly, ExcludeBots: antenna.ExcludeBots, WithReplies: antenna.WithReplies,
		WithFile: antenna.WithFile, ExcludeNotesInSensitiveChannel: antenna.ExcludeNotesInSensitiveChannel,
		IsActive: antenna.IsActive, CreatedAt: antenna.CreatedAt, UpdatedAt: antenna.UpdatedAt, LastUsedAt: antenna.LastUsedAt,
	}, nil
}

func toAntenna(row *antennaPO) (*domain.Antenna, error) {
	var keywords, excluded [][]string
	var users []string
	if err := json.Unmarshal(row.Keywords, &keywords); err != nil {
		return nil, err
	}
	if err := json.Unmarshal(row.ExcludeKeywords, &excluded); err != nil {
		return nil, err
	}
	if err := json.Unmarshal(row.Users, &users); err != nil {
		return nil, err
	}
	var listID int64
	if row.UserListID != nil {
		listID = *row.UserListID
	}
	return &domain.Antenna{
		ID: row.ID, OwnerID: row.OwnerID, Name: row.Name, Source: row.Source, UserListID: listID,
		Keywords: keywords, ExcludeKeywords: excluded, Users: users, CaseSensitive: row.CaseSensitive,
		LocalOnly: row.LocalOnly, ExcludeBots: row.ExcludeBots, WithReplies: row.WithReplies,
		WithFile: row.WithFile, ExcludeNotesInSensitiveChannel: row.ExcludeNotesInSensitiveChannel,
		IsActive: row.IsActive, CreatedAt: row.CreatedAt, UpdatedAt: row.UpdatedAt, LastUsedAt: row.LastUsedAt,
	}, nil
}

func lockAntennaOwner(tx *gorm.DB, ownerID int64) error {
	return tx.Exec("SELECT pg_advisory_xact_lock(hashtextextended(?, 0))", fmt.Sprintf("antenna-owner:%d", ownerID)).Error
}

func ensureAntennaUserListVisible(tx *gorm.DB, ownerID, listID int64) error {
	if listID == 0 {
		return nil
	}
	var count int64
	if err := tx.Model(&userListPO{}).Where("id = ? AND (owner_id = ? OR is_public = TRUE)", listID, ownerID).Count(&count).Error; err != nil {
		return err
	}
	if count == 0 {
		return domain.ErrUserListNotFound
	}
	return nil
}
