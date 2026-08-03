package user

import (
	"errors"
	"strings"
	"testing"
)

func TestUserListNameUsesUnicodeRuneLimit(t *testing.T) {
	name := strings.Repeat("界", MaxUserListNameRunes)
	list, err := NewUserList(1, 2, "  "+name+"  ", false)
	if err != nil {
		t.Fatalf("NewUserList() error = %v", err)
	}
	if list.Name != name {
		t.Fatalf("normalized name = %q", list.Name)
	}
	if _, err := NewUserList(1, 2, strings.Repeat("界", MaxUserListNameRunes+1), false); !errors.Is(err, ErrUserListNameTooLong) {
		t.Fatalf("long Unicode name error = %v, want ErrUserListNameTooLong", err)
	}
	if _, err := NewUserList(1, 2, " \t\n ", false); !errors.Is(err, ErrUserListNameRequired) {
		t.Fatalf("blank name error = %v, want ErrUserListNameRequired", err)
	}
}
