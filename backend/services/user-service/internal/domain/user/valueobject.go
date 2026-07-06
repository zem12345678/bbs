package user

import (
	"regexp"
	"strings"
)

const (
	MinUsernameLen = 3
	MaxUsernameLen = 32
	MaxNicknameLen = 64
	MaxBioRunes    = 500
)

var usernamePattern = regexp.MustCompile(`^[a-zA-Z0-9_]+$`)

type Status int32

const (
	StatusActive Status = 1
	StatusMuted  Status = 2
)

func (s Status) IsValid() bool {
	return s == StatusActive || s == StatusMuted
}

func NormalizeUsername(username string) string {
	return strings.ToLower(strings.TrimSpace(username))
}

func NormalizeEmail(email string) string {
	return strings.ToLower(strings.TrimSpace(email))
}

func ValidUsername(username string) bool {
	if len(username) < MinUsernameLen || len(username) > MaxUsernameLen {
		return false
	}
	return usernamePattern.MatchString(username)
}
