package user

import (
	"net/mail"
	"strings"
	"time"
)

const (
	ProfileThemeDefault = "default"
	ProfileThemePro     = "theme-pro"
	// InitialCredentialVersion is retained for users whose password credentials
	// have never been rotated. It is also the migration default for existing
	// users, so legacy JWTs remain distinguishable from rotated credentials.
	InitialCredentialVersion = "0"
)

type User struct {
	ID                int64
	Username          string
	Email             string
	PasswordHash      string
	CredentialVersion string
	Nickname          string
	AvatarURL         string
	BackgroundURL     string
	ProfileTheme      string
	Bio               string
	Status            Status
	FollowerCount     int64
	FollowingCount    int64
	CreatedAt         time.Time
	UpdatedAt         time.Time
	LastLoginAt       *time.Time
	EmailVerifiedAt   *time.Time

	events []DomainEvent
}

type RegisterCmd struct {
	Username      string
	Email         string
	Password      string
	Nickname      string
	InviteCode    string
	RequireInvite bool
}

type OAuthLoginCmd struct {
	Provider       string
	ProviderUserID string
	Username       string
	Email          string
	Nickname       string
	AvatarURL      string
	ExistingOnly   bool
}

type WebmasterLoginCmd struct {
	Username string
	Password string
	Email    string
	Nickname string
}

type UpdateProfileCmd struct {
	Nickname      string
	AvatarURL     string
	BackgroundURL string
	ProfileTheme  string
	Bio           string
}

type OAuthAccount struct {
	Provider       string
	ProviderUserID string
	UserID         int64
	Username       string
	Email          string
	Nickname       string
	AvatarURL      string
	CreatedAt      time.Time
	UpdatedAt      time.Time
	LastLoginAt    *time.Time
}

func New(id int64, cmd RegisterCmd, passwordHash string) (*User, error) {
	now := time.Now()
	nickname := strings.TrimSpace(cmd.Nickname)
	if nickname == "" {
		nickname = NormalizeUsername(cmd.Username)
	}
	u := &User{
		ID:                id,
		Username:          NormalizeUsername(cmd.Username),
		Email:             NormalizeEmail(cmd.Email),
		PasswordHash:      passwordHash,
		CredentialVersion: InitialCredentialVersion,
		Nickname:          nickname,
		ProfileTheme:      ProfileThemeDefault,
		Status:            StatusActive,
		CreatedAt:         now,
		UpdatedAt:         now,
	}
	if err := u.Validate(); err != nil {
		return nil, err
	}
	u.AddEvent(NewCreatedEvent(u))
	return u, nil
}

// NormalizeCredentialVersion returns the legacy initial value for an empty
// in-memory value. PostgreSQL stores the column as NOT NULL with this default;
// this fallback keeps callers that construct a User directly compatible.
func NormalizeCredentialVersion(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return InitialCredentialVersion
	}
	return value
}

func ValidCredentialVersion(value string) bool {
	return strings.TrimSpace(value) != ""
}

func (u *User) Validate() error {
	if u == nil || u.ID <= 0 {
		return ErrInvalidID
	}
	if u.Username == "" {
		return ErrUsernameRequired
	}
	if !ValidUsername(u.Username) {
		return ErrUsernameInvalid
	}
	if u.Email == "" {
		return ErrEmailRequired
	}
	if _, err := mail.ParseAddress(u.Email); err != nil {
		return ErrEmailInvalid
	}
	if u.PasswordHash == "" {
		return ErrPasswordRequired
	}
	if strings.TrimSpace(u.Nickname) == "" {
		return ErrNicknameRequired
	}
	if len([]rune(u.Nickname)) > MaxNicknameLen {
		u.Nickname = string([]rune(u.Nickname)[:MaxNicknameLen])
	}
	if len([]rune(u.Bio)) > MaxBioRunes {
		u.Bio = string([]rune(u.Bio)[:MaxBioRunes])
	}
	u.ProfileTheme = NormalizeProfileTheme(u.ProfileTheme)
	if !ValidProfileTheme(u.ProfileTheme) {
		return ErrInvalidProfileTheme
	}
	if !u.Status.IsValid() {
		return ErrInvalidStatus
	}
	return nil
}

func NormalizeProfileTheme(value string) string {
	theme := strings.ToLower(strings.TrimSpace(value))
	if theme == "" {
		return ProfileThemeDefault
	}
	return theme
}

func ValidProfileTheme(value string) bool {
	switch NormalizeProfileTheme(value) {
	case ProfileThemeDefault, ProfileThemePro:
		return true
	default:
		return false
	}
}

func (u *User) UpdateProfile(cmd UpdateProfileCmd) error {
	u.Nickname = strings.TrimSpace(cmd.Nickname)
	u.AvatarURL = strings.TrimSpace(cmd.AvatarURL)
	u.BackgroundURL = strings.TrimSpace(cmd.BackgroundURL)
	if strings.TrimSpace(cmd.ProfileTheme) != "" {
		u.ProfileTheme = NormalizeProfileTheme(cmd.ProfileTheme)
	}
	u.Bio = strings.TrimSpace(cmd.Bio)
	u.UpdatedAt = time.Now()
	if err := u.Validate(); err != nil {
		return err
	}
	u.AddEvent(NewUpdatedEvent(u))
	return nil
}

func (u *User) ChangePasswordHash(passwordHash string) error {
	if passwordHash == "" {
		return ErrPasswordRequired
	}
	u.PasswordHash = passwordHash
	u.UpdatedAt = time.Now()
	u.AddEvent(NewUpdatedEvent(u))
	return nil
}

func (u *User) UpdateStatus(status Status) error {
	u.Status = status
	u.UpdatedAt = time.Now()
	if err := u.Validate(); err != nil {
		return err
	}
	u.AddEvent(NewUpdatedEvent(u))
	return nil
}

func (u *User) EnsureActive() error {
	if !u.Status.IsValid() {
		return ErrInvalidStatus
	}
	return nil
}

func (u *User) TouchLogin(at time.Time) {
	u.LastLoginAt = &at
	u.UpdatedAt = at
}

func (u *User) MarkEmailVerified(at time.Time) {
	if at.IsZero() {
		at = time.Now()
	}
	u.EmailVerifiedAt = &at
	u.UpdatedAt = at
	u.AddEvent(NewUpdatedEvent(u))
}

func (u *User) AddEvent(event DomainEvent) {
	u.events = append(u.events, event)
}

func (u *User) Events() []DomainEvent {
	events := u.events
	u.events = nil
	return events
}
