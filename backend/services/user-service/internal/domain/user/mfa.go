package user

import (
	"context"
	"time"
)

const MFAMaxLoginAttempts = 5

type MFAState struct {
	UserID                  int64
	SecretCiphertext        string
	PendingSecretCiphertext string
	EnabledAt               *time.Time
	LastTOTPStep            int64
	RecoveryCodesRemaining  int64
	UpdatedAt               time.Time
}

func (s MFAState) Enabled() bool {
	return s.EnabledAt != nil && s.SecretCiphertext != ""
}

type MFALoginChallenge struct {
	TokenHash string
	UserID    int64
	ExpiresAt time.Time
	Attempts  int
	UsedAt    *time.Time
	CreatedAt time.Time
}

type TOTPEnrollment struct {
	Secret    string
	URL       string
	QRDataURL string
	Issuer    string
	Account   string
}

type MFARepository interface {
	GetMFAState(ctx context.Context, userID int64) (MFAState, error)
	SavePendingTOTP(ctx context.Context, userID int64, secretCiphertext string, now time.Time) error
	EnableTOTP(ctx context.Context, userID int64, expectedPendingCiphertext string, recoveryCodeHashes []string, now time.Time) error
	ReplaceMFARecoveryCodes(ctx context.Context, userID int64, recoveryCodeHashes []string, now time.Time) error
	DisableTOTP(ctx context.Context, userID int64) error
	UseTOTPStep(ctx context.Context, userID int64, step int64, now time.Time) error
	UseMFARecoveryCode(ctx context.Context, userID int64, codeHash string, now time.Time) error
	CreateMFALoginChallenge(ctx context.Context, challenge MFALoginChallenge) error
	GetMFALoginChallenge(ctx context.Context, tokenHash string, now time.Time) (MFALoginChallenge, error)
	RecordMFALoginFailure(ctx context.Context, tokenHash string, now time.Time) error
	CompleteMFALoginWithTOTP(ctx context.Context, tokenHash string, userID int64, step int64, now time.Time) error
	CompleteMFALoginWithRecoveryCode(ctx context.Context, tokenHash string, userID int64, codeHash string, now time.Time) error
}
