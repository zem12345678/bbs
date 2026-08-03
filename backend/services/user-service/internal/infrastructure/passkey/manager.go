package passkey

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	domain "user-service/internal/domain/user"

	"github.com/go-webauthn/webauthn/protocol"
	"github.com/go-webauthn/webauthn/webauthn"
)

const defaultCeremonyTTL = 5 * time.Minute

type Options struct {
	RPID          string
	RPDisplayName string
	RPOrigins     []string
	EncryptionKey string
	CeremonyTTL   time.Duration
}

type Manager struct {
	webAuthn *webauthn.WebAuthn
	aead     cipher.AEAD
}

func New(options Options) (*Manager, error) {
	options.RPID = strings.TrimSpace(options.RPID)
	options.RPDisplayName = strings.TrimSpace(options.RPDisplayName)
	options.EncryptionKey = strings.TrimSpace(options.EncryptionKey)
	if options.RPID == "" || options.RPDisplayName == "" || options.EncryptionKey == "" {
		return nil, fmt.Errorf("passkey RP ID, display name, and encryption key are required")
	}
	origins := make([]string, 0, len(options.RPOrigins))
	for _, origin := range options.RPOrigins {
		if origin = strings.TrimSpace(origin); origin != "" {
			origins = append(origins, origin)
		}
	}
	if len(origins) == 0 {
		return nil, fmt.Errorf("at least one passkey RP origin is required")
	}
	if options.CeremonyTTL <= 0 {
		options.CeremonyTTL = defaultCeremonyTTL
	}
	wa, err := webauthn.New(&webauthn.Config{
		RPID:                  options.RPID,
		RPDisplayName:         options.RPDisplayName,
		RPOrigins:             origins,
		AttestationPreference: protocol.PreferNoAttestation,
		AuthenticatorSelection: protocol.AuthenticatorSelection{
			RequireResidentKey: protocol.ResidentKeyRequired(),
			ResidentKey:        protocol.ResidentKeyRequirementRequired,
			UserVerification:   protocol.VerificationRequired,
		},
		Timeouts: webauthn.TimeoutsConfig{
			Login:        webauthn.TimeoutConfig{Enforce: true, Timeout: options.CeremonyTTL, TimeoutUVD: options.CeremonyTTL},
			Registration: webauthn.TimeoutConfig{Enforce: true, Timeout: options.CeremonyTTL, TimeoutUVD: options.CeremonyTTL},
		},
	})
	if err != nil {
		return nil, fmt.Errorf("configure passkeys: %w", err)
	}
	key := sha256.Sum256([]byte("bbs/passkeys/v1\x00" + options.EncryptionKey))
	block, err := aes.NewCipher(key[:])
	if err != nil {
		return nil, fmt.Errorf("create passkey cipher: %w", err)
	}
	aead, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("create passkey GCM: %w", err)
	}
	return &Manager{webAuthn: wa, aead: aead}, nil
}

func (m *Manager) NewChallenge() (string, string, error) {
	raw := make([]byte, 32)
	if _, err := rand.Read(raw); err != nil {
		return "", "", fmt.Errorf("generate passkey challenge token: %w", err)
	}
	token := base64.RawURLEncoding.EncodeToString(raw)
	return token, m.HashChallenge(token), nil
}

func (m *Manager) HashChallenge(token string) string {
	sum := sha256.Sum256([]byte(strings.TrimSpace(token)))
	return hex.EncodeToString(sum[:])
}

func (m *Manager) BeginRegistration(user domain.PasskeyUser) (domain.PasskeyCeremony, error) {
	waUser, err := m.user(user)
	if err != nil {
		return domain.PasskeyCeremony{}, err
	}
	creation, session, err := m.webAuthn.BeginRegistration(
		waUser,
		webauthn.WithExclusions(webauthn.Credentials(waUser.credentials).CredentialDescriptors()),
		webauthn.WithResidentKeyRequirement(protocol.ResidentKeyRequirementRequired),
	)
	if err != nil {
		return domain.PasskeyCeremony{}, fmt.Errorf("begin passkey registration: %w", err)
	}
	return m.ceremony(creation, session)
}

func (m *Manager) FinishRegistration(user domain.PasskeyUser, sessionCiphertext string, responseJSON string) (domain.PasskeyCredential, error) {
	waUser, err := m.user(user)
	if err != nil {
		return domain.PasskeyCredential{}, err
	}
	session, err := m.session(sessionCiphertext)
	if err != nil {
		return domain.PasskeyCredential{}, err
	}
	parsed, err := protocol.ParseCredentialCreationResponseBytes([]byte(responseJSON))
	if err != nil {
		return domain.PasskeyCredential{}, fmt.Errorf("parse passkey registration response: %w", err)
	}
	credential, err := m.webAuthn.CreateCredential(waUser, session, parsed)
	if err != nil {
		return domain.PasskeyCredential{}, fmt.Errorf("verify passkey registration: %w", err)
	}
	return m.credential(user.ID, "", 0, credential)
}

func (m *Manager) BeginLogin(user domain.PasskeyUser) (domain.PasskeyCeremony, error) {
	waUser, err := m.user(user)
	if err != nil {
		return domain.PasskeyCeremony{}, err
	}
	assertion, session, err := m.webAuthn.BeginLogin(waUser, webauthn.WithUserVerification(protocol.VerificationRequired))
	if err != nil {
		return domain.PasskeyCeremony{}, fmt.Errorf("begin passkey login: %w", err)
	}
	return m.ceremony(assertion, session)
}

func (m *Manager) FinishLogin(user domain.PasskeyUser, sessionCiphertext string, responseJSON string) (domain.PasskeyCredential, error) {
	waUser, err := m.user(user)
	if err != nil {
		return domain.PasskeyCredential{}, err
	}
	session, err := m.session(sessionCiphertext)
	if err != nil {
		return domain.PasskeyCredential{}, err
	}
	parsed, err := protocol.ParseCredentialRequestResponseBytes([]byte(responseJSON))
	if err != nil {
		return domain.PasskeyCredential{}, fmt.Errorf("parse passkey login response: %w", err)
	}
	credential, err := m.webAuthn.ValidateLogin(waUser, session, parsed)
	if err != nil {
		return domain.PasskeyCredential{}, fmt.Errorf("verify passkey login: %w", err)
	}
	source, ok := findCredential(user.Credentials, credential.ID)
	if !ok {
		return domain.PasskeyCredential{}, fmt.Errorf("verified passkey is not present in source credentials")
	}
	return m.credential(user.ID, source.Name, source.Version, credential)
}

func (m *Manager) BeginPasswordlessLogin() (domain.PasskeyCeremony, error) {
	assertion, session, err := m.webAuthn.BeginDiscoverableLogin(webauthn.WithUserVerification(protocol.VerificationRequired))
	if err != nil {
		return domain.PasskeyCeremony{}, fmt.Errorf("begin passwordless passkey login: %w", err)
	}
	return m.ceremony(assertion, session)
}

func (m *Manager) FinishPasswordlessLogin(sessionCiphertext string, responseJSON string, lookup func(credentialID string, userID int64) (domain.PasskeyUser, error)) (int64, domain.PasskeyCredential, error) {
	if lookup == nil {
		return 0, domain.PasskeyCredential{}, fmt.Errorf("passkey lookup required")
	}
	session, err := m.session(sessionCiphertext)
	if err != nil {
		return 0, domain.PasskeyCredential{}, err
	}
	parsed, err := protocol.ParseCredentialRequestResponseBytes([]byte(responseJSON))
	if err != nil {
		return 0, domain.PasskeyCredential{}, fmt.Errorf("parse passwordless passkey response: %w", err)
	}
	var resolved *webAuthnUser
	_, credential, err := m.webAuthn.ValidatePasskeyLogin(func(rawID, userHandle []byte) (webauthn.User, error) {
		userID, decodeErr := decodeUserHandle(userHandle)
		if decodeErr != nil {
			return nil, decodeErr
		}
		source, lookupErr := lookup(base64.RawURLEncoding.EncodeToString(rawID), userID)
		if lookupErr != nil {
			return nil, lookupErr
		}
		resolved, lookupErr = m.user(source)
		return resolved, lookupErr
	}, session, parsed)
	if err != nil {
		return 0, domain.PasskeyCredential{}, fmt.Errorf("verify passwordless passkey login: %w", err)
	}
	if resolved == nil {
		return 0, domain.PasskeyCredential{}, fmt.Errorf("passwordless passkey user was not resolved")
	}
	source, ok := findCredential(resolved.source.Credentials, credential.ID)
	if !ok {
		return 0, domain.PasskeyCredential{}, fmt.Errorf("verified passwordless passkey is not present in source credentials")
	}
	updated, err := m.credential(resolved.source.ID, source.Name, source.Version, credential)
	return resolved.source.ID, updated, err
}

func (m *Manager) ceremony(options any, session *webauthn.SessionData) (domain.PasskeyCeremony, error) {
	optionsJSON, err := json.Marshal(options)
	if err != nil {
		return domain.PasskeyCeremony{}, fmt.Errorf("encode passkey options: %w", err)
	}
	sessionJSON, err := json.Marshal(session)
	if err != nil {
		return domain.PasskeyCeremony{}, fmt.Errorf("encode passkey session: %w", err)
	}
	ciphertext, err := m.encrypt(sessionJSON)
	if err != nil {
		return domain.PasskeyCeremony{}, err
	}
	return domain.PasskeyCeremony{OptionsJSON: string(optionsJSON), SessionCiphertext: ciphertext, ExpiresAt: session.Expires}, nil
}

func (m *Manager) session(ciphertext string) (webauthn.SessionData, error) {
	plaintext, err := m.decrypt(ciphertext)
	if err != nil {
		return webauthn.SessionData{}, err
	}
	var session webauthn.SessionData
	if err := json.Unmarshal(plaintext, &session); err != nil {
		return webauthn.SessionData{}, fmt.Errorf("decode passkey session: %w", err)
	}
	return session, nil
}

func (m *Manager) credential(userID int64, name string, version int64, credential *webauthn.Credential) (domain.PasskeyCredential, error) {
	encoded, err := json.Marshal(credential)
	if err != nil {
		return domain.PasskeyCredential{}, fmt.Errorf("encode passkey credential: %w", err)
	}
	ciphertext, err := m.encrypt(encoded)
	if err != nil {
		return domain.PasskeyCredential{}, err
	}
	return domain.PasskeyCredential{
		CredentialID:         base64.RawURLEncoding.EncodeToString(credential.ID),
		UserID:               userID,
		Name:                 name,
		CredentialCiphertext: ciphertext,
		Version:              version,
		BackupEligible:       credential.Flags.BackupEligible,
		BackupState:          credential.Flags.BackupState,
	}, nil
}

func (m *Manager) user(source domain.PasskeyUser) (*webAuthnUser, error) {
	if source.ID <= 0 || strings.TrimSpace(source.Username) == "" {
		return nil, fmt.Errorf("valid passkey user required")
	}
	credentials := make([]webauthn.Credential, 0, len(source.Credentials))
	for _, stored := range source.Credentials {
		plaintext, err := m.decrypt(stored.CredentialCiphertext)
		if err != nil {
			return nil, fmt.Errorf("decrypt passkey credential %q: %w", stored.CredentialID, err)
		}
		var credential webauthn.Credential
		if err := json.Unmarshal(plaintext, &credential); err != nil {
			return nil, fmt.Errorf("decode passkey credential %q: %w", stored.CredentialID, err)
		}
		if base64.RawURLEncoding.EncodeToString(credential.ID) != stored.CredentialID {
			return nil, fmt.Errorf("passkey credential ID mismatch")
		}
		credentials = append(credentials, credential)
	}
	displayName := strings.TrimSpace(source.DisplayName)
	if displayName == "" {
		displayName = source.Username
	}
	return &webAuthnUser{source: source, displayName: displayName, credentials: credentials}, nil
}

func (m *Manager) encrypt(plaintext []byte) (string, error) {
	if m == nil || m.aead == nil || len(plaintext) == 0 {
		return "", fmt.Errorf("passkey cipher unavailable")
	}
	nonce := make([]byte, m.aead.NonceSize())
	if _, err := rand.Read(nonce); err != nil {
		return "", fmt.Errorf("generate passkey nonce: %w", err)
	}
	sealed := m.aead.Seal(nil, nonce, plaintext, nil)
	return base64.RawURLEncoding.EncodeToString(append(nonce, sealed...)), nil
}

func (m *Manager) decrypt(ciphertext string) ([]byte, error) {
	if m == nil || m.aead == nil {
		return nil, fmt.Errorf("passkey cipher unavailable")
	}
	raw, err := base64.RawURLEncoding.DecodeString(strings.TrimSpace(ciphertext))
	if err != nil || len(raw) <= m.aead.NonceSize() {
		return nil, fmt.Errorf("invalid passkey ciphertext")
	}
	plaintext, err := m.aead.Open(nil, raw[:m.aead.NonceSize()], raw[m.aead.NonceSize():], nil)
	if err != nil {
		return nil, fmt.Errorf("decrypt passkey data: %w", err)
	}
	return plaintext, nil
}

type webAuthnUser struct {
	source      domain.PasskeyUser
	displayName string
	credentials []webauthn.Credential
}

func (u *webAuthnUser) WebAuthnID() []byte {
	handle := make([]byte, 8)
	binary.BigEndian.PutUint64(handle, uint64(u.source.ID))
	return handle
}

func (u *webAuthnUser) WebAuthnName() string                       { return u.source.Username }
func (u *webAuthnUser) WebAuthnDisplayName() string                { return u.displayName }
func (u *webAuthnUser) WebAuthnCredentials() []webauthn.Credential { return u.credentials }

func decodeUserHandle(handle []byte) (int64, error) {
	if len(handle) != 8 {
		return 0, fmt.Errorf("invalid passkey user handle")
	}
	value := binary.BigEndian.Uint64(handle)
	if value == 0 || value > uint64(^uint64(0)>>1) {
		return 0, fmt.Errorf("invalid passkey user handle")
	}
	return int64(value), nil
}

func findCredential(credentials []domain.PasskeyCredential, rawID []byte) (domain.PasskeyCredential, bool) {
	id := base64.RawURLEncoding.EncodeToString(rawID)
	for _, credential := range credentials {
		if credential.CredentialID == id {
			return credential, true
		}
	}
	return domain.PasskeyCredential{}, false
}
