package persistence

import (
	"context"
	"errors"
	"strings"
	"time"

	domain "user-service/internal/domain/user"

	"github.com/jackc/pgx/v5/pgconn"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type passkeyCredentialPO struct {
	CredentialID         string `gorm:"primaryKey"`
	UserID               int64
	Name                 string
	CredentialCiphertext string
	Version              int64
	BackupEligible       bool
	BackupState          bool
	CreatedAt            time.Time
	UpdatedAt            time.Time
	LastUsedAt           *time.Time
}

func (passkeyCredentialPO) TableName() string { return "user_passkeys" }

type passkeyChallengePO struct {
	TokenHash         string `gorm:"primaryKey;size:64"`
	Ceremony          string
	UserID            *int64
	MFATokenHash      *string
	PasskeyName       string
	SessionCiphertext string
	ExpiresAt         time.Time
	Attempts          int
	UsedAt            *time.Time
	CreatedAt         time.Time
}

func (passkeyChallengePO) TableName() string { return "user_passkey_challenges" }

var _ domain.PasskeyRepository = (*Repo)(nil)

func (r *Repo) GetPasskeyState(ctx context.Context, userID int64) (domain.PasskeyState, error) {
	if userID <= 0 {
		return domain.PasskeyState{}, domain.ErrInvalidID
	}
	var mfaState mfaTOTPPO
	if err := r.db.WithContext(ctx).
		Where("user_id = ? AND enabled_at IS NOT NULL AND secret_ciphertext <> ''", userID).
		First(&mfaState).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return domain.PasskeyState{}, domain.ErrMFANotEnabled
		}
		return domain.PasskeyState{}, err
	}
	var rows []passkeyCredentialPO
	if err := r.db.WithContext(ctx).
		Where("user_id = ?", userID).
		Order("created_at ASC, credential_id ASC").
		Find(&rows).Error; err != nil {
		return domain.PasskeyState{}, err
	}
	credentials := make([]domain.PasskeyCredential, 0, len(rows))
	for _, row := range rows {
		credentials = append(credentials, toPasskeyCredential(row))
	}
	return domain.PasskeyState{UserID: userID, PasswordlessEnabled: mfaState.PasswordlessEnabled, Credentials: credentials}, nil
}

func (r *Repo) GetPasskeyByCredentialID(ctx context.Context, credentialID string) (domain.PasskeyCredential, error) {
	credentialID = strings.TrimSpace(credentialID)
	if credentialID == "" {
		return domain.PasskeyCredential{}, domain.ErrPasskeyNotFound
	}
	var row passkeyCredentialPO
	err := r.db.WithContext(ctx).
		Table("user_passkeys AS passkey").
		Select("passkey.*").
		Joins("JOIN user_mfa_totp AS mfa ON mfa.user_id = passkey.user_id AND mfa.enabled_at IS NOT NULL AND mfa.secret_ciphertext <> ''").
		Where("passkey.credential_id = ?", credentialID).
		Take(&row).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return domain.PasskeyCredential{}, domain.ErrPasskeyNotFound
	}
	if err != nil {
		return domain.PasskeyCredential{}, err
	}
	return toPasskeyCredential(row), nil
}

func (r *Repo) CreatePasskeyChallenge(ctx context.Context, challenge domain.PasskeyChallenge) error {
	challenge.TokenHash = strings.TrimSpace(challenge.TokenHash)
	challenge.Ceremony = strings.TrimSpace(challenge.Ceremony)
	challenge.MFATokenHash = strings.TrimSpace(challenge.MFATokenHash)
	challenge.PasskeyName = strings.TrimSpace(challenge.PasskeyName)
	challenge.SessionCiphertext = strings.TrimSpace(challenge.SessionCiphertext)
	if len(challenge.TokenHash) != 64 || challenge.SessionCiphertext == "" || !challenge.ExpiresAt.After(challenge.CreatedAt) {
		return domain.ErrPasskeyChallengeInvalid
	}
	row := passkeyChallengePO{
		TokenHash:         challenge.TokenHash,
		Ceremony:          challenge.Ceremony,
		PasskeyName:       challenge.PasskeyName,
		SessionCiphertext: challenge.SessionCiphertext,
		ExpiresAt:         challenge.ExpiresAt,
		CreatedAt:         challenge.CreatedAt,
	}
	switch challenge.Ceremony {
	case domain.PasskeyCeremonyRegistration:
		if challenge.UserID <= 0 || challenge.MFATokenHash != "" || !validPasskeyName(challenge.PasskeyName) {
			return domain.ErrPasskeyChallengeInvalid
		}
		row.UserID = &challenge.UserID
	case domain.PasskeyCeremonyMFA:
		if challenge.UserID <= 0 || len(challenge.MFATokenHash) != 64 || challenge.PasskeyName != "" {
			return domain.ErrPasskeyChallengeInvalid
		}
		row.UserID = &challenge.UserID
		row.MFATokenHash = &challenge.MFATokenHash
	case domain.PasskeyCeremonyPasswordless:
		if challenge.UserID != 0 || challenge.MFATokenHash != "" || challenge.PasskeyName != "" {
			return domain.ErrPasskeyChallengeInvalid
		}
	default:
		return domain.ErrPasskeyChallengeInvalid
	}
	if err := r.db.WithContext(ctx).
		Where("expires_at <= ?", challenge.CreatedAt).
		Delete(&passkeyChallengePO{}).Error; err != nil {
		return err
	}
	return r.db.WithContext(ctx).Create(&row).Error
}

func (r *Repo) GetPasskeyChallenge(ctx context.Context, tokenHash string, now time.Time) (domain.PasskeyChallenge, error) {
	var row passkeyChallengePO
	if err := r.db.WithContext(ctx).First(&row, "token_hash = ?", strings.TrimSpace(tokenHash)).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return domain.PasskeyChallenge{}, domain.ErrPasskeyChallengeInvalid
		}
		return domain.PasskeyChallenge{}, err
	}
	if err := validatePasskeyChallenge(&row, now); err != nil {
		return domain.PasskeyChallenge{}, err
	}
	return toPasskeyChallenge(row), nil
}

func (r *Repo) RecordPasskeyChallengeFailure(ctx context.Context, tokenHash string, now time.Time) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		challenge, err := lockPasskeyChallenge(tx, tokenHash, now)
		if err != nil {
			return err
		}
		challenge.Attempts++
		updates := map[string]any{"attempts": challenge.Attempts}
		if challenge.Attempts >= domain.PasskeyMaxChallengeAttempts {
			updates["used_at"] = now
		}
		if err := tx.Model(&passkeyChallengePO{}).Where("token_hash = ?", challenge.TokenHash).Updates(updates).Error; err != nil {
			return err
		}
		if challenge.MFATokenHash == nil {
			return nil
		}
		mfaChallenge, err := lockMFAChallenge(tx, *challenge.MFATokenHash, now)
		if err != nil {
			return err
		}
		mfaChallenge.Attempts++
		mfaUpdates := map[string]any{"attempts": mfaChallenge.Attempts}
		if mfaChallenge.Attempts >= domain.MFAMaxLoginAttempts {
			mfaUpdates["used_at"] = now
		}
		return tx.Model(&mfaLoginChallengePO{}).Where("token_hash = ?", mfaChallenge.TokenHash).Updates(mfaUpdates).Error
	})
}

func (r *Repo) CreatePasskeyFromChallenge(ctx context.Context, tokenHash string, userID int64, credential domain.PasskeyCredential, now time.Time) error {
	if userID <= 0 || !validStoredPasskey(credential) {
		return domain.ErrPasskeyVerificationFailed
	}
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		challenge, err := lockPasskeyChallenge(tx, tokenHash, now)
		if err != nil {
			return err
		}
		if challenge.Ceremony != domain.PasskeyCeremonyRegistration || challenge.UserID == nil || *challenge.UserID != userID {
			return domain.ErrPasskeyChallengeInvalid
		}
		if _, err := lockEnabledPasskeyMFA(tx, userID); err != nil {
			return err
		}
		var count int64
		if err := tx.Model(&passkeyCredentialPO{}).Where("user_id = ?", userID).Count(&count).Error; err != nil {
			return err
		}
		if count >= domain.PasskeyMaxCredentials {
			return domain.ErrPasskeyLimitReached
		}
		row := passkeyCredentialPO{
			CredentialID:         credential.CredentialID,
			UserID:               userID,
			Name:                 challenge.PasskeyName,
			CredentialCiphertext: credential.CredentialCiphertext,
			Version:              1,
			BackupEligible:       credential.BackupEligible,
			BackupState:          credential.BackupState,
			CreatedAt:            now,
			UpdatedAt:            now,
		}
		if err := tx.Create(&row).Error; err != nil {
			return translatePasskeyPersistenceError(err)
		}
		return tx.Model(&passkeyChallengePO{}).Where("token_hash = ?", challenge.TokenHash).Update("used_at", now).Error
	})
}

func (r *Repo) UpdatePasskeyName(ctx context.Context, userID int64, credentialID string, name string, now time.Time) (domain.PasskeyCredential, error) {
	credentialID = strings.TrimSpace(credentialID)
	name = strings.TrimSpace(name)
	if userID <= 0 {
		return domain.PasskeyCredential{}, domain.ErrInvalidID
	}
	if !validPasskeyName(name) {
		if name == "" {
			return domain.PasskeyCredential{}, domain.ErrPasskeyNameRequired
		}
		return domain.PasskeyCredential{}, domain.ErrPasskeyNameTooLong
	}
	res := r.db.WithContext(ctx).Model(&passkeyCredentialPO{}).
		Where("user_id = ? AND credential_id = ?", userID, credentialID).
		Updates(map[string]any{"name": name, "updated_at": now})
	if res.Error != nil {
		return domain.PasskeyCredential{}, res.Error
	}
	if res.RowsAffected != 1 {
		return domain.PasskeyCredential{}, domain.ErrPasskeyNotFound
	}
	var row passkeyCredentialPO
	if err := r.db.WithContext(ctx).First(&row, "user_id = ? AND credential_id = ?", userID, credentialID).Error; err != nil {
		return domain.PasskeyCredential{}, err
	}
	return toPasskeyCredential(row), nil
}

func (r *Repo) DeletePasskey(ctx context.Context, userID int64, credentialID string, now time.Time) error {
	credentialID = strings.TrimSpace(credentialID)
	if userID <= 0 {
		return domain.ErrInvalidID
	}
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if _, err := lockEnabledPasskeyMFA(tx, userID); err != nil {
			return err
		}
		res := tx.Where("user_id = ? AND credential_id = ?", userID, credentialID).Delete(&passkeyCredentialPO{})
		if res.Error != nil {
			return res.Error
		}
		if res.RowsAffected != 1 {
			return domain.ErrPasskeyNotFound
		}
		var remaining int64
		if err := tx.Model(&passkeyCredentialPO{}).Where("user_id = ?", userID).Count(&remaining).Error; err != nil {
			return err
		}
		if remaining == 0 {
			return tx.Model(&mfaTOTPPO{}).Where("user_id = ?", userID).
				Updates(map[string]any{"passwordless_enabled": false, "updated_at": now}).Error
		}
		return nil
	})
}

func (r *Repo) SetPasskeyPasswordless(ctx context.Context, userID int64, enabled bool, now time.Time) error {
	if userID <= 0 {
		return domain.ErrInvalidID
	}
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if _, err := lockEnabledPasskeyMFA(tx, userID); err != nil {
			return err
		}
		if enabled {
			var count int64
			if err := tx.Model(&passkeyCredentialPO{}).Where("user_id = ?", userID).Count(&count).Error; err != nil {
				return err
			}
			if count == 0 {
				return domain.ErrPasskeyPasswordlessUnavailable
			}
		}
		return tx.Model(&mfaTOTPPO{}).Where("user_id = ?", userID).
			Updates(map[string]any{"passwordless_enabled": enabled, "updated_at": now}).Error
	})
}

func (r *Repo) CompletePasskeyMFALogin(ctx context.Context, tokenHash string, mfaTokenHash string, userID int64, credential domain.PasskeyCredential, expectedVersion int64, now time.Time) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		challenge, err := lockPasskeyChallenge(tx, tokenHash, now)
		if err != nil {
			return err
		}
		if challenge.Ceremony != domain.PasskeyCeremonyMFA || challenge.UserID == nil || *challenge.UserID != userID || challenge.MFATokenHash == nil || *challenge.MFATokenHash != strings.TrimSpace(mfaTokenHash) {
			return domain.ErrPasskeyChallengeInvalid
		}
		mfaChallenge, err := lockMFAChallenge(tx, mfaTokenHash, now)
		if err != nil {
			return err
		}
		if mfaChallenge.UserID != userID {
			return domain.ErrMFAChallengeInvalid
		}
		if err := updatePasskeyCredential(tx, userID, credential, expectedVersion, now); err != nil {
			return err
		}
		if err := tx.Model(&mfaLoginChallengePO{}).Where("token_hash = ?", mfaChallenge.TokenHash).Update("used_at", now).Error; err != nil {
			return err
		}
		return tx.Model(&passkeyChallengePO{}).Where("token_hash = ?", challenge.TokenHash).Update("used_at", now).Error
	})
}

func (r *Repo) CompletePasskeyPasswordlessLogin(ctx context.Context, tokenHash string, userID int64, credential domain.PasskeyCredential, expectedVersion int64, now time.Time) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		challenge, err := lockPasskeyChallenge(tx, tokenHash, now)
		if err != nil {
			return err
		}
		if challenge.Ceremony != domain.PasskeyCeremonyPasswordless || challenge.UserID != nil || challenge.MFATokenHash != nil {
			return domain.ErrPasskeyChallengeInvalid
		}
		mfaState, err := lockEnabledPasskeyMFA(tx, userID)
		if err != nil {
			return err
		}
		if !mfaState.PasswordlessEnabled {
			return domain.ErrPasskeyPasswordlessUnavailable
		}
		if err := updatePasskeyCredential(tx, userID, credential, expectedVersion, now); err != nil {
			return err
		}
		return tx.Model(&passkeyChallengePO{}).Where("token_hash = ?", challenge.TokenHash).Update("used_at", now).Error
	})
}

func updatePasskeyCredential(tx *gorm.DB, userID int64, credential domain.PasskeyCredential, expectedVersion int64, now time.Time) error {
	if userID <= 0 || expectedVersion <= 0 || !validStoredPasskey(credential) {
		return domain.ErrPasskeyVerificationFailed
	}
	res := tx.Model(&passkeyCredentialPO{}).
		Where("user_id = ? AND credential_id = ? AND version = ?", userID, credential.CredentialID, expectedVersion).
		Updates(map[string]any{
			"credential_ciphertext": credential.CredentialCiphertext,
			"version":               expectedVersion + 1,
			"backup_eligible":       credential.BackupEligible,
			"backup_state":          credential.BackupState,
			"updated_at":            now,
			"last_used_at":          now,
		})
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected != 1 {
		return domain.ErrPasskeyCredentialChanged
	}
	return nil
}

func lockEnabledPasskeyMFA(tx *gorm.DB, userID int64) (*mfaTOTPPO, error) {
	var row mfaTOTPPO
	err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
		Where("user_id = ? AND enabled_at IS NOT NULL AND secret_ciphertext <> ''", userID).
		First(&row).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, domain.ErrMFANotEnabled
	}
	return &row, err
}

func lockPasskeyChallenge(tx *gorm.DB, tokenHash string, now time.Time) (*passkeyChallengePO, error) {
	var row passkeyChallengePO
	err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).First(&row, "token_hash = ?", strings.TrimSpace(tokenHash)).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, domain.ErrPasskeyChallengeInvalid
	}
	if err != nil {
		return nil, err
	}
	if err := validatePasskeyChallenge(&row, now); err != nil {
		return nil, err
	}
	return &row, nil
}

func validatePasskeyChallenge(row *passkeyChallengePO, now time.Time) error {
	if row == nil || strings.TrimSpace(row.TokenHash) == "" || strings.TrimSpace(row.SessionCiphertext) == "" {
		return domain.ErrPasskeyChallengeInvalid
	}
	if !row.ExpiresAt.After(now) {
		return domain.ErrPasskeyChallengeExpired
	}
	if row.Attempts >= domain.PasskeyMaxChallengeAttempts {
		return domain.ErrPasskeyChallengeAttemptsExceeded
	}
	if row.UsedAt != nil {
		return domain.ErrPasskeyChallengeInvalid
	}
	return nil
}

func validPasskeyName(name string) bool {
	length := len([]rune(strings.TrimSpace(name)))
	return length >= 1 && length <= domain.PasskeyNameMaxRunes
}

func validStoredPasskey(credential domain.PasskeyCredential) bool {
	return strings.TrimSpace(credential.CredentialID) != "" && strings.TrimSpace(credential.CredentialCiphertext) != ""
}

func translatePasskeyPersistenceError(err error) error {
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) && pgErr.Code == "23505" {
		return domain.ErrPasskeyCredentialExists
	}
	return err
}

func toPasskeyCredential(row passkeyCredentialPO) domain.PasskeyCredential {
	return domain.PasskeyCredential{
		CredentialID:         row.CredentialID,
		UserID:               row.UserID,
		Name:                 row.Name,
		CredentialCiphertext: row.CredentialCiphertext,
		Version:              row.Version,
		BackupEligible:       row.BackupEligible,
		BackupState:          row.BackupState,
		CreatedAt:            row.CreatedAt,
		UpdatedAt:            row.UpdatedAt,
		LastUsedAt:           row.LastUsedAt,
	}
}

func toPasskeyChallenge(row passkeyChallengePO) domain.PasskeyChallenge {
	challenge := domain.PasskeyChallenge{
		TokenHash:         strings.TrimSpace(row.TokenHash),
		Ceremony:          row.Ceremony,
		PasskeyName:       row.PasskeyName,
		SessionCiphertext: row.SessionCiphertext,
		ExpiresAt:         row.ExpiresAt,
		Attempts:          row.Attempts,
		UsedAt:            row.UsedAt,
		CreatedAt:         row.CreatedAt,
	}
	if row.UserID != nil {
		challenge.UserID = *row.UserID
	}
	if row.MFATokenHash != nil {
		challenge.MFATokenHash = strings.TrimSpace(*row.MFATokenHash)
	}
	return challenge
}
