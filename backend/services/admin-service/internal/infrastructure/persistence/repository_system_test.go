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

func TestRepositoryRejectsDeletingSystemMenuWithChildren(t *testing.T) {
	dsn := os.Getenv("BBS_ADMIN_TEST_DSN")
	if dsn == "" {
		t.Skip("set BBS_ADMIN_TEST_DSN to run postgres-backed repository tests")
	}

	ctx := context.Background()
	repo, cleanup := repositoryForProtectedRoleTest(t, ctx, dsn)
	defer cleanup()

	suffix := time.Now().UnixNano()
	parent, err := repo.CreateSystemMenu(ctx, domain.UpsertSystemMenuCommand{
		Name:       fmt.Sprintf("test.parent.%d", suffix),
		Title:      "Test Parent",
		Path:       fmt.Sprintf("/test-parent-%d", suffix),
		Type:       "M",
		Permission: fmt.Sprintf("system:test_parent_%d", suffix),
		Sort:       9999,
	})
	if err != nil {
		t.Fatalf("CreateSystemMenu(parent) error = %v", err)
	}
	parentCreated := true
	defer func() {
		if parentCreated {
			_ = repo.DeleteSystemMenu(ctx, parent.ID)
		}
	}()

	child, err := repo.CreateSystemMenu(ctx, domain.UpsertSystemMenuCommand{
		ParentID:   parent.ID,
		Name:       fmt.Sprintf("test.child.%d", suffix),
		Title:      "Test Child",
		Path:       fmt.Sprintf("/test-child-%d", suffix),
		Component:  "system/test/index",
		Type:       "C",
		Permission: fmt.Sprintf("system:test_child_%d", suffix),
		Sort:       10000,
	})
	if err != nil {
		t.Fatalf("CreateSystemMenu(child) error = %v", err)
	}
	childCreated := true
	defer func() {
		if childCreated {
			_ = repo.DeleteSystemMenu(ctx, child.ID)
		}
	}()

	if err := repo.DeleteSystemMenu(ctx, parent.ID); !errors.Is(err, domain.ErrSystemMenuHasChildren) {
		t.Fatalf("DeleteSystemMenu(parent) error = %v, want ErrSystemMenuHasChildren", err)
	}
	if err := repo.DeleteSystemMenu(ctx, child.ID); err != nil {
		t.Fatalf("DeleteSystemMenu(child) error = %v", err)
	}
	childCreated = false
	if err := repo.DeleteSystemMenu(ctx, parent.ID); err != nil {
		t.Fatalf("DeleteSystemMenu(parent after child) error = %v", err)
	}
	parentCreated = false
}

func TestRepositoryRejectsDeletingSystemDeptWithChildren(t *testing.T) {
	dsn := os.Getenv("BBS_ADMIN_TEST_DSN")
	if dsn == "" {
		t.Skip("set BBS_ADMIN_TEST_DSN to run postgres-backed repository tests")
	}

	ctx := context.Background()
	repo, cleanup := repositoryForProtectedRoleTest(t, ctx, dsn)
	defer cleanup()

	suffix := time.Now().UnixNano()
	parent, err := repo.CreateSystemDept(ctx, domain.UpsertSystemDeptCommand{
		Name:   fmt.Sprintf("Test Parent %d", suffix),
		Sort:   9999,
		Status: 1,
	})
	if err != nil {
		t.Fatalf("CreateSystemDept(parent) error = %v", err)
	}
	parentCreated := true
	defer func() {
		if parentCreated {
			_ = repo.DeleteSystemDept(ctx, parent.ID)
		}
	}()

	child, err := repo.CreateSystemDept(ctx, domain.UpsertSystemDeptCommand{
		ParentID: parent.ID,
		Name:     fmt.Sprintf("Test Child %d", suffix),
		Sort:     10000,
		Status:   1,
	})
	if err != nil {
		t.Fatalf("CreateSystemDept(child) error = %v", err)
	}
	childCreated := true
	defer func() {
		if childCreated {
			_ = repo.DeleteSystemDept(ctx, child.ID)
		}
	}()

	if err := repo.DeleteSystemDept(ctx, parent.ID); !errors.Is(err, domain.ErrSystemDeptHasChildren) {
		t.Fatalf("DeleteSystemDept(parent) error = %v, want ErrSystemDeptHasChildren", err)
	}
	if err := repo.DeleteSystemDept(ctx, child.ID); err != nil {
		t.Fatalf("DeleteSystemDept(child) error = %v", err)
	}
	childCreated = false
	if err := repo.DeleteSystemDept(ctx, parent.ID); err != nil {
		t.Fatalf("DeleteSystemDept(parent after child) error = %v", err)
	}
	parentCreated = false
}

func TestRepositoryRejectsDeletingSystemDeptWithUsers(t *testing.T) {
	dsn := os.Getenv("BBS_ADMIN_TEST_DSN")
	if dsn == "" {
		t.Skip("set BBS_ADMIN_TEST_DSN to run postgres-backed repository tests")
	}

	ctx := context.Background()
	repo, cleanup := repositoryForProtectedRoleTest(t, ctx, dsn)
	defer cleanup()

	suffix := time.Now().UnixNano()
	dept, err := repo.CreateSystemDept(ctx, domain.UpsertSystemDeptCommand{
		Name:   fmt.Sprintf("User Dept %d", suffix),
		Sort:   9999,
		Status: 1,
	})
	if err != nil {
		t.Fatalf("CreateSystemDept(dept) error = %v", err)
	}
	deptCreated := true
	defer func() {
		if deptCreated {
			_ = repo.DeleteSystemDept(ctx, dept.ID)
		}
	}()

	user, err := repo.CreateSystemUser(ctx, domain.UpsertSystemUserCommand{
		Username: fmt.Sprintf("dept_user_%d", suffix),
		Nickname: "Dept User",
		Email:    fmt.Sprintf("dept_user_%d@example.com", suffix),
		Status:   1,
		DeptID:   dept.ID,
	}, "hash")
	if err != nil {
		t.Fatalf("CreateSystemUser(user) error = %v", err)
	}
	userCreated := true
	defer func() {
		if userCreated {
			_ = repo.DeleteSystemUser(ctx, user.ID)
		}
	}()

	if err := repo.DeleteSystemDept(ctx, dept.ID); !errors.Is(err, domain.ErrSystemDeptHasUsers) {
		t.Fatalf("DeleteSystemDept(dept) error = %v, want ErrSystemDeptHasUsers", err)
	}
	if err := repo.DeleteSystemUser(ctx, user.ID); err != nil {
		t.Fatalf("DeleteSystemUser(user) error = %v", err)
	}
	userCreated = false
	if err := repo.DeleteSystemDept(ctx, dept.ID); err != nil {
		t.Fatalf("DeleteSystemDept(dept after user) error = %v", err)
	}
	deptCreated = false
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
