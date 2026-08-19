package user

import (
	"errors"
	"strings"
	"testing"
)

func TestNewRegistryItemPreservesAddressAndValue(t *testing.T) {
	domain := " client "
	scope := []string{"account", "preferences_1"}
	key := " theme "
	value := []byte(`{"large":9007199254740993}`)

	item, err := NewRegistryItem(1, 2, &domain, scope, key, value)
	if err != nil {
		t.Fatalf("NewRegistryItem() error = %v", err)
	}
	if item.Domain == nil || *item.Domain != domain {
		t.Fatalf("domain = %v, want %q", item.Domain, domain)
	}
	if len(item.Scope) != len(scope) || item.Scope[0] != scope[0] || item.Scope[1] != scope[1] {
		t.Fatalf("scope = %v, want %v", item.Scope, scope)
	}
	if item.Key != key {
		t.Fatalf("key = %q, want %q", item.Key, key)
	}
	if string(item.Value) != string(value) {
		t.Fatalf("value = %s, want %s", item.Value, value)
	}

	domain = "changed"
	scope[0] = "changed"
	value[0] = '['
	if *item.Domain != " client " || item.Scope[0] != "account" || string(item.Value) != `{"large":9007199254740993}` {
		t.Fatalf("item retained caller-owned data: %+v", item)
	}
}

func TestRegistryValidationRejectsInvalidInputWithoutNormalizing(t *testing.T) {
	if _, err := NormalizeRegistryScope([]string{" account "}); !errors.Is(err, ErrRegistryScopeInvalid) {
		t.Fatalf("spaced scope error = %v, want ErrRegistryScopeInvalid", err)
	}
	if _, err := NormalizeRegistryScope([]string{"account", "hyphen-name"}); !errors.Is(err, ErrRegistryScopeInvalid) {
		t.Fatalf("hyphenated scope error = %v, want ErrRegistryScopeInvalid", err)
	}
	if _, err := NormalizeRegistryScope([]string{strings.Repeat("a", MaxRegistryScopeRunes+1)}); !errors.Is(err, ErrRegistryScopeInvalid) {
		t.Fatalf("long scope error = %v, want ErrRegistryScopeInvalid", err)
	}
	if _, err := NormalizeRegistryKey(""); !errors.Is(err, ErrRegistryKeyRequired) {
		t.Fatalf("empty key error = %v, want ErrRegistryKeyRequired", err)
	}
	if _, err := NormalizeRegistryKey(strings.Repeat("界", MaxRegistryKeyRunes+1)); !errors.Is(err, ErrRegistryKeyTooLong) {
		t.Fatalf("long key error = %v, want ErrRegistryKeyTooLong", err)
	}
	emptyDomain := ""
	validatedDomain, err := NormalizeRegistryDomain(&emptyDomain)
	if err != nil {
		t.Fatalf("empty domain error = %v", err)
	}
	if validatedDomain == nil || *validatedDomain != "" {
		t.Fatalf("empty domain = %v, want non-nil empty string", validatedDomain)
	}
	if validatedDomain == &emptyDomain {
		t.Fatal("NormalizeRegistryDomain returned caller-owned pointer")
	}
}

func TestRegistryValueRequiresValidJSON(t *testing.T) {
	for _, value := range [][]byte{nil, {}, []byte(`{"broken":`)} {
		if _, err := NormalizeRegistryValue(value); !errors.Is(err, ErrRegistryValueRequired) {
			t.Fatalf("NormalizeRegistryValue(%q) error = %v, want ErrRegistryValueRequired", value, err)
		}
	}
	if value, err := NormalizeRegistryValue([]byte("null")); err != nil || string(value) != "null" {
		t.Fatalf("explicit null value = %q, err = %v", value, err)
	}
}

func TestRegistryValidationRejectsOversizedScopeAndValue(t *testing.T) {
	if _, err := NormalizeRegistryScope([]string{
		strings.Repeat("a", MaxRegistryScopeRunes/2+1),
		strings.Repeat("b", MaxRegistryScopeRunes/2),
	}); !errors.Is(err, ErrRegistryScopeInvalid) {
		t.Fatalf("oversized total scope error = %v, want ErrRegistryScopeInvalid", err)
	}

	boundary := []byte(`"` + strings.Repeat("a", MaxRegistryValueBytes-2) + `"`)
	if _, err := NormalizeRegistryValue(boundary); err != nil {
		t.Fatalf("boundary registry value error = %v", err)
	}
	oversized := []byte(`"` + strings.Repeat("a", MaxRegistryValueBytes-1) + `"`)
	if _, err := NormalizeRegistryValue(oversized); !errors.Is(err, ErrRegistryValueTooLarge) {
		t.Fatalf("oversized registry value error = %v, want ErrRegistryValueTooLarge", err)
	}
}
