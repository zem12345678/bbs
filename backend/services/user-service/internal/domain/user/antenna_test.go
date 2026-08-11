package user

import (
	"errors"
	"strings"
	"testing"
)

func TestAntennaValidationNormalizesAndRequiresKeywords(t *testing.T) {
	a, err := NewAntenna(1, 2, "  Backend  ", "ALL", 99, [][]string{{" Go ", ""}}, nil, []string{" @Alice ", "alice"}, false, false, false, false, false, false)
	if err != nil {
		t.Fatalf("NewAntenna() error = %v", err)
	}
	if a.Name != "Backend" || a.Source != "all" || len(a.Keywords) != 1 || len(a.Keywords[0]) != 1 || len(a.Users) != 1 || a.Users[0] != "Alice" || a.UserListID != 0 {
		t.Fatalf("normalized antenna = %+v", a)
	}
	if _, err := NewAntenna(1, 2, "No filters", "all", 0, nil, nil, nil, false, false, false, false, false, false); !errors.Is(err, ErrAntennaKeywordsRequired) {
		t.Fatalf("empty filter error = %v, want ErrAntennaKeywordsRequired", err)
	}
}

func TestAntennaValidationChecksSourceSpecificFields(t *testing.T) {
	if _, err := NewAntenna(1, 2, "List", "list", 0, [][]string{{"x"}}, nil, nil, false, false, false, false, false, false); !errors.Is(err, ErrAntennaUserListRequired) {
		t.Fatalf("list source error = %v", err)
	}
	if _, err := NewAntenna(1, 2, "Users", "users", 0, [][]string{{"x"}}, nil, nil, false, false, false, false, false, false); !errors.Is(err, ErrAntennaUsersRequired) {
		t.Fatalf("users source error = %v", err)
	}
	if _, err := NewAntenna(1, 2, strings.Repeat("界", MaxAntennaNameRunes+1), "all", 0, [][]string{{"x"}}, nil, nil, false, false, false, false, false, false); !errors.Is(err, ErrAntennaNameTooLong) {
		t.Fatalf("long name error = %v", err)
	}
}
