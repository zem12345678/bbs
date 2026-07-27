package persistence

import (
	"testing"

	domain "user-service/internal/domain/user"
)

func TestCredentialVersionRoundTripsThroughUserPersistenceModel(t *testing.T) {
	user := &domain.User{
		ID:                42,
		CredentialVersion: "rotated-version",
	}

	row := toPO(user)
	if row.CredentialVersion != "rotated-version" {
		t.Fatalf("persistent credential version = %q", row.CredentialVersion)
	}
	if got := toEntity(&row).CredentialVersion; got != "rotated-version" {
		t.Fatalf("entity credential version = %q", got)
	}
}

func TestCredentialVersionUsesLegacyInitialValueWhenUnsetInMemory(t *testing.T) {
	row := toPO(&domain.User{})
	if row.CredentialVersion != domain.InitialCredentialVersion {
		t.Fatalf("credential version = %q, want %q", row.CredentialVersion, domain.InitialCredentialVersion)
	}
}
