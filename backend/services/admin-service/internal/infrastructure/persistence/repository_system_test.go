package persistence

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"os"
	"testing"
	"time"

	domain "admin/internal/domain/admin"

	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

func TestRepositoryProtectsBuiltInSystemRoles(t *testing.T) {
	dsn := os.Getenv("BBS_ADMIN_TEST_DSN")
	if dsn == "" {
		t.Skip("set BBS_ADMIN_TEST_DSN to run postgres-backed repository tests")
	}

	ctx := context.Background()
	repo, cleanup := repositoryForProtectedRoleTest(t, ctx, dsn)
	defer cleanup()

	adminRole := systemRoleByKey(t, ctx, repo, "admin")
	if _, err := repo.CreateSystemRole(ctx, domain.UpsertSystemRoleCommand{Name: "Duplicate admin", Key: "admin", Status: "1"}); !errors.Is(err, domain.ErrProtectedSystemRole) {
		t.Fatalf("CreateSystemRole(admin) error = %v, want ErrProtectedSystemRole", err)
	}
	if _, err := repo.UpdateSystemRole(ctx, domain.UpsertSystemRoleCommand{ID: adminRole.ID, Name: adminRole.Name, Key: adminRole.Key, Status: adminRole.Status, Sort: adminRole.Sort}); !errors.Is(err, domain.ErrProtectedSystemRole) {
		t.Fatalf("UpdateSystemRole(admin) error = %v, want ErrProtectedSystemRole", err)
	}
	if _, err := repo.AssignSystemRoleMenus(ctx, adminRole.ID, nil); !errors.Is(err, domain.ErrProtectedSystemRole) {
		t.Fatalf("AssignSystemRoleMenus(admin) error = %v, want ErrProtectedSystemRole", err)
	}
	if err := repo.DeleteSystemRole(ctx, adminRole.ID); !errors.Is(err, domain.ErrProtectedSystemRole) {
		t.Fatalf("DeleteSystemRole(admin) error = %v, want ErrProtectedSystemRole", err)
	}

	normalRoleKey := fmt.Sprintf("editor_test_%d", time.Now().UnixNano())
	normalRole, err := repo.CreateSystemRole(ctx, domain.UpsertSystemRoleCommand{Name: "Editor", Key: normalRoleKey, Status: "1", Sort: 90})
	if err != nil {
		t.Fatalf("CreateSystemRole(editor) error = %v", err)
	}
	normalRoleCreated := true
	defer func() {
		if normalRoleCreated {
			_ = repo.DeleteSystemRole(ctx, normalRole.ID)
		}
	}()
	menuIDs := adminRole.MenuIDs
	if len(menuIDs) == 0 {
		t.Fatal("seeded admin role has no system menu ids")
	}
	updated, err := repo.AssignSystemRoleMenus(ctx, normalRole.ID, menuIDs[:1])
	if err != nil {
		t.Fatalf("AssignSystemRoleMenus(editor) error = %v", err)
	}
	if len(updated.MenuIDs) != 1 || updated.MenuIDs[0] != menuIDs[0] {
		t.Fatalf("updated editor menu ids = %v, want [%d]", updated.MenuIDs, menuIDs[0])
	}
	if err := repo.DeleteSystemRole(ctx, normalRole.ID); err != nil {
		t.Fatalf("DeleteSystemRole(editor) error = %v", err)
	}
	normalRoleCreated = false
}

func TestRepositoryProtectsBuiltInSystemUsers(t *testing.T) {
	dsn := os.Getenv("BBS_ADMIN_TEST_DSN")
	if dsn == "" {
		t.Skip("set BBS_ADMIN_TEST_DSN to run postgres-backed repository tests")
	}

	ctx := context.Background()
	repo, cleanup := repositoryForProtectedRoleTest(t, ctx, dsn)
	defer cleanup()

	adminUser := systemUserByUsername(t, ctx, repo, "admin")
	if _, err := repo.CreateSystemUser(ctx, domain.UpsertSystemUserCommand{Username: "admin", Password: "Admin123!"}, "hash"); !errors.Is(err, domain.ErrProtectedSystemUser) {
		t.Fatalf("CreateSystemUser(admin) error = %v, want ErrProtectedSystemUser", err)
	}
	if _, err := repo.UpdateSystemUser(ctx, domain.UpsertSystemUserCommand{ID: adminUser.ID, Username: adminUser.Username, Status: adminUser.Status}); !errors.Is(err, domain.ErrProtectedSystemUser) {
		t.Fatalf("UpdateSystemUser(admin) error = %v, want ErrProtectedSystemUser", err)
	}
	if _, err := repo.ResetSystemUserPassword(ctx, adminUser.ID, "hash"); !errors.Is(err, domain.ErrProtectedSystemUser) {
		t.Fatalf("ResetSystemUserPassword(admin) error = %v, want ErrProtectedSystemUser", err)
	}
	if _, err := repo.AssignSystemUserRoles(ctx, adminUser.ID, adminUser.RoleIDs); !errors.Is(err, domain.ErrProtectedSystemUser) {
		t.Fatalf("AssignSystemUserRoles(admin) error = %v, want ErrProtectedSystemUser", err)
	}
	if err := repo.DeleteSystemUser(ctx, adminUser.ID); !errors.Is(err, domain.ErrProtectedSystemUser) {
		t.Fatalf("DeleteSystemUser(admin) error = %v, want ErrProtectedSystemUser", err)
	}
}

func openPostgresForTest(t *testing.T, dsn string) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Fatalf("open postgres test db: %v", err)
	}
	return db
}

func testDSNWithSearchPath(t *testing.T, dsn string, searchPath string) string {
	t.Helper()
	parsed, err := url.Parse(dsn)
	if err != nil || parsed.Scheme == "" || parsed.Host == "" {
		t.Fatalf("BBS_ADMIN_TEST_DSN must be a postgres URL DSN: %v", err)
	}
	query := parsed.Query()
	if searchPath == "" {
		query.Del("search_path")
	} else {
		query.Set("search_path", searchPath)
	}
	parsed.RawQuery = query.Encode()
	return parsed.String()
}

func repositoryForProtectedRoleTest(t *testing.T, ctx context.Context, dsn string) (*Repository, func()) {
	t.Helper()
	if os.Getenv("BBS_ADMIN_TEST_USE_EXISTING_SCHEMA") == "1" {
		return NewRepository(openPostgresForTest(t, dsn)), func() {}
	}

	schema := fmt.Sprintf("admin_service_test_%d", time.Now().UnixNano())
	adminDB := openPostgresForTest(t, testDSNWithSearchPath(t, dsn, ""))
	createTestSchema(t, ctx, adminDB, schema)
	cleanup := func() {
		dropTestSchema(t, ctx, adminDB, schema)
	}

	db := openPostgresForTest(t, testDSNWithSearchPath(t, dsn, schema))
	repo := NewRepository(db)
	if err := repo.EnsureSchema(ctx); err != nil {
		cleanup()
		t.Fatalf("EnsureSchema() error = %v", err)
	}
	if err := repo.SeedDefaults(ctx, []string{"admin"}, "Admin123!"); err != nil {
		cleanup()
		t.Fatalf("SeedDefaults() error = %v", err)
	}
	return repo, cleanup
}

func createTestSchema(t *testing.T, ctx context.Context, db *gorm.DB, schema string) {
	t.Helper()
	if err := db.WithContext(ctx).Exec(fmt.Sprintf(`CREATE SCHEMA "%s"`, schema)).Error; err != nil {
		t.Fatalf("create test schema %s: %v", schema, err)
	}
}

func dropTestSchema(t *testing.T, ctx context.Context, db *gorm.DB, schema string) {
	t.Helper()
	if err := db.WithContext(ctx).Exec(fmt.Sprintf(`DROP SCHEMA IF EXISTS "%s" CASCADE`, schema)).Error; err != nil {
		t.Fatalf("drop test schema %s: %v", schema, err)
	}
}

func systemRoleByKey(t *testing.T, ctx context.Context, repo *Repository, key string) domain.SystemRole {
	t.Helper()
	roles, err := repo.ListSystemRoles(ctx, "", "", 1, 100)
	if err != nil {
		t.Fatalf("ListSystemRoles() error = %v", err)
	}
	for _, role := range roles.Items {
		if role.Key == key {
			return role
		}
	}
	t.Fatalf("role %q not found in %#v", key, roles.Items)
	return domain.SystemRole{}
}

func systemUserByUsername(t *testing.T, ctx context.Context, repo *Repository, username string) domain.SystemUser {
	t.Helper()
	users, err := repo.ListSystemUsers(ctx, username, 0, 1, 100)
	if err != nil {
		t.Fatalf("ListSystemUsers() error = %v", err)
	}
	for _, user := range users.Items {
		if user.Username == username {
			return user
		}
	}
	t.Fatalf("user %q not found in %#v", username, users.Items)
	return domain.SystemUser{}
}
