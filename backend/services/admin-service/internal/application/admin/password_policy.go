package admin

import (
	"unicode"

	domain "admin/internal/domain/admin"
)

const (
	minAdminPasswordLength = 8
	maxAdminPasswordLength = 64
)

func validatePasswordPolicy(password string) error {
	runes := []rune(password)
	if len(runes) < minAdminPasswordLength || len(runes) > maxAdminPasswordLength {
		return domain.ErrInvalidPassword
	}
	var hasLetter bool
	var hasDigit bool
	var hasSpecial bool
	for _, r := range runes {
		if unicode.IsSpace(r) || unicode.IsControl(r) {
			return domain.ErrInvalidPassword
		}
		switch {
		case unicode.IsLetter(r):
			hasLetter = true
		case unicode.IsDigit(r):
			hasDigit = true
		default:
			hasSpecial = true
		}
	}
	if !hasLetter || !hasDigit || !hasSpecial {
		return domain.ErrInvalidPassword
	}
	return nil
}
