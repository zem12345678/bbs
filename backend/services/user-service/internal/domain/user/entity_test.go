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
	if u.CredentialVersion != InitialCredentialVersion {
		t.Fatalf("credential version = %q, want %q", u.CredentialVersion, InitialCredentialVersion)
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

func TestUpdateProfileValidatesBirthdayAndListVisibility(t *testing.T) {
	u, err := New(2, RegisterCmd{
		Username: "birthday_user",
		Email:    "birthday@example.com",
		Password: "password123",
		Nickname: "Birthday",
	}, "hash")
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	birthday := "2000-02-29"
	followingVisibility := UserVisibilityPrivate
	followersVisibility := UserVisibilityFollowers
	if err := u.UpdateProfile(UpdateProfileCmd{
		Nickname: "Birthday", BirthdaySet: true, Birthday: &birthday,
		FollowingVisibility: &followingVisibility, FollowersVisibility: &followersVisibility,
	}); err != nil {
		t.Fatalf("valid profile update error = %v", err)
	}
	if u.Birthday == nil || *u.Birthday != birthday || u.FollowingVisibility != UserVisibilityPrivate || u.FollowersVisibility != UserVisibilityFollowers {
		t.Fatalf("updated profile = %+v", u)
	}
	invalidBirthday := "2001-02-29"
	if err := u.UpdateProfile(UpdateProfileCmd{Nickname: "Birthday", BirthdaySet: true, Birthday: &invalidBirthday}); err != ErrInvalidBirthday {
		t.Fatalf("invalid birthday error = %v", err)
	}
	if err := u.UpdateProfile(UpdateProfileCmd{Nickname: "Birthday", BirthdaySet: true, Birthday: &birthday}); err != nil {
		t.Fatalf("restore birthday error = %v", err)
	}
	invalidVisibility := UserVisibility("unknown")
	if err := u.UpdateProfile(UpdateProfileCmd{Nickname: "Birthday", FollowingVisibility: &invalidVisibility}); err != ErrInvalidFollowingVisibility {
		t.Fatalf("invalid visibility error = %v", err)
	}
}
