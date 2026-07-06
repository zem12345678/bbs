package admin

import (
	"errors"
	"strings"
	"testing"

	domain "admin/internal/domain/admin"
)

func TestValidatePasswordPolicy(t *testing.T) {
	tests := []struct {
		name      string
		password  string
		wantError bool
	}{
		{name: "valid", password: "Admin123!", wantError: false},
		{name: "minimum length valid", password: "A1!aaaaa", wantError: false},
		{name: "too short", password: "A1!aaaa", wantError: true},
		{name: "too long", password: "A1!" + strings.Repeat("a", maxAdminPasswordLength), wantError: true},
		{name: "missing letter", password: "12345678!", wantError: true},
		{name: "missing digit", password: "Password!", wantError: true},
		{name: "missing special", password: "Password1", wantError: true},
		{name: "contains space", password: "Admin 123!", wantError: true},
		{name: "contains newline", password: "Admin123!\n", wantError: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validatePasswordPolicy(tt.password)
			if tt.wantError {
				if !errors.Is(err, domain.ErrInvalidPassword) {
					t.Fatalf("validatePasswordPolicy() error = %v, want ErrInvalidPassword", err)
				}
				return
			}
			if err != nil {
				t.Fatalf("validatePasswordPolicy() error = %v, want nil", err)
			}
		})
	}
}
