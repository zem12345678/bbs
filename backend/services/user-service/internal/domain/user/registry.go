package user

import (
	"encoding/json"
	"regexp"
	"strings"
	"time"
	"unicode/utf8"
)

const (
	MaxRegistryKeysPerDomain   = 1024
	MaxRegistryKeysPerUser     = 4096
	MaxRegistryKeyRunes        = 1024
	MaxRegistryScopeRunes      = 1024
	MaxRegistryDomainRunes     = 512
	MaxRegistryValueBytes      = 1 << 20
	MaxRegistryScopeValueBytes = 1 << 20
	MaxRegistryUserValueBytes  = 16 << 20
)

var registryScopeSegmentPattern = regexp.MustCompile(`^[a-zA-Z0-9_]+$`)

// RegistryItem is a user-owned key/value record addressed by a scope path and an
// optional third-party domain.
type RegistryItem struct {
	ID        int64
	UserID    int64
	Domain    *string
	Scope     []string
	Key       string
	Value     []byte
	CreatedAt time.Time
	UpdatedAt time.Time
}

type RegistryScopeDomain struct {
	Domain *string
	Scopes [][]string
}

func NewRegistryItem(id, userID int64, domain *string, scope []string, key string, value []byte) (*RegistryItem, error) {
	now := time.Now()
	item := &RegistryItem{
		ID: id, UserID: userID, Domain: domain, Scope: scope, Key: key, Value: value,
		CreatedAt: now, UpdatedAt: now,
	}
	if err := item.Validate(); err != nil {
		return nil, err
	}
	return item, nil
}

func (i *RegistryItem) Validate() error {
	if i == nil || i.ID <= 0 || i.UserID <= 0 {
		return ErrInvalidID
	}
	validatedDomain, err := NormalizeRegistryDomain(i.Domain)
	if err != nil {
		return err
	}
	i.Domain = validatedDomain
	scope, err := NormalizeRegistryScope(i.Scope)
	if err != nil {
		return err
	}
	i.Scope = scope
	validatedKey, err := NormalizeRegistryKey(i.Key)
	if err != nil {
		return err
	}
	i.Key = validatedKey
	value, err := NormalizeRegistryValue(i.Value)
	if err != nil {
		return err
	}
	i.Value = value
	return nil
}

// NormalizeRegistryScope keeps the caller-provided segment order because a scope
// is a path, not a set. An empty scope is the account-level root scope.
func NormalizeRegistryScope(scope []string) ([]string, error) {
	validated := make([]string, 0, len(scope))
	totalRunes := 0
	for _, segment := range scope {
		segmentRunes := utf8.RuneCountInString(segment)
		totalRunes += segmentRunes
		if segmentRunes > MaxRegistryScopeRunes || totalRunes > MaxRegistryScopeRunes || !registryScopeSegmentPattern.MatchString(segment) {
			return nil, ErrRegistryScopeInvalid
		}
		validated = append(validated, segment)
	}
	return validated, nil
}

func NormalizeRegistryKey(key string) (string, error) {
	if key == "" {
		return "", ErrRegistryKeyRequired
	}
	if utf8.RuneCountInString(key) > MaxRegistryKeyRunes {
		return "", ErrRegistryKeyTooLong
	}
	return key, nil
}

func NormalizeRegistryDomain(domain *string) (*string, error) {
	if domain == nil {
		return nil, nil
	}
	value := *domain
	if utf8.RuneCountInString(value) > MaxRegistryDomainRunes {
		return nil, ErrRegistryDomainTooLong
	}
	return &value, nil
}

// NormalizeRegistryValue requires a concrete JSON document. Callers that mean
// "no value" must send an explicit JSON null.
func NormalizeRegistryValue(value []byte) ([]byte, error) {
	if len(value) == 0 || !json.Valid(value) {
		return nil, ErrRegistryValueRequired
	}
	if len(value) > MaxRegistryValueBytes {
		return nil, ErrRegistryValueTooLarge
	}
	return append([]byte(nil), value...), nil
}

// RegistryScopeKey renders a scope path for stable comparison and grouping.
func RegistryScopeKey(scope []string) string {
	return strings.Join(scope, ".")
}
