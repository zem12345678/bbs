package user

import (
	"net/mail"
	"strings"
	"time"
)

type User struct {
	ID             int64
	Username       string
	Email          string
	PasswordHash   string
	Nickname       string
	AvatarURL      string
	Bio            string
	Status         Status
	FollowerCount  int64
	FollowingCount int64
	CreatedAt      time.Time
	UpdatedAt      time.Time
	LastLoginAt    *time.Time

	events []DomainEvent
}

type RegisterCmd struct {
	Username string
	Email    string
	Password string
	Nickname string
}

type UpdateProfileCmd struct {
	Nickname  string
	AvatarURL string
	Bio       string
}

func New(id int64, cmd RegisterCmd, passwordHash string) (*User, error) {
	now := time.Now()
	nickname := strings.TrimSpace(cmd.Nickname)
	if nickname == "" {
		nickname = NormalizeUsername(cmd.Username)
	}
	u := &User{
		ID:           id,
		Username:     NormalizeUsername(cmd.Username),
		Email:        NormalizeEmail(cmd.Email),
		PasswordHash: passwordHash,
		Nickname:     nickname,
		Status:       StatusActive,
		CreatedAt:    now,
		UpdatedAt:    now,
	}
	if err := u.Validate(); err != nil {
		return nil, err
	}
	u.AddEvent(NewCreatedEvent(u))
	return u, nil
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
	if !u.Status.IsValid() {
		return ErrInvalidStatus
	}
	return nil
}

func (u *User) UpdateProfile(cmd UpdateProfileCmd) error {
	u.Nickname = strings.TrimSpace(cmd.Nickname)
	u.AvatarURL = strings.TrimSpace(cmd.AvatarURL)
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

func (u *User) AddEvent(event DomainEvent) {
	u.events = append(u.events, event)
}

func (u *User) Events() []DomainEvent {
	events := u.events
	u.events = nil
	return events
}
