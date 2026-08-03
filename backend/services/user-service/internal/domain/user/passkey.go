package user

import (
	"context"
	"time"
)

const (
	PasskeyMaxCredentials       = 20
	PasskeyMaxChallengeAttempts = 5
	PasskeyNameMaxRunes         = 30

	PasskeyCeremonyRegistration = "registration"
	PasskeyCeremonyMFA          = "mfa"
	PasskeyCeremonyPasswordless = "passwordless"
)

type PasskeyCredential struct {
	CredentialID         string
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

type PasskeyState struct {
	UserID              int64
	PasswordlessEnabled bool
	Credentials         []PasskeyCredential
}

type PasskeyChallenge struct {
	TokenHash         string
	Ceremony          string
	UserID            int64
	MFATokenHash      string
	PasskeyName       string
	SessionCiphertext string
	ExpiresAt         time.Time
	Attempts          int
	UsedAt            *time.Time
	CreatedAt         time.Time
}

type PasskeyUser struct {
	ID          int64
	Username    string
	DisplayName string
	Credentials []PasskeyCredential
}

type PasskeyCeremony struct {
	OptionsJSON       string
	SessionCiphertext string
	ExpiresAt         time.Time
}

type PasskeyRepository interface {
	GetPasskeyState(ctx context.Context, userID int64) (PasskeyState, error)
	GetPasskeyByCredentialID(ctx context.Context, credentialID string) (PasskeyCredential, error)
	CreatePasskeyChallenge(ctx context.Context, challenge PasskeyChallenge) error
	GetPasskeyChallenge(ctx context.Context, tokenHash string, now time.Time) (PasskeyChallenge, error)
	RecordPasskeyChallengeFailure(ctx context.Context, tokenHash string, now time.Time) error
	CreatePasskeyFromChallenge(ctx context.Context, tokenHash string, userID int64, credential PasskeyCredential, now time.Time) error
	UpdatePasskeyName(ctx context.Context, userID int64, credentialID string, name string, now time.Time) (PasskeyCredential, error)
	DeletePasskey(ctx context.Context, userID int64, credentialID string, now time.Time) error
	SetPasskeyPasswordless(ctx context.Context, userID int64, enabled bool, now time.Time) error
	CompletePasskeyMFALogin(ctx context.Context, tokenHash string, mfaTokenHash string, userID int64, credential PasskeyCredential, expectedVersion int64, now time.Time) error
	CompletePasskeyPasswordlessLogin(ctx context.Context, tokenHash string, userID int64, credential PasskeyCredential, expectedVersion int64, now time.Time) error
}
