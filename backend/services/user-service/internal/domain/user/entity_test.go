package user

import "testing"

func TestNewUserNormalizesUsernameAndEmail(t *testing.T) {
	u, err := New(1, RegisterCmd{
		Username: " Alice_01 ",
		Email:    " Alice@Example.COM ",
		Password: "password123",
		Nickname: "",
	}, "hash")
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	if u.Username != "alice_01" {
		t.Fatalf("username = %q", u.Username)
	}
	if u.Email != "alice@example.com" {
		t.Fatalf("email = %q", u.Email)
	}
	if len(u.Events()) != 1 {
		t.Fatalf("expected created event")
	}
}

func TestNewUserRejectsInvalidUsername(t *testing.T) {
	_, err := New(1, RegisterCmd{
		Username: "a!",
		Email:    "a@example.com",
		Password: "password123",
		Nickname: "A",
	}, "hash")
	if err != ErrUsernameInvalid {
		t.Fatalf("error = %v, want %v", err, ErrUsernameInvalid)
	}
}
