package persistence

import (
	"context"
	"errors"
	"strings"
	"time"

	domain "user-service/internal/domain/user"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type mfaTOTPPO struct {
	UserID                  int64 `gorm:"primaryKey"`
	SecretCiphertext        string
	PendingSecretCiphertext string
	EnabledAt               *time.Time
	LastTOTPStep            int64
	UpdatedAt               time.Time
}

func (mfaTOTPPO) TableName() string { return "user_mfa_totp" }

type mfaRecoveryCodePO struct {
	CodeHash  string `gorm:"primaryKey;size:64"`
	UserID    int64
	UsedAt    *time.Time
	CreatedAt time.Time
}

func (mfaRecoveryCodePO) TableName() string { return "user_mfa_recovery_codes" }

type mfaLoginChallengePO struct {
	TokenHash string `gorm:"primaryKey;size:64"`
	UserID    int64
	ExpiresAt time.Time
	Attempts  int
	UsedAt    *time.Time
	CreatedAt time.Time
}

func (mfaLoginChallengePO) TableName() string { return "user_mfa_login_challenges" }

var _ domain.MFARepository = (*Repo)(nil)

func (r *Repo) GetMFAState(ctx context.Context, userID int64) (domain.MFAState, error) {
	if userID <= 0 {
		return domain.MFAState{}, domain.ErrInvalidID
	}
	var row mfaTOTPPO
	if err := r.db.WithContext(ctx).First(&row, "user_id = ?", userID).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return domain.MFAState{}, domain.ErrNotFound
		}
		return domain.MFAState{}, err
	}
	var remaining int64
	if err := r.db.WithContext(ctx).Model(&mfaRecoveryCodePO{}).
		Where("user_id = ? AND used_at IS NULL", userID).
		Count(&remaining).Error; err != nil {
		return domain.MFAState{}, err
	}
	return domain.MFAState{
		UserID:                  row.UserID,
		SecretCiphertext:        row.SecretCiphertext,
		PendingSecretCiphertext: row.PendingSecretCiphertext,
		EnabledAt:               row.EnabledAt,
		LastTOTPStep:            row.LastTOTPStep,
		RecoveryCodesRemaining:  remaining,
		UpdatedAt:               row.UpdatedAt,
	}, nil
}

func (r *Repo) SavePendingTOTP(ctx context.Context, userID int64, secretCiphertext string, now time.Time) error {
	secretCiphertext = strings.TrimSpace(secretCiphertext)
	if userID <= 0 {
		return domain.ErrInvalidID
	}
	if secretCiphertext == "" {
		return domain.ErrMFAEncryptionUnavailable
	}
	row := mfaTOTPPO{
		UserID:                  userID,
		PendingSecretCiphertext: secretCiphertext,
		LastTOTPStep:            -1,
		UpdatedAt:               now,
	}
	return r.db.WithContext(ctx).Clauses(clause.OnConflict{
		Columns: []clause.Column{{Name: "user_id"}},
		DoUpdates: clause.Assignments(map[string]any{
			"pending_secret_ciphertext": secretCiphertext,
			"updated_at":                now,
		}),
	}).Create(&row).Error
}

func (r *Repo) EnableTOTP(ctx context.Context, userID int64, expectedPendingCiphertext string, recoveryCodeHashes []string, now time.Time) error {
	if userID <= 0 {
		return domain.ErrInvalidID
	}
	expectedPendingCiphertext = strings.TrimSpace(expectedPendingCiphertext)
	if expectedPendingCiphertext == "" || len(recoveryCodeHashes) == 0 {
		return domain.ErrMFAEnrollmentNotStarted
	}
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		res := tx.Model(&mfaTOTPPO{}).
			Where("user_id = ? AND pending_secret_ciphertext = ?", userID, expectedPendingCiphertext).
			Updates(map[string]any{
				"secret_ciphertext":         gorm.Expr("pending_secret_ciphertext"),
				"pending_secret_ciphertext": "",
				"enabled_at":                now,
				"last_totp_step":            -1,
				"updated_at":                now,
			})
		if res.Error != nil {
			return res.Error
		}
		if res.RowsAffected != 1 {
			return domain.ErrMFAEnrollmentNotStarted
		}
		if err := tx.Where("user_id = ?", userID).Delete(&mfaRecoveryCodePO{}).Error; err != nil {
			return err
		}
		return createMFARecoveryCodes(tx, userID, recoveryCodeHashes, now)
	})
}

func (r *Repo) ReplaceMFARecoveryCodes(ctx context.Context, userID int64, recoveryCodeHashes []string, now time.Time) error {
	if userID <= 0 {
		return domain.ErrInvalidID
	}
	if len(recoveryCodeHashes) == 0 {
		return domain.ErrMFACodeInvalid
	}
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var state mfaTOTPPO
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
			Where("user_id = ? AND enabled_at IS NOT NULL AND secret_ciphertext <> ''", userID).
			First(&state).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return domain.ErrMFANotEnabled
			}
			return err
		}
		if err := tx.Where("user_id = ?", userID).Delete(&mfaRecoveryCodePO{}).Error; err != nil {
			return err
		}
		return createMFARecoveryCodes(tx, userID, recoveryCodeHashes, now)
	})
}

func createMFARecoveryCodes(tx *gorm.DB, userID int64, hashes []string, now time.Time) error {
	rows := make([]mfaRecoveryCodePO, 0, len(hashes))
	seen := make(map[string]struct{}, len(hashes))
	for _, hash := range hashes {
		hash = strings.TrimSpace(hash)
		if len(hash) != 64 {
			return domain.ErrMFACodeInvalid
		}
		if _, exists := seen[hash]; exists {
			return domain.ErrMFACodeInvalid
		}
		seen[hash] = struct{}{}
		rows = append(rows, mfaRecoveryCodePO{CodeHash: hash, UserID: userID, CreatedAt: now})
	}
	return tx.Create(&rows).Error
}

func (r *Repo) DisableTOTP(ctx context.Context, userID int64) error {
	if userID <= 0 {
		return domain.ErrInvalidID
	}
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		res := tx.Where("user_id = ? AND enabled_at IS NOT NULL", userID).Delete(&mfaTOTPPO{})
		if res.Error != nil {
			return res.Error
		}
		if res.RowsAffected != 1 {
			return domain.ErrMFANotEnabled
		}
		return tx.Where("user_id = ? AND used_at IS NULL", userID).Delete(&mfaLoginChallengePO{}).Error
	})
}

func (r *Repo) UseTOTPStep(ctx context.Context, userID int64, step int64, now time.Time) error {
	res := r.db.WithContext(ctx).Model(&mfaTOTPPO{}).
		Where("user_id = ? AND enabled_at IS NOT NULL AND last_totp_step < ?", userID, step).
		Updates(map[string]any{"last_totp_step": step, "updated_at": now})
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected != 1 {
		return domain.ErrMFACodeReplayed
	}
	return nil
}

func (r *Repo) UseMFARecoveryCode(ctx context.Context, userID int64, codeHash string, now time.Time) error {
	res := r.db.WithContext(ctx).Model(&mfaRecoveryCodePO{}).
		Where("user_id = ? AND code_hash = ? AND used_at IS NULL", userID, strings.TrimSpace(codeHash)).
		Update("used_at", now)
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected != 1 {
		return domain.ErrMFACodeInvalid
	}
	return nil
}

func (r *Repo) CreateMFALoginChallenge(ctx context.Context, challenge domain.MFALoginChallenge) error {
	if challenge.UserID <= 0 || strings.TrimSpace(challenge.TokenHash) == "" || !challenge.ExpiresAt.After(challenge.CreatedAt) {
		return domain.ErrMFAChallengeInvalid
	}
	row := mfaLoginChallengePO{
		TokenHash: challenge.TokenHash,
		UserID:    challenge.UserID,
		ExpiresAt: challenge.ExpiresAt,
		CreatedAt: challenge.CreatedAt,
	}
	return r.db.WithContext(ctx).Create(&row).Error
}

func (r *Repo) GetMFALoginChallenge(ctx context.Context, tokenHash string, now time.Time) (domain.MFALoginChallenge, error) {
	var row mfaLoginChallengePO
	if err := r.db.WithContext(ctx).First(&row, "token_hash = ?", strings.TrimSpace(tokenHash)).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return domain.MFALoginChallenge{}, domain.ErrMFAChallengeInvalid
		}
		return domain.MFALoginChallenge{}, err
	}
	if err := validateMFAChallenge(&row, now); err != nil {
		return domain.MFALoginChallenge{}, err
	}
	return toMFALoginChallenge(row), nil
}

func (r *Repo) RecordMFALoginFailure(ctx context.Context, tokenHash string, now time.Time) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		row, err := lockMFAChallenge(tx, tokenHash, now)
		if err != nil {
			return err
		}
		row.Attempts++
		updates := map[string]any{"attempts": row.Attempts}
		if row.Attempts >= domain.MFAMaxLoginAttempts {
			updates["used_at"] = now
		}
		return tx.Model(&mfaLoginChallengePO{}).Where("token_hash = ?", row.TokenHash).Updates(updates).Error
	})
}

func (r *Repo) CompleteMFALoginWithTOTP(ctx context.Context, tokenHash string, userID int64, step int64, now time.Time) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		challenge, err := lockMFAChallenge(tx, tokenHash, now)
		if err != nil {
			return err
		}
		if challenge.UserID != userID {
			return domain.ErrMFAChallengeInvalid
		}
		res := tx.Model(&mfaTOTPPO{}).
			Where("user_id = ? AND enabled_at IS NOT NULL AND last_totp_step < ?", userID, step).
			Updates(map[string]any{"last_totp_step": step, "updated_at": now})
		if res.Error != nil {
			return res.Error
		}
		if res.RowsAffected != 1 {
			return domain.ErrMFACodeReplayed
		}
		return tx.Model(&mfaLoginChallengePO{}).Where("token_hash = ?", challenge.TokenHash).Update("used_at", now).Error
	})
}

func (r *Repo) CompleteMFALoginWithRecoveryCode(ctx context.Context, tokenHash string, userID int64, codeHash string, now time.Time) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		challenge, err := lockMFAChallenge(tx, tokenHash, now)
		if err != nil {
			return err
		}
		if challenge.UserID != userID {
			return domain.ErrMFAChallengeInvalid
		}
		res := tx.Model(&mfaRecoveryCodePO{}).
			Where("user_id = ? AND code_hash = ? AND used_at IS NULL", userID, strings.TrimSpace(codeHash)).
			Update("used_at", now)
		if res.Error != nil {
			return res.Error
		}
		if res.RowsAffected != 1 {
			return domain.ErrMFACodeInvalid
		}
		return tx.Model(&mfaLoginChallengePO{}).Where("token_hash = ?", challenge.TokenHash).Update("used_at", now).Error
	})
}

func lockMFAChallenge(tx *gorm.DB, tokenHash string, now time.Time) (*mfaLoginChallengePO, error) {
	var row mfaLoginChallengePO
	if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).First(&row, "token_hash = ?", strings.TrimSpace(tokenHash)).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, domain.ErrMFAChallengeInvalid
		}
		return nil, err
	}
	if err := validateMFAChallenge(&row, now); err != nil {
		return nil, err
	}
	return &row, nil
}

func validateMFAChallenge(row *mfaLoginChallengePO, now time.Time) error {
	if row == nil || row.UserID <= 0 {
		return domain.ErrMFAChallengeInvalid
	}
	if !row.ExpiresAt.After(now) {
		return domain.ErrMFAChallengeExpired
	}
	if row.Attempts >= domain.MFAMaxLoginAttempts {
		return domain.ErrMFAChallengeAttemptsExceeded
	}
	if row.UsedAt != nil {
		return domain.ErrMFAChallengeInvalid
	}
	return nil
}

func toMFALoginChallenge(row mfaLoginChallengePO) domain.MFALoginChallenge {
	return domain.MFALoginChallenge{
		TokenHash: row.TokenHash,
		UserID:    row.UserID,
		ExpiresAt: row.ExpiresAt,
		Attempts:  row.Attempts,
		UsedAt:    row.UsedAt,
		CreatedAt: row.CreatedAt,
	}
}
