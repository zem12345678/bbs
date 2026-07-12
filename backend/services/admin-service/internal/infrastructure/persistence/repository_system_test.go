package persistence

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"os"
	"strings"
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

func TestSeedDefaultsGrantsDashboardPermissionToAdmin(t *testing.T) {
	dsn := os.Getenv("BBS_ADMIN_TEST_DSN")
	if dsn == "" {
		t.Skip("set BBS_ADMIN_TEST_DSN to run postgres-backed repository tests")
	}

	ctx := context.Background()
	repo, cleanup := repositoryForProtectedRoleTest(t, ctx, dsn)
	defer cleanup()

	for _, role := range []string{"admin", "superadmin"} {
		permissions, err := repo.PermissionsByRoleKeys(ctx, []string{role})
		if err != nil {
			t.Fatalf("PermissionsByRoleKeys(%q) error = %v", role, err)
		}
		if !containsString(permissions, "system:view_dashboard") {
			t.Fatalf("PermissionsByRoleKeys(%q) = %v, want system:view_dashboard", role, permissions)
		}
		if !containsString(permissions, "mall:recover_paying_orders") {
			t.Fatalf("PermissionsByRoleKeys(%q) = %v, want mall:recover_paying_orders", role, permissions)
		}
	}
}

func TestSeedDefaultsDoesNotGrantMenuPermissionFromSystemRoot(t *testing.T) {
	dsn := os.Getenv("BBS_ADMIN_TEST_DSN")
	if dsn == "" {
		t.Skip("set BBS_ADMIN_TEST_DSN to run postgres-backed repository tests")
	}

	ctx := context.Background()
	repo, cleanup := repositoryForProtectedRoleTest(t, ctx, dsn)
	defer cleanup()

	systemRoot := systemMenuByName(t, ctx, repo, "system")
	if systemRoot.Permission != "" {
		t.Fatalf("system root permission = %q, want empty navigation-only permission", systemRoot.Permission)
	}
	systemMenu := systemMenuByName(t, ctx, repo, "system.menu")
	if systemMenu.Permission != "system:list_system_menus" {
		t.Fatalf("system.menu permission = %q, want system:list_system_menus", systemMenu.Permission)
	}

	roleKey := fmt.Sprintf("system_root_only_%d", time.Now().UnixNano())
	role, err := repo.CreateSystemRole(ctx, domain.UpsertSystemRoleCommand{
		Name:   "System Root Only",
		Key:    roleKey,
		Status: "1",
		Sort:   91,
	})
	if err != nil {
		t.Fatalf("CreateSystemRole(system root only) error = %v", err)
	}
	defer func() {
		_ = repo.DeleteSystemRole(ctx, role.ID)
	}()

	assigned, err := repo.AssignSystemRoleMenus(ctx, role.ID, []int64{systemRoot.ID})
	if err != nil {
		t.Fatalf("AssignSystemRoleMenus(system root only) error = %v", err)
	}
	if containsString(assigned.Permissions, "system:list_system_menus") {
		t.Fatalf("root-only role permissions = %v, should not include system:list_system_menus", assigned.Permissions)
	}
}

func containsString(values []string, target string) bool {
	for _, value := range values {
		if value == target {
			return true
		}
	}
	return false
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

func TestRepositoryRejectsDuplicateSystemUserIdentityOnUpdate(t *testing.T) {
	dsn := os.Getenv("BBS_ADMIN_TEST_DSN")
	if dsn == "" {
		t.Skip("set BBS_ADMIN_TEST_DSN to run postgres-backed repository tests")
	}

	ctx := context.Background()
	repo, cleanup := repositoryForProtectedRoleTest(t, ctx, dsn)
	defer cleanup()

	suffix := time.Now().UnixNano()
	userA, err := repo.CreateSystemUser(ctx, domain.UpsertSystemUserCommand{
		Username: fmt.Sprintf("identity_a_%d", suffix),
		Nickname: "Identity A",
		Email:    fmt.Sprintf("identity_a_%d@example.com", suffix),
		Phone:    fmt.Sprintf("13%09d", suffix%1_000_000_000),
		Status:   1,
	}, "hash")
	if err != nil {
		t.Fatalf("CreateSystemUser(userA) error = %v", err)
	}
	userACreated := true
	defer func() {
		if userACreated {
			_ = repo.DeleteSystemUser(ctx, userA.ID)
		}
	}()

	userB, err := repo.CreateSystemUser(ctx, domain.UpsertSystemUserCommand{
		Username: fmt.Sprintf("identity_b_%d", suffix),
		Nickname: "Identity B",
		Email:    fmt.Sprintf("identity_b_%d@example.com", suffix),
		Phone:    fmt.Sprintf("15%09d", suffix%1_000_000_000),
		Status:   1,
	}, "hash")
	if err != nil {
		t.Fatalf("CreateSystemUser(userB) error = %v", err)
	}
	userBCreated := true
	defer func() {
		if userBCreated {
			_ = repo.DeleteSystemUser(ctx, userB.ID)
		}
	}()

	updateB := domain.UpsertSystemUserCommand{
		ID:        userB.ID,
		Username:  userB.Username,
		Nickname:  userB.Nickname,
		Email:     userB.Email,
		Phone:     userB.Phone,
		AvatarURL: userB.AvatarURL,
		Status:    userB.Status,
		DeptID:    userB.DeptID,
		PostID:    userB.PostID,
		RoleIDs:   userB.RoleIDs,
	}
	if _, err := repo.UpdateSystemUser(ctx, updateB); err != nil {
		t.Fatalf("UpdateSystemUser(same identity) error = %v", err)
	}

	duplicateUsername := updateB
	duplicateUsername.Username = strings.ToUpper(userA.Username)
	if _, err := repo.UpdateSystemUser(ctx, duplicateUsername); !errors.Is(err, domain.ErrAdminUserExists) {
		t.Fatalf("UpdateSystemUser(duplicate username) error = %v, want ErrAdminUserExists", err)
	}

	duplicateEmail := updateB
	duplicateEmail.Email = strings.ToUpper(userA.Email)
	if _, err := repo.UpdateSystemUser(ctx, duplicateEmail); !errors.Is(err, domain.ErrAdminUserExists) {
		t.Fatalf("UpdateSystemUser(duplicate email) error = %v, want ErrAdminUserExists", err)
	}

	duplicatePhone := updateB
	duplicatePhone.Phone = userA.Phone
	if _, err := repo.UpdateSystemUser(ctx, duplicatePhone); !errors.Is(err, domain.ErrAdminUserExists) {
		t.Fatalf("UpdateSystemUser(duplicate phone) error = %v, want ErrAdminUserExists", err)
	}

	if err := repo.DeleteSystemUser(ctx, userB.ID); err != nil {
		t.Fatalf("DeleteSystemUser(userB) error = %v", err)
	}
	userBCreated = false
	if err := repo.DeleteSystemUser(ctx, userA.ID); err != nil {
		t.Fatalf("DeleteSystemUser(userA) error = %v", err)
	}
	userACreated = false
}

func TestRepositoryEnsuresSystemUserIdentityIndexes(t *testing.T) {
	dsn := os.Getenv("BBS_ADMIN_TEST_DSN")
	if dsn == "" {
		t.Skip("set BBS_ADMIN_TEST_DSN to run postgres-backed repository tests")
	}

	ctx := context.Background()
	repo, cleanup := repositoryForProtectedRoleTest(t, ctx, dsn)
	defer cleanup()

	if err := repo.EnsureSchema(ctx); err != nil {
		t.Fatalf("EnsureSchema() error = %v", err)
	}
	for _, indexName := range []string{"idx_sys_user_username_lower_unique", "idx_sys_user_email_lower_unique"} {
		if !postgresIndexExists(t, ctx, repo.db, "sys_user", indexName) {
			t.Fatalf("expected postgres index %s to exist", indexName)
		}
	}

	suffix := time.Now().UnixNano()
	user, err := repo.CreateSystemUser(ctx, domain.UpsertSystemUserCommand{
		Username: fmt.Sprintf("indexed_identity_%d", suffix),
		Nickname: "Indexed Identity",
		Email:    fmt.Sprintf("indexed_identity_%d@example.com", suffix),
		Phone:    fmt.Sprintf("16%09d", suffix%1_000_000_000),
		Status:   1,
	}, "hash")
	if err != nil {
		t.Fatalf("CreateSystemUser(indexed) error = %v", err)
	}
	userCreated := true
	defer func() {
		if userCreated {
			_ = repo.DeleteSystemUser(ctx, user.ID)
		}
		_ = repo.db.WithContext(ctx).
			Exec(
				"DELETE FROM sys_user WHERE LOWER(user_name) IN (?, ?)",
				strings.ToLower(user.Username),
				strings.ToLower(fmt.Sprintf("indexed_identity_copy_%d", suffix)),
			).Error
	}()

	err = repo.db.WithContext(ctx).Exec(
		`INSERT INTO sys_user (user_name, phone, email, password, status, role_id, dept_id, post_id, create_time, update_time)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, NOW(), NOW())`,
		strings.ToUpper(user.Username),
		fmt.Sprintf("17%09d", suffix%1_000_000_000),
		fmt.Sprintf("indexed_identity_copy_%d@example.com", suffix),
		"hash",
		1,
		user.RoleID,
		user.DeptID,
		user.PostID,
	).Error
	if !isAdminUserIdentityUniqueViolation(err) {
		t.Fatalf("raw duplicate username insert error = %v, want admin user identity unique violation", err)
	}

	err = repo.db.WithContext(ctx).Exec(
		`INSERT INTO sys_user (user_name, phone, email, password, status, role_id, dept_id, post_id, create_time, update_time)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, NOW(), NOW())`,
		fmt.Sprintf("indexed_identity_copy_%d", suffix),
		fmt.Sprintf("18%09d", suffix%1_000_000_000),
		strings.ToUpper(user.Email),
		"hash",
		1,
		user.RoleID,
		user.DeptID,
		user.PostID,
	).Error
	if !isAdminUserIdentityUniqueViolation(err) {
		t.Fatalf("raw duplicate email insert error = %v, want admin user identity unique violation", err)
	}

	if err := repo.DeleteSystemUser(ctx, user.ID); err != nil {
		t.Fatalf("DeleteSystemUser(indexed) error = %v", err)
	}
	userCreated = false
}

func TestRepositoryEnsuresSystemManagementUniqueIndexes(t *testing.T) {
	dsn := os.Getenv("BBS_ADMIN_TEST_DSN")
	if dsn == "" {
		t.Skip("set BBS_ADMIN_TEST_DSN to run postgres-backed repository tests")
	}

	ctx := context.Background()
	repo, cleanup := repositoryForProtectedRoleTest(t, ctx, dsn)
	defer cleanup()

	if err := repo.EnsureSchema(ctx); err != nil {
		t.Fatalf("EnsureSchema() error = %v", err)
	}
	expectedIndexes := map[string]string{
		"sys_role": "idx_sys_role_key_lower_unique",
		"sys_menu": "idx_sys_menu_name_lower_unique",
		"sys_dept": "idx_sys_dept_parent_name_lower_unique",
	}
	for tableName, indexName := range expectedIndexes {
		if !postgresIndexExists(t, ctx, repo.db, tableName, indexName) {
			t.Fatalf("expected postgres index %s on %s to exist", indexName, tableName)
		}
	}

	suffix := time.Now().UnixNano()
	roleKey := fmt.Sprintf("indexed_role_%d", suffix)
	role, err := repo.CreateSystemRole(ctx, domain.UpsertSystemRoleCommand{
		Name:   "Indexed Role",
		Key:    roleKey,
		Status: "1",
		Sort:   9000,
	})
	if err != nil {
		t.Fatalf("CreateSystemRole(indexed) error = %v", err)
	}
	roleCreated := true
	defer func() {
		_ = repo.db.WithContext(ctx).Exec("DELETE FROM sys_role WHERE LOWER(\"key\") IN (?, ?)", normalize(roleKey), normalize("indexed_role_copy_"+fmt.Sprint(suffix))).Error
		if roleCreated {
			_ = repo.DeleteSystemRole(ctx, role.ID)
		}
	}()

	err = repo.db.WithContext(ctx).Exec(
		`INSERT INTO sys_role (name, key, status, sort, create_time, update_time) VALUES (?, ?, ?, ?, NOW(), NOW())`,
		"Indexed Role Copy",
		strings.ToUpper(roleKey),
		"1",
		9001,
	).Error
	if !errors.Is(systemManagementUniqueViolationError(err), domain.ErrSystemRoleExists) {
		t.Fatalf("raw duplicate role key insert error = %v, want ErrSystemRoleExists", err)
	}

	menuName := fmt.Sprintf("indexed.menu.%d", suffix)
	menu, err := repo.CreateSystemMenu(ctx, domain.UpsertSystemMenuCommand{
		Name:       menuName,
		Title:      "Indexed Menu",
		Path:       fmt.Sprintf("/indexed-menu-%d", suffix),
		Component:  "system/test/index",
		Type:       "C",
		Permission: fmt.Sprintf("system:indexed_menu_%d", suffix),
		Sort:       9002,
	})
	if err != nil {
		t.Fatalf("CreateSystemMenu(indexed) error = %v", err)
	}
	menuCreated := true
	defer func() {
		_ = repo.db.WithContext(ctx).Exec("DELETE FROM sys_menu WHERE LOWER(name) IN (?, ?)", normalize(menuName), normalize("indexed.menu.copy."+fmt.Sprint(suffix))).Error
		if menuCreated {
			_ = repo.DeleteSystemMenu(ctx, menu.ID)
		}
	}()

	err = repo.db.WithContext(ctx).Exec(
		`INSERT INTO sys_menu (name, title, path, component, type, permission, sort, create_time, update_time)
		 VALUES (?, ?, ?, ?, ?, ?, ?, NOW(), NOW())`,
		strings.ToUpper(menuName),
		"Indexed Menu Copy",
		fmt.Sprintf("/indexed-menu-copy-%d", suffix),
		"system/test/index",
		"C",
		fmt.Sprintf("system:indexed_menu_copy_%d", suffix),
		9003,
	).Error
	if !errors.Is(systemManagementUniqueViolationError(err), domain.ErrSystemMenuExists) {
		t.Fatalf("raw duplicate menu name insert error = %v, want ErrSystemMenuExists", err)
	}

	parent, err := repo.CreateSystemDept(ctx, domain.UpsertSystemDeptCommand{
		Name:   fmt.Sprintf("Indexed Parent %d", suffix),
		Sort:   9004,
		Status: 1,
	})
	if err != nil {
		t.Fatalf("CreateSystemDept(parent) error = %v", err)
	}
	parentCreated := true
	defer func() {
		_ = repo.db.WithContext(ctx).Exec("DELETE FROM sys_dept WHERE parent_id = ? AND LOWER(name) IN (?, ?)", parent.ID, normalize("Indexed Dept "+fmt.Sprint(suffix)), normalize("Indexed Dept Copy "+fmt.Sprint(suffix))).Error
		if parentCreated {
			_ = repo.DeleteSystemDept(ctx, parent.ID)
		}
	}()

	deptName := fmt.Sprintf("Indexed Dept %d", suffix)
	dept, err := repo.CreateSystemDept(ctx, domain.UpsertSystemDeptCommand{
		ParentID: parent.ID,
		Name:     deptName,
		Sort:     9005,
		Status:   1,
	})
	if err != nil {
		t.Fatalf("CreateSystemDept(indexed) error = %v", err)
	}
	deptCreated := true
	defer func() {
		if deptCreated {
			_ = repo.DeleteSystemDept(ctx, dept.ID)
		}
	}()

	err = repo.db.WithContext(ctx).Exec(
		`INSERT INTO sys_dept (parent_id, name, sort, status, create_time, update_time) VALUES (?, ?, ?, ?, NOW(), NOW())`,
		parent.ID,
		strings.ToUpper(deptName),
		9006,
		1,
	).Error
	if !errors.Is(systemManagementUniqueViolationError(err), domain.ErrSystemDeptExists) {
		t.Fatalf("raw duplicate dept name insert error = %v, want ErrSystemDeptExists", err)
	}

	if err := repo.DeleteSystemDept(ctx, dept.ID); err != nil {
		t.Fatalf("DeleteSystemDept(indexed) error = %v", err)
	}
	deptCreated = false
	if err := repo.DeleteSystemDept(ctx, parent.ID); err != nil {
		t.Fatalf("DeleteSystemDept(parent) error = %v", err)
	}
	parentCreated = false
	if err := repo.DeleteSystemMenu(ctx, menu.ID); err != nil {
		t.Fatalf("DeleteSystemMenu(indexed) error = %v", err)
	}
	menuCreated = false
	if err := repo.DeleteSystemRole(ctx, role.ID); err != nil {
		t.Fatalf("DeleteSystemRole(indexed) error = %v", err)
	}
	roleCreated = false
}

func TestRepositoryRejectsDeletingSystemRoleWithUsers(t *testing.T) {
	dsn := os.Getenv("BBS_ADMIN_TEST_DSN")
	if dsn == "" {
		t.Skip("set BBS_ADMIN_TEST_DSN to run postgres-backed repository tests")
	}

	ctx := context.Background()
	repo, cleanup := repositoryForProtectedRoleTest(t, ctx, dsn)
	defer cleanup()

	suffix := time.Now().UnixNano()
	role, err := repo.CreateSystemRole(ctx, domain.UpsertSystemRoleCommand{
		Name:   "Assigned Role",
		Key:    fmt.Sprintf("assigned_role_%d", suffix),
		Status: "1",
		Sort:   90,
	})
	if err != nil {
		t.Fatalf("CreateSystemRole(assigned) error = %v", err)
	}
	roleCreated := true
	defer func() {
		if roleCreated {
			_ = repo.DeleteSystemRole(ctx, role.ID)
		}
	}()

	user, err := repo.CreateSystemUser(ctx, domain.UpsertSystemUserCommand{
		Username: fmt.Sprintf("assigned_role_user_%d", suffix),
		Nickname: "Assigned Role User",
		Email:    fmt.Sprintf("assigned_role_user_%d@example.com", suffix),
		Status:   1,
		RoleIDs:  []int64{role.ID},
	}, "hash")
	if err != nil {
		t.Fatalf("CreateSystemUser(assigned role user) error = %v", err)
	}
	userCreated := true
	defer func() {
		if userCreated {
			_ = repo.DeleteSystemUser(ctx, user.ID)
		}
	}()

	if err := repo.DeleteSystemRole(ctx, role.ID); !errors.Is(err, domain.ErrSystemRoleHasUsers) {
		t.Fatalf("DeleteSystemRole(assigned) error = %v, want ErrSystemRoleHasUsers", err)
	}
	if err := repo.DeleteSystemUser(ctx, user.ID); err != nil {
		t.Fatalf("DeleteSystemUser(assigned role user) error = %v", err)
	}
	userCreated = false
	if err := repo.DeleteSystemRole(ctx, role.ID); err != nil {
		t.Fatalf("DeleteSystemRole(after user delete) error = %v", err)
	}
	roleCreated = false
}

func TestRepositoryRejectsDuplicateSystemRoleKeys(t *testing.T) {
	dsn := os.Getenv("BBS_ADMIN_TEST_DSN")
	if dsn == "" {
		t.Skip("set BBS_ADMIN_TEST_DSN to run postgres-backed repository tests")
	}

	ctx := context.Background()
	repo, cleanup := repositoryForProtectedRoleTest(t, ctx, dsn)
	defer cleanup()

	suffix := time.Now().UnixNano()
	roleKey := fmt.Sprintf("duplicate_role_%d", suffix)
	role, err := repo.CreateSystemRole(ctx, domain.UpsertSystemRoleCommand{Name: "Duplicate Role", Key: roleKey, Status: "1", Sort: 90})
	if err != nil {
		t.Fatalf("CreateSystemRole(role) error = %v", err)
	}
	roleCreated := true
	defer func() {
		if roleCreated {
			_ = repo.DeleteSystemRole(ctx, role.ID)
		}
	}()

	if _, err := repo.CreateSystemRole(ctx, domain.UpsertSystemRoleCommand{Name: "Duplicate Role Copy", Key: strings.ToUpper(roleKey), Status: "1", Sort: 91}); !errors.Is(err, domain.ErrSystemRoleExists) {
		t.Fatalf("CreateSystemRole(duplicate key) error = %v, want ErrSystemRoleExists", err)
	}

	other, err := repo.CreateSystemRole(ctx, domain.UpsertSystemRoleCommand{Name: "Other Role", Key: fmt.Sprintf("other_role_%d", suffix), Status: "1", Sort: 92})
	if err != nil {
		t.Fatalf("CreateSystemRole(other) error = %v", err)
	}
	otherCreated := true
	defer func() {
		if otherCreated {
			_ = repo.DeleteSystemRole(ctx, other.ID)
		}
	}()
	other.Key = roleKey
	if _, err := repo.UpdateSystemRole(ctx, domain.UpsertSystemRoleCommand{ID: other.ID, Name: other.Name, Key: other.Key, Status: other.Status, Sort: other.Sort}); !errors.Is(err, domain.ErrSystemRoleExists) {
		t.Fatalf("UpdateSystemRole(duplicate key) error = %v, want ErrSystemRoleExists", err)
	}

	if err := repo.DeleteSystemRole(ctx, other.ID); err != nil {
		t.Fatalf("DeleteSystemRole(other) error = %v", err)
	}
	otherCreated = false
	if err := repo.DeleteSystemRole(ctx, role.ID); err != nil {
		t.Fatalf("DeleteSystemRole(role) error = %v", err)
	}
	roleCreated = false
}

func TestRepositoryRejectsDuplicateSystemMenuNames(t *testing.T) {
	dsn := os.Getenv("BBS_ADMIN_TEST_DSN")
	if dsn == "" {
		t.Skip("set BBS_ADMIN_TEST_DSN to run postgres-backed repository tests")
	}

	ctx := context.Background()
	repo, cleanup := repositoryForProtectedRoleTest(t, ctx, dsn)
	defer cleanup()

	suffix := time.Now().UnixNano()
	menuName := fmt.Sprintf("test.duplicate.menu.%d", suffix)
	menu, err := repo.CreateSystemMenu(ctx, domain.UpsertSystemMenuCommand{
		Name:       menuName,
		Title:      "Duplicate Menu",
		Path:       fmt.Sprintf("/duplicate-menu-%d", suffix),
		Component:  "system/test/index",
		Type:       "C",
		Permission: fmt.Sprintf("system:duplicate_menu_%d", suffix),
		Sort:       9999,
	})
	if err != nil {
		t.Fatalf("CreateSystemMenu(menu) error = %v", err)
	}
	menuCreated := true
	defer func() {
		if menuCreated {
			_ = repo.DeleteSystemMenu(ctx, menu.ID)
		}
	}()

	if _, err := repo.CreateSystemMenu(ctx, domain.UpsertSystemMenuCommand{
		Name:       strings.ToUpper(menuName),
		Title:      "Duplicate Menu Copy",
		Path:       fmt.Sprintf("/duplicate-menu-copy-%d", suffix),
		Component:  "system/test/index",
		Type:       "C",
		Permission: fmt.Sprintf("system:duplicate_menu_copy_%d", suffix),
		Sort:       10000,
	}); !errors.Is(err, domain.ErrSystemMenuExists) {
		t.Fatalf("CreateSystemMenu(duplicate name) error = %v, want ErrSystemMenuExists", err)
	}

	other, err := repo.CreateSystemMenu(ctx, domain.UpsertSystemMenuCommand{
		Name:       fmt.Sprintf("test.other.menu.%d", suffix),
		Title:      "Other Menu",
		Path:       fmt.Sprintf("/other-menu-%d", suffix),
		Component:  "system/test/index",
		Type:       "C",
		Permission: fmt.Sprintf("system:other_menu_%d", suffix),
		Sort:       10001,
	})
	if err != nil {
		t.Fatalf("CreateSystemMenu(other) error = %v", err)
	}
	otherCreated := true
	defer func() {
		if otherCreated {
			_ = repo.DeleteSystemMenu(ctx, other.ID)
		}
	}()
	other.Name = menuName
	if _, err := repo.UpdateSystemMenu(ctx, domain.UpsertSystemMenuCommand{
		ID:         other.ID,
		ParentID:   other.ParentID,
		Name:       other.Name,
		Title:      other.Title,
		Path:       other.Path,
		Component:  other.Component,
		Type:       other.Type,
		Permission: other.Permission,
		Status:     other.Status,
		Visible:    other.Visible,
		IsHide:     other.IsHide,
		Sort:       other.Sort,
		Remark:     other.Remark,
	}); !errors.Is(err, domain.ErrSystemMenuExists) {
		t.Fatalf("UpdateSystemMenu(duplicate name) error = %v, want ErrSystemMenuExists", err)
	}

	if err := repo.DeleteSystemMenu(ctx, other.ID); err != nil {
		t.Fatalf("DeleteSystemMenu(other) error = %v", err)
	}
	otherCreated = false
	if err := repo.DeleteSystemMenu(ctx, menu.ID); err != nil {
		t.Fatalf("DeleteSystemMenu(menu) error = %v", err)
	}
	menuCreated = false
}

func TestRepositoryRejectsDuplicateSiblingSystemDeptNames(t *testing.T) {
	dsn := os.Getenv("BBS_ADMIN_TEST_DSN")
	if dsn == "" {
		t.Skip("set BBS_ADMIN_TEST_DSN to run postgres-backed repository tests")
	}

	ctx := context.Background()
	repo, cleanup := repositoryForProtectedRoleTest(t, ctx, dsn)
	defer cleanup()

	suffix := time.Now().UnixNano()
	parentA, err := repo.CreateSystemDept(ctx, domain.UpsertSystemDeptCommand{Name: fmt.Sprintf("Duplicate Parent A %d", suffix), Sort: 9999, Status: 1})
	if err != nil {
		t.Fatalf("CreateSystemDept(parentA) error = %v", err)
	}
	parentACreated := true
	defer func() {
		if parentACreated {
			_ = repo.DeleteSystemDept(ctx, parentA.ID)
		}
	}()
	parentB, err := repo.CreateSystemDept(ctx, domain.UpsertSystemDeptCommand{Name: fmt.Sprintf("Duplicate Parent B %d", suffix), Sort: 10000, Status: 1})
	if err != nil {
		t.Fatalf("CreateSystemDept(parentB) error = %v", err)
	}
	parentBCreated := true
	defer func() {
		if parentBCreated {
			_ = repo.DeleteSystemDept(ctx, parentB.ID)
		}
	}()

	deptName := fmt.Sprintf("Duplicate Dept %d", suffix)
	dept, err := repo.CreateSystemDept(ctx, domain.UpsertSystemDeptCommand{ParentID: parentA.ID, Name: deptName, Sort: 10001, Status: 1})
	if err != nil {
		t.Fatalf("CreateSystemDept(dept) error = %v", err)
	}
	deptCreated := true
	defer func() {
		if deptCreated {
			_ = repo.DeleteSystemDept(ctx, dept.ID)
		}
	}()

	if _, err := repo.CreateSystemDept(ctx, domain.UpsertSystemDeptCommand{ParentID: parentA.ID, Name: strings.ToUpper(deptName), Sort: 10002, Status: 1}); !errors.Is(err, domain.ErrSystemDeptExists) {
		t.Fatalf("CreateSystemDept(duplicate sibling) error = %v, want ErrSystemDeptExists", err)
	}

	other, err := repo.CreateSystemDept(ctx, domain.UpsertSystemDeptCommand{ParentID: parentB.ID, Name: deptName, Sort: 10003, Status: 1})
	if err != nil {
		t.Fatalf("CreateSystemDept(same name different parent) error = %v", err)
	}
	otherCreated := true
	defer func() {
		if otherCreated {
			_ = repo.DeleteSystemDept(ctx, other.ID)
		}
	}()
	if _, err := repo.UpdateSystemDept(ctx, domain.UpsertSystemDeptCommand{ID: other.ID, ParentID: parentA.ID, Name: other.Name, Sort: other.Sort, Status: other.Status}); !errors.Is(err, domain.ErrSystemDeptExists) {
		t.Fatalf("UpdateSystemDept(duplicate sibling) error = %v, want ErrSystemDeptExists", err)
	}

	if err := repo.DeleteSystemDept(ctx, other.ID); err != nil {
		t.Fatalf("DeleteSystemDept(other) error = %v", err)
	}
	otherCreated = false
	if err := repo.DeleteSystemDept(ctx, dept.ID); err != nil {
		t.Fatalf("DeleteSystemDept(dept) error = %v", err)
	}
	deptCreated = false
	if err := repo.DeleteSystemDept(ctx, parentB.ID); err != nil {
		t.Fatalf("DeleteSystemDept(parentB) error = %v", err)
	}
	parentBCreated = false
	if err := repo.DeleteSystemDept(ctx, parentA.ID); err != nil {
		t.Fatalf("DeleteSystemDept(parentA) error = %v", err)
	}
	parentACreated = false
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

func TestRepositoryRejectsInvalidSystemMenuParents(t *testing.T) {
	dsn := os.Getenv("BBS_ADMIN_TEST_DSN")
	if dsn == "" {
		t.Skip("set BBS_ADMIN_TEST_DSN to run postgres-backed repository tests")
	}

	ctx := context.Background()
	repo, cleanup := repositoryForProtectedRoleTest(t, ctx, dsn)
	defer cleanup()

	suffix := time.Now().UnixNano()
	if _, err := repo.CreateSystemMenu(ctx, domain.UpsertSystemMenuCommand{
		ParentID:   987654321000 + suffix%100000,
		Name:       fmt.Sprintf("test.missing.parent.%d", suffix),
		Title:      "Missing Parent",
		Path:       fmt.Sprintf("/missing-parent-%d", suffix),
		Type:       "C",
		Permission: fmt.Sprintf("system:missing_parent_%d", suffix),
		Sort:       9999,
	}); !errors.Is(err, domain.ErrSystemMenuParentNotFound) {
		t.Fatalf("CreateSystemMenu(missing parent) error = %v, want ErrSystemMenuParentNotFound", err)
	}

	parent, err := repo.CreateSystemMenu(ctx, domain.UpsertSystemMenuCommand{
		Name:       fmt.Sprintf("test.invalid.parent.%d", suffix),
		Title:      "Invalid Parent",
		Path:       fmt.Sprintf("/invalid-parent-%d", suffix),
		Type:       "M",
		Permission: fmt.Sprintf("system:invalid_parent_%d", suffix),
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
		Name:       fmt.Sprintf("test.invalid.child.%d", suffix),
		Title:      "Invalid Child",
		Path:       fmt.Sprintf("/invalid-child-%d", suffix),
		Component:  "system/test/index",
		Type:       "C",
		Permission: fmt.Sprintf("system:invalid_child_%d", suffix),
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

	updateParent := domain.UpsertSystemMenuCommand{
		ID:         parent.ID,
		ParentID:   parent.ID,
		Name:       parent.Name,
		Title:      parent.Title,
		Path:       parent.Path,
		Component:  parent.Component,
		Type:       parent.Type,
		Permission: parent.Permission,
		Status:     parent.Status,
		Visible:    parent.Visible,
		IsHide:     parent.IsHide,
		Sort:       parent.Sort,
		Remark:     parent.Remark,
	}
	if _, err := repo.UpdateSystemMenu(ctx, updateParent); !errors.Is(err, domain.ErrSystemMenuInvalidParent) {
		t.Fatalf("UpdateSystemMenu(self parent) error = %v, want ErrSystemMenuInvalidParent", err)
	}
	updateParent.ParentID = child.ID
	if _, err := repo.UpdateSystemMenu(ctx, updateParent); !errors.Is(err, domain.ErrSystemMenuInvalidParent) {
		t.Fatalf("UpdateSystemMenu(descendant parent) error = %v, want ErrSystemMenuInvalidParent", err)
	}

	if err := repo.DeleteSystemMenu(ctx, child.ID); err != nil {
		t.Fatalf("DeleteSystemMenu(child) error = %v", err)
	}
	childCreated = false
	if err := repo.DeleteSystemMenu(ctx, parent.ID); err != nil {
		t.Fatalf("DeleteSystemMenu(parent) error = %v", err)
	}
	parentCreated = false
}

func TestRepositoryRejectsInvalidSystemDeptParents(t *testing.T) {
	dsn := os.Getenv("BBS_ADMIN_TEST_DSN")
	if dsn == "" {
		t.Skip("set BBS_ADMIN_TEST_DSN to run postgres-backed repository tests")
	}

	ctx := context.Background()
	repo, cleanup := repositoryForProtectedRoleTest(t, ctx, dsn)
	defer cleanup()

	suffix := time.Now().UnixNano()
	if _, err := repo.CreateSystemDept(ctx, domain.UpsertSystemDeptCommand{
		ParentID: 987654321000 + suffix%100000,
		Name:     fmt.Sprintf("Missing Parent Dept %d", suffix),
		Sort:     9999,
		Status:   1,
	}); !errors.Is(err, domain.ErrSystemDeptParentNotFound) {
		t.Fatalf("CreateSystemDept(missing parent) error = %v, want ErrSystemDeptParentNotFound", err)
	}

	parent, err := repo.CreateSystemDept(ctx, domain.UpsertSystemDeptCommand{
		Name:   fmt.Sprintf("Invalid Parent Dept %d", suffix),
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
		Name:     fmt.Sprintf("Invalid Child Dept %d", suffix),
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

	updateParent := domain.UpsertSystemDeptCommand{
		ID:       parent.ID,
		ParentID: parent.ID,
		Name:     parent.Name,
		Sort:     parent.Sort,
		Leader:   parent.Leader,
		Phone:    parent.Phone,
		Email:    parent.Email,
		Status:   parent.Status,
	}
	if _, err := repo.UpdateSystemDept(ctx, updateParent); !errors.Is(err, domain.ErrSystemDeptInvalidParent) {
		t.Fatalf("UpdateSystemDept(self parent) error = %v, want ErrSystemDeptInvalidParent", err)
	}
	updateParent.ParentID = child.ID
	if _, err := repo.UpdateSystemDept(ctx, updateParent); !errors.Is(err, domain.ErrSystemDeptInvalidParent) {
		t.Fatalf("UpdateSystemDept(descendant parent) error = %v, want ErrSystemDeptInvalidParent", err)
	}

	if err := repo.DeleteSystemDept(ctx, child.ID); err != nil {
		t.Fatalf("DeleteSystemDept(child) error = %v", err)
	}
	childCreated = false
	if err := repo.DeleteSystemDept(ctx, parent.ID); err != nil {
		t.Fatalf("DeleteSystemDept(parent) error = %v", err)
	}
	parentCreated = false
}

func TestRepositoryUpdatesSystemDeptSubtreePathOnMove(t *testing.T) {
	dsn := os.Getenv("BBS_ADMIN_TEST_DSN")
	if dsn == "" {
		t.Skip("set BBS_ADMIN_TEST_DSN to run postgres-backed repository tests")
	}

	ctx := context.Background()
	repo, cleanup := repositoryForProtectedRoleTest(t, ctx, dsn)
	defer cleanup()

	suffix := time.Now().UnixNano()
	var rootA, rootB, child, grandchild domain.SystemDept
	defer func() {
		if grandchild.ID > 0 {
			_ = repo.DeleteSystemDept(ctx, grandchild.ID)
		}
		if child.ID > 0 {
			_ = repo.DeleteSystemDept(ctx, child.ID)
		}
		if rootB.ID > 0 {
			_ = repo.DeleteSystemDept(ctx, rootB.ID)
		}
		if rootA.ID > 0 {
			_ = repo.DeleteSystemDept(ctx, rootA.ID)
		}
	}()

	var err error
	rootA, err = repo.CreateSystemDept(ctx, domain.UpsertSystemDeptCommand{Name: fmt.Sprintf("Move Root A %d", suffix), Sort: 9999, Status: 1})
	if err != nil {
		t.Fatalf("CreateSystemDept(rootA) error = %v", err)
	}
	rootB, err = repo.CreateSystemDept(ctx, domain.UpsertSystemDeptCommand{Name: fmt.Sprintf("Move Root B %d", suffix), Sort: 10000, Status: 1})
	if err != nil {
		t.Fatalf("CreateSystemDept(rootB) error = %v", err)
	}
	child, err = repo.CreateSystemDept(ctx, domain.UpsertSystemDeptCommand{ParentID: rootA.ID, Name: fmt.Sprintf("Move Child %d", suffix), Sort: 10001, Status: 1})
	if err != nil {
		t.Fatalf("CreateSystemDept(child) error = %v", err)
	}
	grandchild, err = repo.CreateSystemDept(ctx, domain.UpsertSystemDeptCommand{ParentID: child.ID, Name: fmt.Sprintf("Move Grandchild %d", suffix), Sort: 10002, Status: 1})
	if err != nil {
		t.Fatalf("CreateSystemDept(grandchild) error = %v", err)
	}

	moved, err := repo.UpdateSystemDept(ctx, domain.UpsertSystemDeptCommand{
		ID:       child.ID,
		ParentID: rootB.ID,
		Name:     child.Name,
		Sort:     child.Sort,
		Leader:   child.Leader,
		Phone:    child.Phone,
		Email:    child.Email,
		Status:   child.Status,
	})
	if err != nil {
		t.Fatalf("UpdateSystemDept(move child) error = %v", err)
	}

	gotGrandchild := systemDeptByID(t, ctx, repo, grandchild.ID)
	wantChildPath := rootB.Path + "/" + fmt.Sprint(child.ID)
	wantGrandchildPath := wantChildPath + "/" + fmt.Sprint(grandchild.ID)
	if moved.Path != wantChildPath {
		t.Fatalf("moved child path = %q, want %q", moved.Path, wantChildPath)
	}
	if gotGrandchild.Path != wantGrandchildPath {
		t.Fatalf("grandchild path = %q, want %q", gotGrandchild.Path, wantGrandchildPath)
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

func systemMenuByName(t *testing.T, ctx context.Context, repo *Repository, name string) domain.SystemMenu {
	t.Helper()
	menus, err := repo.ListSystemMenus(ctx, "", "")
	if err != nil {
		t.Fatalf("ListSystemMenus() error = %v", err)
	}
	for _, menu := range menus.Items {
		if menu.Name == name {
			return menu
		}
	}
	t.Fatalf("menu %q not found in %#v", name, menus.Items)
	return domain.SystemMenu{}
}

func postgresIndexExists(t *testing.T, ctx context.Context, db *gorm.DB, tableName string, indexName string) bool {
	t.Helper()
	var count int64
	if err := db.WithContext(ctx).Raw(
		"SELECT COUNT(*) FROM pg_indexes WHERE schemaname = current_schema() AND tablename = ? AND indexname = ?",
		tableName,
		indexName,
	).Scan(&count).Error; err != nil {
		t.Fatalf("query postgres index %s: %v", indexName, err)
	}
	return count > 0
}

func systemDeptByID(t *testing.T, ctx context.Context, repo *Repository, id int64) domain.SystemDept {
	t.Helper()
	depts, err := repo.ListSystemDepts(ctx, "", 0)
	if err != nil {
		t.Fatalf("ListSystemDepts() error = %v", err)
	}
	for _, dept := range depts.Items {
		if dept.ID == id {
			return dept
		}
	}
	t.Fatalf("dept %d not found in %#v", id, depts.Items)
	return domain.SystemDept{}
}
