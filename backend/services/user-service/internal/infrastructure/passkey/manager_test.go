package passkey

import (
	"encoding/binary"
	"encoding/json"
	"strings"
	"testing"
	"time"

	domain "user-service/internal/domain/user"

	"github.com/descope/virtualwebauthn"
	"github.com/go-webauthn/webauthn/protocol"
	"github.com/go-webauthn/webauthn/webauthn"
)

func TestManagerCompletesVirtualWebAuthnCeremonies(t *testing.T) {
	const userID int64 = 42
	manager, err := New(Options{
		RPID: "example.com", RPDisplayName: "Example Community", RPOrigins: []string{"https://example.com"},
		EncryptionKey: "virtual-webauthn-test-encryption-key",
	})
	if err != nil {
		t.Fatalf("new manager: %v", err)
	}
	user := domain.PasskeyUser{ID: userID, Username: "alice", DisplayName: "Alice"}
	rp := virtualwebauthn.RelyingParty{Name: "Example Community", ID: "example.com", Origin: "https://example.com"}
	authenticator := virtualwebauthn.NewAuthenticator()
	credential := virtualwebauthn.NewCredential(virtualwebauthn.KeyTypeEC2)

	registration, err := manager.BeginRegistration(user)
	if err != nil {
		t.Fatalf("begin registration: %v", err)
	}
	registrationOptions, err := virtualwebauthn.ParseAttestationOptions(registration.OptionsJSON)
	if err != nil {
		t.Fatalf("parse registration options: %v", err)
	}
	registrationResponse := virtualwebauthn.CreateAttestationResponse(rp, authenticator, credential, *registrationOptions)
	registered, err := manager.FinishRegistration(user, registration.SessionCiphertext, registrationResponse)
	if err != nil {
		t.Fatalf("finish registration: %v", err)
	}
	if registered.CredentialID == "" || registered.CredentialCiphertext == "" || registered.UserID != userID {
		t.Fatalf("registered credential = %+v", registered)
	}
	registered.Name = "Laptop"
	registered.Version = 7
	user.Credentials = []domain.PasskeyCredential{registered}

	userHandle := make([]byte, 8)
	binary.BigEndian.PutUint64(userHandle, uint64(userID))
	authenticator.Options.UserHandle = userHandle
	authenticator.AddCredential(credential)
	credential.Counter = 1

	login, err := manager.BeginLogin(user)
	if err != nil {
		t.Fatalf("begin login: %v", err)
	}
	loginOptions, err := virtualwebauthn.ParseAssertionOptions(login.OptionsJSON)
	if err != nil {
		t.Fatalf("parse login options: %v", err)
	}
	if authenticator.FindAllowedCredential(*loginOptions) == nil {
		t.Fatal("registered credential is not allowed for username login")
	}
	loginResponse := virtualwebauthn.CreateAssertionResponse(rp, authenticator, credential, *loginOptions)
	loggedIn, err := manager.FinishLogin(user, login.SessionCiphertext, loginResponse)
	if err != nil {
		t.Fatalf("finish login: %v", err)
	}
	if loggedIn.CredentialID != registered.CredentialID || loggedIn.Name != registered.Name || loggedIn.Version != registered.Version {
		t.Fatalf("updated username-login credential = %+v", loggedIn)
	}
	assertStoredCounter(t, manager, user, loggedIn, 1)

	user.Credentials = []domain.PasskeyCredential{loggedIn}
	credential.Counter = 2
	passwordless, err := manager.BeginPasswordlessLogin()
	if err != nil {
		t.Fatalf("begin passwordless login: %v", err)
	}
	passwordlessOptions, err := virtualwebauthn.ParseAssertionOptions(passwordless.OptionsJSON)
	if err != nil {
		t.Fatalf("parse passwordless options: %v", err)
	}
	passwordlessResponse := virtualwebauthn.CreateAssertionResponse(rp, authenticator, credential, *passwordlessOptions)
	lookupCalled := false
	resolvedID, passwordlessCredential, err := manager.FinishPasswordlessLogin(passwordless.SessionCiphertext, passwordlessResponse, func(credentialID string, resolvedUserID int64) (domain.PasskeyUser, error) {
		lookupCalled = true
		if credentialID != registered.CredentialID || resolvedUserID != userID {
			t.Errorf("passwordless lookup = credential %q, user %d", credentialID, resolvedUserID)
		}
		return user, nil
	})
	if err != nil {
		t.Fatalf("finish passwordless login: %v", err)
	}
	if !lookupCalled || resolvedID != userID || passwordlessCredential.CredentialID != registered.CredentialID {
		t.Fatalf("passwordless result = user %d, credential %+v, lookup called %t", resolvedID, passwordlessCredential, lookupCalled)
	}
	assertStoredCounter(t, manager, user, passwordlessCredential, 2)
}

func assertStoredCounter(t *testing.T, manager *Manager, user domain.PasskeyUser, credential domain.PasskeyCredential, want uint32) {
	t.Helper()
	user.Credentials = []domain.PasskeyCredential{credential}
	decoded, err := manager.user(user)
	if err != nil {
		t.Fatalf("decode updated credential: %v", err)
	}
	if len(decoded.credentials) != 1 || decoded.credentials[0].Authenticator.SignCount != want {
		t.Fatalf("credential sign count = %+v, want %d", decoded.credentials, want)
	}
}

func TestManagerProducesEncryptedRequiredPasskeyOptions(t *testing.T) {
	manager, err := New(Options{
		RPID: "example.com", RPDisplayName: "Example Community", RPOrigins: []string{"https://example.com"},
		EncryptionKey: "passkey-manager-test-encryption-key", CeremonyTTL: 4 * time.Minute,
	})
	if err != nil {
		t.Fatalf("new manager: %v", err)
	}
	user := domain.PasskeyUser{ID: 42, Username: "alice", DisplayName: "Alice"}
	registration, err := manager.BeginRegistration(user)
	if err != nil {
		t.Fatalf("begin registration: %v", err)
	}
	if registration.SessionCiphertext == "" || strings.HasPrefix(registration.SessionCiphertext, "{") || registration.ExpiresAt.IsZero() {
		t.Fatalf("registration session is not encrypted with an expiry: %+v", registration)
	}
	var options struct {
		PublicKey struct {
			Challenge string `json:"challenge"`
			RP        struct {
				ID string `json:"id"`
			} `json:"rp"`
			AuthenticatorSelection struct {
				ResidentKey      string `json:"residentKey"`
				UserVerification string `json:"userVerification"`
			} `json:"authenticatorSelection"`
		} `json:"publicKey"`
	}
	if err := json.Unmarshal([]byte(registration.OptionsJSON), &options); err != nil {
		t.Fatalf("decode registration options: %v", err)
	}
	if options.PublicKey.Challenge == "" || options.PublicKey.RP.ID != "example.com" || options.PublicKey.AuthenticatorSelection.ResidentKey != "required" || options.PublicKey.AuthenticatorSelection.UserVerification != "required" {
		t.Fatalf("registration policy = %+v", options.PublicKey)
	}
	session, err := manager.session(registration.SessionCiphertext)
	if err != nil {
		t.Fatalf("decrypt session: %v", err)
	}
	if session.Challenge != options.PublicKey.Challenge || session.Expires.IsZero() {
		t.Fatalf("stored session does not match options: %+v", session)
	}

	passwordless, err := manager.BeginPasswordlessLogin()
	if err != nil {
		t.Fatalf("begin passwordless login: %v", err)
	}
	var loginOptions struct {
		PublicKey struct {
			UserVerification string `json:"userVerification"`
			AllowCredentials []any  `json:"allowCredentials"`
		} `json:"publicKey"`
	}
	if err := json.Unmarshal([]byte(passwordless.OptionsJSON), &loginOptions); err != nil {
		t.Fatalf("decode passwordless options: %v", err)
	}
	if loginOptions.PublicKey.UserVerification != "required" || len(loginOptions.PublicKey.AllowCredentials) != 0 {
		t.Fatalf("passwordless options = %+v", loginOptions.PublicKey)
	}
}

func TestManagerEncryptsCredentialAndGeneratesOpaqueChallenges(t *testing.T) {
	manager, err := New(Options{RPID: "example.com", RPDisplayName: "Example", RPOrigins: []string{"https://example.com"}, EncryptionKey: "credential-encryption-key"})
	if err != nil {
		t.Fatalf("new manager: %v", err)
	}
	credential := &webauthn.Credential{
		ID: []byte{1, 2, 3}, PublicKey: []byte{4, 5, 6},
		Flags:         webauthn.NewCredentialFlags(protocol.FlagUserPresent | protocol.FlagUserVerified | protocol.FlagBackupEligible),
		Authenticator: webauthn.Authenticator{SignCount: 7},
	}
	stored, err := manager.credential(42, "Laptop", 3, credential)
	if err != nil {
		t.Fatalf("encrypt credential: %v", err)
	}
	if stored.CredentialCiphertext == "" || strings.Contains(stored.CredentialCiphertext, "publicKey") || stored.CredentialID != "AQID" {
		t.Fatalf("stored credential = %+v", stored)
	}
	user, err := manager.user(domain.PasskeyUser{ID: 42, Username: "alice", Credentials: []domain.PasskeyCredential{stored}})
	if err != nil {
		t.Fatalf("decrypt credential: %v", err)
	}
	if len(user.credentials) != 1 || user.credentials[0].Authenticator.SignCount != 7 || string(user.credentials[0].PublicKey) != string(credential.PublicKey) {
		t.Fatalf("decrypted credential = %+v", user.credentials)
	}

	rawOne, hashOne, err := manager.NewChallenge()
	if err != nil {
		t.Fatalf("new challenge: %v", err)
	}
	rawTwo, hashTwo, err := manager.NewChallenge()
	if err != nil {
		t.Fatalf("new second challenge: %v", err)
	}
	if rawOne == rawTwo || hashOne == hashTwo || len(hashOne) != 64 || manager.HashChallenge(rawOne) != hashOne {
		t.Fatalf("challenge tokens are not opaque and unique: %q %q", rawOne, rawTwo)
	}
}
