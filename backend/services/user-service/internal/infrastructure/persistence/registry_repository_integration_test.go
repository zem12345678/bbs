package persistence

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"strings"
	"testing"
	"time"

	domain "user-service/internal/domain/user"

	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

func TestRegistryRepoPostgresIdentityAndMissingRemove(t *testing.T) {
	db, repo := openRegistryPostgres(t)
	ctx := context.Background()
	base := time.Now().UnixNano()
	owner := createRegistryOwner(t, repo, base)
	t.Cleanup(func() {
		_ = db.Exec("DELETE FROM users WHERE id = ?", owner.ID).Error
	})

	emptyDomain := ""
	spacedDomain := " client "
	scope := []string{"account", "preferences"}
	key := " theme "
	fixtures := []*domain.RegistryItem{
		newRegistryFixture(t, base+1, owner.ID, nil, scope, key, `{"large":9007199254740993}`),
		newRegistryFixture(t, base+2, owner.ID, &emptyDomain, scope, key, `{"source":"empty"}`),
		newRegistryFixture(t, base+3, owner.ID, &spacedDomain, scope, key, `{"source":"spaced"}`),
	}
	for _, fixture := range fixtures {
		if err := repo.SetRegistryItem(ctx, fixture); err != nil {
			t.Fatalf("SetRegistryItem(domain=%v) error = %v", fixture.Domain, err)
		}
	}

	nullItem, err := repo.GetRegistryItem(ctx, owner.ID, nil, scope, key)
	if err != nil {
		t.Fatalf("GetRegistryItem(null domain) error = %v", err)
	}
	if nullItem.Domain != nil || !bytes.Contains(nullItem.Value, []byte("9007199254740993")) {
		t.Fatalf("null-domain item = %+v value=%s", nullItem, nullItem.Value)
	}
	emptyItem, err := repo.GetRegistryItem(ctx, owner.ID, &emptyDomain, scope, key)
	if err != nil {
		t.Fatalf("GetRegistryItem(empty domain) error = %v", err)
	}
	if emptyItem.Domain == nil || *emptyItem.Domain != "" || !bytes.Contains(emptyItem.Value, []byte(`"empty"`)) {
		t.Fatalf("empty-domain item = %+v value=%s", emptyItem, emptyItem.Value)
	}
	spacedItem, err := repo.GetRegistryItem(ctx, owner.ID, &spacedDomain, scope, key)
	if err != nil {
		t.Fatalf("GetRegistryItem(spaced domain) error = %v", err)
	}
	if spacedItem.Domain == nil || *spacedItem.Domain != spacedDomain {
		t.Fatalf("spaced domain = %v, want %q", spacedItem.Domain, spacedDomain)
	}

	if err := repo.RemoveRegistryItem(ctx, owner.ID, nil, scope, key); err != nil {
		t.Fatalf("first RemoveRegistryItem() error = %v", err)
	}
	if err := repo.RemoveRegistryItem(ctx, owner.ID, nil, scope, key); !errors.Is(err, domain.ErrRegistryItemNotFound) {
		t.Fatalf("second RemoveRegistryItem() error = %v, want ErrRegistryItemNotFound", err)
	}
	if _, err := repo.GetRegistryItem(ctx, owner.ID, nil, scope, key); !errors.Is(err, domain.ErrRegistryItemNotFound) {
		t.Fatalf("removed null-domain lookup error = %v, want ErrRegistryItemNotFound", err)
	}
	if _, err := repo.GetRegistryItem(ctx, owner.ID, &emptyDomain, scope, key); err != nil {
		t.Fatalf("empty-domain item was removed with null-domain item: %v", err)
	}
	if _, err := repo.GetRegistryItem(ctx, owner.ID, nil, scope, ""); !errors.Is(err, domain.ErrRegistryItemNotFound) {
		t.Fatalf("empty lookup key error = %v, want ErrRegistryItemNotFound", err)
	}
	if err := repo.RemoveRegistryItem(ctx, owner.ID, nil, scope, ""); !errors.Is(err, domain.ErrRegistryItemNotFound) {
		t.Fatalf("empty-key RemoveRegistryItem() error = %v, want ErrRegistryItemNotFound", err)
	}
}

func TestRegistryRepoPostgresKeyLimitAllowsExistingUpdates(t *testing.T) {
	db, repo := openRegistryPostgres(t)
	ctx := context.Background()
	base := time.Now().UnixNano()
	owner := createRegistryOwner(t, repo, base)
	t.Cleanup(func() {
		_ = db.Exec("DELETE FROM users WHERE id = ?", owner.ID).Error
	})

	now := time.Now().UTC()
	if err := db.Exec(`
		INSERT INTO registry_items (id, user_id, domain, scope, key, value, created_at, updated_at)
		SELECT ?::bigint + n::bigint, ?, NULL, '["limit"]'::jsonb, 'key-' || n, '0'::jsonb, ?, ?
		FROM generate_series(1, ?) AS n`, base, owner.ID, now, now, domain.MaxRegistryKeysPerDomain).Error; err != nil {
		t.Fatalf("seed registry capacity: %v", err)
	}

	overflow := newRegistryFixture(t, base+2000, owner.ID, nil, []string{"limit"}, "overflow", `1`)
	if err := repo.SetRegistryItem(ctx, overflow); !errors.Is(err, domain.ErrRegistryKeyLimitReached) {
		t.Fatalf("overflow SetRegistryItem() error = %v, want ErrRegistryKeyLimitReached", err)
	}
	existing := newRegistryFixture(t, base+3000, owner.ID, nil, []string{"limit"}, "key-1", `{"updated":true}`)
	if err := repo.SetRegistryItem(ctx, existing); err != nil {
		t.Fatalf("update existing item at capacity: %v", err)
	}
	stored, err := repo.GetRegistryItem(ctx, owner.ID, nil, []string{"limit"}, "key-1")
	if err != nil {
		t.Fatalf("read updated item: %v", err)
	}
	if !bytes.Contains(stored.Value, []byte(`"updated": true`)) && !bytes.Contains(stored.Value, []byte(`"updated":true`)) {
		t.Fatalf("updated value = %s", stored.Value)
	}
}

func TestRegistryRepoPostgresScopeValueQuotaAllowsShrinkingUpdates(t *testing.T) {
	db, repo := openRegistryPostgres(t)
	ctx := context.Background()
	base := time.Now().UnixNano()
	owner := createRegistryOwner(t, repo, base)
	t.Cleanup(func() {
		_ = db.Exec("DELETE FROM users WHERE id = ?", owner.ID).Error
	})

	scope := []string{"quota"}
	boundaryValue := `"` + strings.Repeat("a", domain.MaxRegistryScopeValueBytes-2) + `"`
	boundary := newRegistryFixture(t, base+1, owner.ID, nil, scope, "boundary", boundaryValue)
	if err := repo.SetRegistryItem(ctx, boundary); err != nil {
		t.Fatalf("SetRegistryItem(boundary) error = %v", err)
	}
	overflow := newRegistryFixture(t, base+2, owner.ID, nil, scope, "overflow", `true`)
	if err := repo.SetRegistryItem(ctx, overflow); !errors.Is(err, domain.ErrRegistryValueTooLarge) {
		t.Fatalf("scope overflow error = %v, want ErrRegistryValueTooLarge", err)
	}

	shrunk := newRegistryFixture(t, base+3, owner.ID, nil, scope, "boundary", `false`)
	if err := repo.SetRegistryItem(ctx, shrunk); err != nil {
		t.Fatalf("shrink existing value at capacity: %v", err)
	}
	if err := repo.SetRegistryItem(ctx, overflow); err != nil {
		t.Fatalf("insert after shrinking value: %v", err)
	}
}

func TestRegistryRepoPostgresUserKeyQuotaAcrossDomains(t *testing.T) {
	db, repo := openRegistryPostgres(t)
	ctx := context.Background()
	base := time.Now().UnixNano()
	owner := createRegistryOwner(t, repo, base)
	t.Cleanup(func() {
		_ = db.Exec("DELETE FROM users WHERE id = ?", owner.ID).Error
	})

	now := time.Now().UTC()
	if err := db.Exec(`
		INSERT INTO registry_items (id, user_id, domain, scope, key, value, created_at, updated_at)
		SELECT ?::bigint + n::bigint, ?, 'domain-' || n, '["userquota"]'::jsonb,
			'key-' || n, '0'::jsonb, ?, ?
		FROM generate_series(1, ?) AS n`, base, owner.ID, now, now, domain.MaxRegistryKeysPerUser).Error; err != nil {
		t.Fatalf("seed user registry key capacity: %v", err)
	}

	newDomain := "overflow-domain"
	overflow := newRegistryFixture(t, base+5000, owner.ID, &newDomain, []string{"userquota"}, "overflow", `true`)
	if err := repo.SetRegistryItem(ctx, overflow); !errors.Is(err, domain.ErrRegistryKeyLimitReached) {
		t.Fatalf("user key overflow error = %v, want ErrRegistryKeyLimitReached", err)
	}
	existingDomain := "domain-1"
	existing := newRegistryFixture(t, base+5001, owner.ID, &existingDomain, []string{"userquota"}, "key-1", `true`)
	if err := repo.SetRegistryItem(ctx, existing); err != nil {
		t.Fatalf("update existing item at user key capacity: %v", err)
	}
}

func TestRegistryRepoPostgresUserValueQuotaAllowsShrinkingUpdates(t *testing.T) {
	db, repo := openRegistryPostgres(t)
	ctx := context.Background()
	base := time.Now().UnixNano()
	owner := createRegistryOwner(t, repo, base)
	t.Cleanup(func() {
		_ = db.Exec("DELETE FROM users WHERE id = ?", owner.ID).Error
	})

	now := time.Now().UTC()
	valueCount := domain.MaxRegistryUserValueBytes / domain.MaxRegistryScopeValueBytes
	if err := db.Exec(`
		INSERT INTO registry_items (id, user_id, domain, scope, key, value, created_at, updated_at)
		SELECT ?::bigint + n::bigint, ?, NULL, jsonb_build_array('userquota', n::text),
			'key-' || n, to_jsonb(repeat('a', ?)), ?, ?
		FROM generate_series(1, ?) AS n`, base, owner.ID, domain.MaxRegistryScopeValueBytes-2, now, now, valueCount).Error; err != nil {
		t.Fatalf("seed user registry value capacity: %v", err)
	}

	overflow := newRegistryFixture(t, base+100, owner.ID, nil, []string{"userquota", "overflow"}, "overflow", `true`)
	if err := repo.SetRegistryItem(ctx, overflow); !errors.Is(err, domain.ErrRegistryValueTooLarge) {
		t.Fatalf("user value overflow error = %v, want ErrRegistryValueTooLarge", err)
	}
	shrunk := newRegistryFixture(t, base+101, owner.ID, nil, []string{"userquota", "1"}, "key-1", `false`)
	if err := repo.SetRegistryItem(ctx, shrunk); err != nil {
		t.Fatalf("shrink existing value at user capacity: %v", err)
	}
	if err := repo.SetRegistryItem(ctx, overflow); err != nil {
		t.Fatalf("insert after shrinking user value: %v", err)
	}
}

func openRegistryPostgres(t *testing.T) (*gorm.DB, *Repo) {
	t.Helper()
	if os.Getenv("BBS_PG_SMOKE") != "1" {
		t.Skip("set BBS_PG_SMOKE=1 to run PostgreSQL registry repository tests")
	}
	dsn := os.Getenv("BBS_USER_PG_DSN")
	if dsn == "" {
		dsn = "postgres://bbs_user_app:local_user_pass@127.0.0.1:5432/bbs?sslmode=disable&search_path=bbs_user"
	}
	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Fatalf("open postgres: %v", err)
	}
	var registryTableExists bool
	if err := db.Raw("SELECT to_regclass(current_schema() || '.registry_items') IS NOT NULL").Scan(&registryTableExists).Error; err != nil {
		t.Fatalf("check registry_items table: %v", err)
	}
	if !registryTableExists {
		t.Fatal("registry_items table is missing; apply migrations through 0021 before running this test")
	}
	return db, NewRepo(db)
}

func createRegistryOwner(t *testing.T, repo *Repo, id int64) *domain.User {
	t.Helper()
	suffix := fmt.Sprintf("%d", id%1000000000)
	owner, err := domain.New(id, domain.RegisterCmd{
		Username: "registry_" + suffix,
		Email:    "registry_" + suffix + "@example.com",
		Password: "password123",
		Nickname: "Registry Owner",
	}, "hash")
	if err != nil {
		t.Fatalf("new registry owner: %v", err)
	}
	if err := repo.Create(context.Background(), owner); err != nil {
		t.Fatalf("create registry owner: %v", err)
	}
	return owner
}

func newRegistryFixture(t *testing.T, id, userID int64, itemDomain *string, scope []string, key, value string) *domain.RegistryItem {
	t.Helper()
	item, err := domain.NewRegistryItem(id, userID, itemDomain, scope, key, []byte(value))
	if err != nil {
		t.Fatalf("NewRegistryItem() error = %v", err)
	}
	return item
}
