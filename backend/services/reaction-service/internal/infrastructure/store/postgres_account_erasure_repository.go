package store

import (
	"context"
	"errors"
	"time"

	accountDomain "reaction-service/internal/domain/account"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type reactionErasedUserPO struct {
	UserID                   int64 `gorm:"primaryKey"`
	DeletionJobID            int64
	PolicyVersion            int32
	DeletedLikes             int64
	DeletedFavorites         int64
	DeletedCollections       int64
	AnonymizedReports        int64
	AnonymizedHandledReports int64
	ErasedAt                 time.Time
}

func (reactionErasedUserPO) TableName() string { return "reaction_erased_users" }

type AccountErasureRepository struct {
	db *gorm.DB
}

func NewAccountErasureRepository(db *gorm.DB) *AccountErasureRepository {
	return &AccountErasureRepository{db: db}
}

func (r *AccountErasureRepository) EnsureSchema(ctx context.Context) error {
	if r == nil || r.db == nil {
		return accountDomain.ErrInvalidErasure
	}
	statements := []string{
		`CREATE TABLE IF NOT EXISTS reaction_erased_users (
  user_id BIGINT PRIMARY KEY,
  deletion_job_id BIGINT NOT NULL,
  policy_version INTEGER NOT NULL,
  deleted_likes BIGINT NOT NULL DEFAULT 0,
  deleted_favorites BIGINT NOT NULL DEFAULT 0,
  deleted_collections BIGINT NOT NULL DEFAULT 0,
  anonymized_reports BIGINT NOT NULL DEFAULT 0,
  anonymized_handled_reports BIGINT NOT NULL DEFAULT 0,
  erased_at TIMESTAMPTZ NOT NULL DEFAULT now()
)`,
		`CREATE INDEX IF NOT EXISTS idx_reaction_erased_users_job ON reaction_erased_users(deletion_job_id)`,
	}
	for _, statement := range statements {
		if err := r.db.WithContext(ctx).Exec(statement).Error; err != nil {
			return err
		}
	}
	return nil
}

func (r *AccountErasureRepository) EraseAccountReactions(ctx context.Context, userID, deletionJobID int64, policyVersion int32) (accountDomain.ErasureResult, error) {
	if r == nil || r.db == nil || userID <= 0 || deletionJobID <= 0 || policyVersion <= 0 {
		return accountDomain.ErasureResult{}, accountDomain.ErrInvalidErasure
	}

	var result accountDomain.ErasureResult
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := lockReactionUser(tx, userID); err != nil {
			return err
		}

		var receipt reactionErasedUserPO
		err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).First(&receipt, "user_id = ?", userID).Error
		switch {
		case err == nil && policyVersion <= receipt.PolicyVersion:
			result = accountErasureResult(receipt)
			return nil
		case err == nil:
			result = accountErasureResult(receipt)
			if err := tx.Model(&reactionErasedUserPO{}).Where("user_id = ?", userID).Updates(map[string]any{
				"deletion_job_id": deletionJobID,
				"policy_version":  policyVersion,
			}).Error; err != nil {
				return err
			}
		case errors.Is(err, gorm.ErrRecordNotFound):
			receipt = reactionErasedUserPO{
				UserID: userID, DeletionJobID: deletionJobID, PolicyVersion: policyVersion, ErasedAt: time.Now().UTC(),
			}
			if err := tx.Create(&receipt).Error; err != nil {
				return err
			}
		default:
			return err
		}

		deletedLikes := tx.Where("user_id = ?", userID).Delete(&likePO{})
		if deletedLikes.Error != nil {
			return deletedLikes.Error
		}
		deletedFavorites := tx.Where("user_id = ?", userID).Delete(&favoritePO{})
		if deletedFavorites.Error != nil {
			return deletedFavorites.Error
		}
		deletedPins := tx.Where("user_id = ?", userID).Delete(&pinPO{})
		if deletedPins.Error != nil {
			return deletedPins.Error
		}
		deletedCollections := tx.Where("user_id = ?", userID).Delete(&collectionPO{})
		if deletedCollections.Error != nil {
			return deletedCollections.Error
		}

		now := time.Now().UTC()
		anonymizedReports := tx.Model(&reportPO{}).Where("reporter_id = ?", userID).Updates(map[string]any{
			"reporter_id": int64(0),
			"updated_at":  now,
		})
		if anonymizedReports.Error != nil {
			return anonymizedReports.Error
		}
		anonymizedHandledReports := tx.Model(&reportPO{}).Where("handled_by = ?", userID).Updates(map[string]any{
			"handled_by": int64(0),
			"updated_at": now,
		})
		if anonymizedHandledReports.Error != nil {
			return anonymizedHandledReports.Error
		}

		result.DeletedLikes += deletedLikes.RowsAffected
		result.DeletedFavorites += deletedFavorites.RowsAffected
		result.DeletedCollections += deletedCollections.RowsAffected
		result.AnonymizedReports += anonymizedReports.RowsAffected
		result.AnonymizedHandledReports += anonymizedHandledReports.RowsAffected
		return tx.Model(&reactionErasedUserPO{}).Where("user_id = ?", userID).Updates(map[string]any{
			"deleted_likes":              result.DeletedLikes,
			"deleted_favorites":          result.DeletedFavorites,
			"deleted_collections":        result.DeletedCollections,
			"anonymized_reports":         result.AnonymizedReports,
			"anonymized_handled_reports": result.AnonymizedHandledReports,
		}).Error
	})
	return result, err
}

func accountErasureResult(receipt reactionErasedUserPO) accountDomain.ErasureResult {
	return accountDomain.ErasureResult{
		DeletedLikes:             receipt.DeletedLikes,
		DeletedFavorites:         receipt.DeletedFavorites,
		DeletedCollections:       receipt.DeletedCollections,
		AnonymizedReports:        receipt.AnonymizedReports,
		AnonymizedHandledReports: receipt.AnonymizedHandledReports,
	}
}

func lockReactionUser(tx *gorm.DB, userID int64) error {
	return tx.Exec(`SELECT pg_advisory_xact_lock(hashtextextended('bbs-reaction-user:' || CAST(CAST(? AS BIGINT) AS TEXT), 0))`, userID).Error
}

func ensureReactionUserActive(tx *gorm.DB, userID int64) error {
	if err := lockReactionUser(tx, userID); err != nil {
		return err
	}
	var erased bool
	if err := tx.Raw(`SELECT EXISTS (SELECT 1 FROM reaction_erased_users WHERE user_id = CAST(? AS BIGINT))`, userID).Scan(&erased).Error; err != nil {
		return err
	}
	if erased {
		return accountDomain.ErrUserErased
	}
	return nil
}

var _ accountDomain.ErasureRepository = (*AccountErasureRepository)(nil)
