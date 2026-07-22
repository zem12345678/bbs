package persistence

import (
	"context"
	"errors"
	"fmt"
	"os"
	"testing"
	"time"

	domain "admin/internal/domain/admin"

	"gorm.io/gorm"
	gormlogger "gorm.io/gorm/logger"
)

func TestRepositoryListEndpointsAvoidNPlusOneRoleQueries(t *testing.T) {
	dsn := os.Getenv("BBS_ADMIN_TEST_DSN")
	if dsn == "" {
		t.Skip("set BBS_ADMIN_TEST_DSN to run postgres-backed repository tests")
	}

	ctx := context.Background()
	repo, cleanup := repositoryForTemporarySchemaTest(t, ctx, dsn)
	defer cleanup()

	suffix := time.Now().UnixNano()
	usernames := make([]string, 0, 3)
	for index := 0; index < 3; index++ {
		username := fmt.Sprintf("batch_list_user_%d_%d", suffix, index)
		usernames = append(usernames, username)
		if _, err := repo.CreateSystemUser(ctx, domain.UpsertSystemUserCommand{
			Username: username,
			Email:    username + "@example.com",
			Status:   1,
		}, "hash"); err != nil {
			t.Fatalf("CreateSystemUser(%q) error = %v", username, err)
		}
	}

	queryCounter := &repositoryQueryCounter{}
	countedRepo := NewRepository(repo.db.Session(&gorm.Session{Logger: queryCounter}))

	queryCounter.Reset()
	systemUsers, err := countedRepo.ListSystemUsers(ctx, "", 0, 1, 100)
	if err != nil {
		t.Fatalf("ListSystemUsers() error = %v", err)
	}
	if queryCounter.Count() != 3 {
		t.Fatalf("ListSystemUsers() executed %d queries, want 3", queryCounter.Count())
	}
	if len(systemUsers.Items) < 4 {
		t.Fatalf("ListSystemUsers() returned %d users, want seeded user plus test users", len(systemUsers.Items))
	}
	for _, username := range usernames {
		found := false
		for _, user := range systemUsers.Items {
			if user.Username != username {
				continue
			}
			found = true
			if !containsListTestString(user.Roles, "user") {
				t.Fatalf("ListSystemUsers() roles for %q = %v, want user", username, user.Roles)
			}
		}
		if !found {
			t.Fatalf("ListSystemUsers() did not return %q", username)
		}
	}

	queryCounter.Reset()
	systemRoles, err := countedRepo.ListSystemRoles(ctx, "", "", 1, 100)
	if err != nil {
		t.Fatalf("ListSystemRoles() error = %v", err)
	}
	if queryCounter.Count() != 4 {
		t.Fatalf("ListSystemRoles() executed %d queries, want 4", queryCounter.Count())
	}
	if len(systemRoles.Items) == 0 {
		t.Fatal("ListSystemRoles() returned no seeded roles")
	}
	var systemAdminRole *domain.SystemRole
	for index := range systemRoles.Items {
		if systemRoles.Items[index].Key == "admin" {
			systemAdminRole = &systemRoles.Items[index]
			break
		}
	}
	if systemAdminRole == nil || len(systemAdminRole.MenuIDs) == 0 || !containsListTestString(systemAdminRole.Permissions, "system:view_dashboard") {
		t.Fatalf("ListSystemRoles() admin role = %#v, want menu ids and system:view_dashboard", systemAdminRole)
	}

	queryCounter.Reset()
	adminUsers, err := countedRepo.ListAdminUsers(ctx, "", 100, 0)
	if err != nil {
		t.Fatalf("ListAdminUsers() error = %v", err)
	}
	if queryCounter.Count() != 4 {
		t.Fatalf("ListAdminUsers() executed %d queries, want 4", queryCounter.Count())
	}
	if len(adminUsers.Items) < 4 {
		t.Fatalf("ListAdminUsers() returned %d users, want seeded user plus test users", len(adminUsers.Items))
	}
	for _, username := range usernames {
		found := false
		for _, user := range adminUsers.Items {
			if user.Username != username {
				continue
			}
			found = true
			if !containsListTestString(user.Roles, "user") {
				t.Fatalf("ListAdminUsers() roles for %q = %v, want user", username, user.Roles)
			}
		}
		if !found {
			t.Fatalf("ListAdminUsers() did not return %q", username)
		}
	}

	queryCounter.Reset()
	roles, err := countedRepo.ListRoles(ctx)
	if err != nil {
		t.Fatalf("ListRoles() error = %v", err)
	}
	if queryCounter.Count() != 2 {
		t.Fatalf("ListRoles() executed %d queries, want 2", queryCounter.Count())
	}
	if len(roles.Items) == 0 {
		t.Fatal("ListRoles() returned no seeded roles")
	}
	var adminRole *domain.Role
	for index := range roles.Items {
		if roles.Items[index].Key == "admin" {
			adminRole = &roles.Items[index]
			break
		}
	}
	if adminRole == nil || !containsListTestString(adminRole.Permissions, "system:view_dashboard") {
		t.Fatalf("ListRoles() admin role = %#v, want system:view_dashboard", adminRole)
	}
}

func TestRepositoryListsCurrentSystemMenuAncestorsWithoutDepthQueries(t *testing.T) {
	dsn := os.Getenv("BBS_ADMIN_TEST_DSN")
	if dsn == "" {
		t.Skip("set BBS_ADMIN_TEST_DSN to run postgres-backed repository tests")
	}

	ctx := context.Background()
	repo, cleanup := repositoryForTemporarySchemaTest(t, ctx, dsn)
	defer cleanup()

	suffix := time.Now().UnixNano()
	role, err := repo.CreateSystemRole(ctx, domain.UpsertSystemRoleCommand{
		Name:   "Current Menu Tree Role",
		Key:    fmt.Sprintf("current_menu_tree_role_%d", suffix),
		Status: "1",
		Sort:   950,
	})
	if err != nil {
		t.Fatalf("CreateSystemRole() error = %v", err)
	}

	menus := make([]domain.SystemMenu, 0, 4)
	parentID := int64(0)
	for index := 0; index < 4; index++ {
		menu, err := repo.CreateSystemMenu(ctx, domain.UpsertSystemMenuCommand{
			ParentID:   parentID,
			Name:       fmt.Sprintf("current.menu.depth.%d.%d", suffix, index),
			Title:      fmt.Sprintf("Current Menu Depth %d", index),
			Path:       fmt.Sprintf("/current-menu-depth-%d-%d", suffix, index),
			Component:  "system/current-menu/index",
			Type:       "C",
			Permission: fmt.Sprintf("system:current_menu_depth_%d_%d", suffix, index),
			Status:     "0",
			Sort:       int32(960 + index),
		})
		if err != nil {
			t.Fatalf("CreateSystemMenu(depth=%d) error = %v", index, err)
		}
		menus = append(menus, menu)
		parentID = menu.ID
	}

	if _, err := repo.AssignSystemRoleMenus(ctx, role.ID, []int64{menus[len(menus)-1].ID}); err != nil {
		t.Fatalf("AssignSystemRoleMenus() error = %v", err)
	}
	user, err := repo.CreateSystemUser(ctx, domain.UpsertSystemUserCommand{
		Username: fmt.Sprintf("current_menu_tree_user_%d", suffix),
		Email:    fmt.Sprintf("current_menu_tree_user_%d@example.com", suffix),
		Status:   1,
		RoleIDs:  []int64{role.ID},
	}, "hash")
	if err != nil {
		t.Fatalf("CreateSystemUser() error = %v", err)
	}

	queryCounter := &repositoryQueryCounter{}
	countedRepo := NewRepository(repo.db.Session(&gorm.Session{Logger: queryCounter}))
	queryCounter.Reset()
	currentMenus, err := countedRepo.ListCurrentSystemMenus(ctx, user.ID)
	if err != nil {
		t.Fatalf("ListCurrentSystemMenus() error = %v", err)
	}
	if queryCounter.Count() != 2 {
		t.Fatalf("ListCurrentSystemMenus() executed %d queries, want 2 regardless of menu depth", queryCounter.Count())
	}
	if len(currentMenus.Items) != len(menus) {
		t.Fatalf("ListCurrentSystemMenus() returned %d menus, want %d", len(currentMenus.Items), len(menus))
	}
	for index, menu := range menus {
		if currentMenus.Items[index].ID != menu.ID {
			t.Fatalf("ListCurrentSystemMenus() item %d id = %d, want %d", index, currentMenus.Items[index].ID, menu.ID)
		}
	}

	allMenuIDs := make([]int64, 0, len(menus))
	for _, menu := range menus {
		allMenuIDs = append(allMenuIDs, menu.ID)
	}
	if _, err := repo.AssignSystemRoleMenus(ctx, role.ID, allMenuIDs); err != nil {
		t.Fatalf("AssignSystemRoleMenus(all menus) error = %v", err)
	}
	queryCounter.Reset()
	if _, err := countedRepo.ListCurrentSystemMenus(ctx, user.ID); err != nil {
		t.Fatalf("ListCurrentSystemMenus(all menus) error = %v", err)
	}
	if queryCounter.Count() != 1 {
		t.Fatalf("ListCurrentSystemMenus(all menus) executed %d queries, want 1", queryCounter.Count())
	}
}

func TestRepositoryValidatesDeepTreeParentsWithoutDepthQueries(t *testing.T) {
	dsn := os.Getenv("BBS_ADMIN_TEST_DSN")
	if dsn == "" {
		t.Skip("set BBS_ADMIN_TEST_DSN to run postgres-backed repository tests")
	}

	ctx := context.Background()
	repo, cleanup := repositoryForTemporarySchemaTest(t, ctx, dsn)
	defer cleanup()

	suffix := time.Now().UnixNano()
	menuParentID := int64(0)
	menuRootID := int64(0)
	for index := 0; index < 4; index++ {
		menu, err := repo.CreateSystemMenu(ctx, domain.UpsertSystemMenuCommand{
			ParentID:   menuParentID,
			Name:       fmt.Sprintf("parent.validation.menu.%d.%d", suffix, index),
			Title:      fmt.Sprintf("Parent Validation Menu %d", index),
			Path:       fmt.Sprintf("/parent-validation-menu-%d-%d", suffix, index),
			Component:  "system/parent-validation/index",
			Type:       "C",
			Permission: fmt.Sprintf("system:parent_validation_menu_%d_%d", suffix, index),
			Sort:       int32(970 + index),
		})
		if err != nil {
			t.Fatalf("CreateSystemMenu(depth=%d) error = %v", index, err)
		}
		if index == 0 {
			menuRootID = menu.ID
		}
		menuParentID = menu.ID
	}

	deptParentID := int64(0)
	deptRootID := int64(0)
	for index := 0; index < 4; index++ {
		dept, err := repo.CreateSystemDept(ctx, domain.UpsertSystemDeptCommand{
			ParentID: deptParentID,
			Name:     fmt.Sprintf("Parent Validation Dept %d %d", suffix, index),
			Status:   1,
			Sort:     int32(980 + index),
		})
		if err != nil {
			t.Fatalf("CreateSystemDept(depth=%d) error = %v", index, err)
		}
		if index == 0 {
			deptRootID = dept.ID
		}
		deptParentID = dept.ID
	}

	queryCounter := &repositoryQueryCounter{}
	countedDB := repo.db.Session(&gorm.Session{Logger: queryCounter})
	queryCounter.Reset()
	if err := validateSystemMenuParent(ctx, countedDB, 0, menuParentID); err != nil {
		t.Fatalf("validateSystemMenuParent() error = %v", err)
	}
	if queryCounter.Count() != 1 {
		t.Fatalf("validateSystemMenuParent() executed %d queries, want 1 regardless of tree depth", queryCounter.Count())
	}

	queryCounter.Reset()
	if err := validateSystemDeptParent(ctx, countedDB, 0, deptParentID); err != nil {
		t.Fatalf("validateSystemDeptParent() error = %v", err)
	}
	if queryCounter.Count() != 1 {
		t.Fatalf("validateSystemDeptParent() executed %d queries, want 1 regardless of tree depth", queryCounter.Count())
	}

	queryCounter.Reset()
	if err := validateSystemMenuParent(ctx, countedDB, menuRootID, menuParentID); !errors.Is(err, domain.ErrSystemMenuInvalidParent) {
		t.Fatalf("validateSystemMenuParent(descendant) error = %v, want ErrSystemMenuInvalidParent", err)
	}
	if queryCounter.Count() != 1 {
		t.Fatalf("validateSystemMenuParent(descendant) executed %d queries, want 1", queryCounter.Count())
	}

	queryCounter.Reset()
	if err := validateSystemDeptParent(ctx, countedDB, deptRootID, deptParentID); !errors.Is(err, domain.ErrSystemDeptInvalidParent) {
		t.Fatalf("validateSystemDeptParent(descendant) error = %v, want ErrSystemDeptInvalidParent", err)
	}
	if queryCounter.Count() != 1 {
		t.Fatalf("validateSystemDeptParent(descendant) executed %d queries, want 1", queryCounter.Count())
	}

	missingParentID := int64(9_000_000_000 + suffix%1_000_000)
	queryCounter.Reset()
	if err := validateSystemMenuParent(ctx, countedDB, 0, missingParentID); !errors.Is(err, domain.ErrSystemMenuParentNotFound) {
		t.Fatalf("validateSystemMenuParent(missing) error = %v, want ErrSystemMenuParentNotFound", err)
	}
	if queryCounter.Count() != 1 {
		t.Fatalf("validateSystemMenuParent(missing) executed %d queries, want 1", queryCounter.Count())
	}
	queryCounter.Reset()
	if err := validateSystemDeptParent(ctx, countedDB, 0, missingParentID); !errors.Is(err, domain.ErrSystemDeptParentNotFound) {
		t.Fatalf("validateSystemDeptParent(missing) error = %v, want ErrSystemDeptParentNotFound", err)
	}
	if queryCounter.Count() != 1 {
		t.Fatalf("validateSystemDeptParent(missing) executed %d queries, want 1", queryCounter.Count())
	}

	if err := repo.db.WithContext(ctx).Table("sys_menu").Where("id = ?", menuRootID).Update("parent_id", menuParentID).Error; err != nil {
		t.Fatalf("create cyclic system menu data error = %v", err)
	}
	queryCounter.Reset()
	if err := validateSystemMenuParent(ctx, countedDB, 0, menuParentID); !errors.Is(err, domain.ErrSystemMenuInvalidParent) {
		t.Fatalf("validateSystemMenuParent(cycle) error = %v, want ErrSystemMenuInvalidParent", err)
	}
	if queryCounter.Count() != 1 {
		t.Fatalf("validateSystemMenuParent(cycle) executed %d queries, want 1", queryCounter.Count())
	}
}

func TestRepositoryBatchWritesRoleRelations(t *testing.T) {
	dsn := os.Getenv("BBS_ADMIN_TEST_DSN")
	if dsn == "" {
		t.Skip("set BBS_ADMIN_TEST_DSN to run postgres-backed repository tests")
	}

	ctx := context.Background()
	repo, cleanup := repositoryForTemporarySchemaTest(t, ctx, dsn)
	defer cleanup()

	suffix := time.Now().UnixNano()
	role, err := repo.CreateSystemRole(ctx, domain.UpsertSystemRoleCommand{
		Name:   "Batch Relation Role",
		Key:    fmt.Sprintf("batch_relation_role_%d", suffix),
		Status: "1",
		Sort:   900,
	})
	if err != nil {
		t.Fatalf("CreateSystemRole() error = %v", err)
	}

	seededRoles, err := repo.ListSystemRoles(ctx, "", "", 1, 100)
	if err != nil {
		t.Fatalf("ListSystemRoles() error = %v", err)
	}
	roleIDs := []int64{role.ID}
	for _, seededRole := range seededRoles.Items {
		if seededRole.ID == role.ID {
			continue
		}
		roleIDs = append(roleIDs, seededRole.ID)
		if len(roleIDs) == 3 {
			break
		}
	}
	if len(roleIDs) != 3 {
		t.Fatalf("ListSystemRoles() returned %d assignable role ids, want 3", len(roleIDs))
	}

	menus, err := repo.ListSystemMenus(ctx, "", "")
	if err != nil {
		t.Fatalf("ListSystemMenus() error = %v", err)
	}
	menuIDs := make([]int64, 0, 3)
	menuPermissions := make([]string, 0, 3)
	for _, menu := range menus.Items {
		if menu.Permission == "" {
			continue
		}
		menuIDs = append(menuIDs, menu.ID)
		menuPermissions = append(menuPermissions, menu.Permission)
		if len(menuIDs) == 3 {
			break
		}
	}
	if len(menuIDs) != 3 {
		t.Fatalf("ListSystemMenus() returned %d permission menus, want 3", len(menuIDs))
	}

	user, err := repo.CreateSystemUser(ctx, domain.UpsertSystemUserCommand{
		Username: fmt.Sprintf("batch_relation_user_%d", suffix),
		Email:    fmt.Sprintf("batch_relation_user_%d@example.com", suffix),
		Status:   1,
		RoleIDs:  []int64{role.ID},
	}, "hash")
	if err != nil {
		t.Fatalf("CreateSystemUser() error = %v", err)
	}

	queryCounter := &repositoryQueryCounter{}
	countedRepo := NewRepository(repo.db.Session(&gorm.Session{Logger: queryCounter}))

	queryCounter.Reset()
	updatedUser, err := countedRepo.AssignSystemUserRoles(ctx, user.ID, roleIDs)
	if err != nil {
		t.Fatalf("AssignSystemUserRoles() error = %v", err)
	}
	if queryCounter.Count() != 6 {
		t.Fatalf("AssignSystemUserRoles() executed %d queries, want 6", queryCounter.Count())
	}
	for _, roleID := range roleIDs {
		found := false
		for _, assignedRoleID := range updatedUser.RoleIDs {
			if assignedRoleID == roleID {
				found = true
				break
			}
		}
		if !found {
			t.Fatalf("AssignSystemUserRoles() role ids = %v, want %d", updatedUser.RoleIDs, roleID)
		}
	}

	queryCounter.Reset()
	updatedRole, err := countedRepo.AssignSystemRoleMenus(ctx, role.ID, menuIDs)
	if err != nil {
		t.Fatalf("AssignSystemRoleMenus() error = %v", err)
	}
	if queryCounter.Count() != 9 {
		t.Fatalf("AssignSystemRoleMenus() executed %d queries, want 9", queryCounter.Count())
	}
	for _, menuID := range menuIDs {
		found := false
		for _, assignedMenuID := range updatedRole.MenuIDs {
			if assignedMenuID == menuID {
				found = true
				break
			}
		}
		if !found {
			t.Fatalf("AssignSystemRoleMenus() menu ids = %v, want %d", updatedRole.MenuIDs, menuID)
		}
	}
	for _, permission := range menuPermissions {
		if !containsListTestString(updatedRole.Permissions, permission) {
			t.Fatalf("AssignSystemRoleMenus() permissions = %v, want %q", updatedRole.Permissions, permission)
		}
	}
}

func TestRepositorySynchronizesMenuPoliciesWithoutNPlusOneQueries(t *testing.T) {
	dsn := os.Getenv("BBS_ADMIN_TEST_DSN")
	if dsn == "" {
		t.Skip("set BBS_ADMIN_TEST_DSN to run postgres-backed repository tests")
	}

	ctx := context.Background()
	repo, cleanup := repositoryForTemporarySchemaTest(t, ctx, dsn)
	defer cleanup()

	suffix := time.Now().UnixNano()
	oldPermission := fmt.Sprintf("system:batch_old_%d", suffix)
	newPermission := fmt.Sprintf("system:batch_new_%d", suffix)
	targetMenu, err := repo.CreateSystemMenu(ctx, domain.UpsertSystemMenuCommand{
		Name:       fmt.Sprintf("batch.policy.target.%d", suffix),
		Title:      "Batch Policy Target",
		Path:       fmt.Sprintf("/batch-policy-target-%d", suffix),
		Component:  "system/batch-policy-target/index",
		Type:       "C",
		Permission: oldPermission,
		Sort:       900,
	})
	if err != nil {
		t.Fatalf("CreateSystemMenu(target) error = %v", err)
	}
	sharedMenu, err := repo.CreateSystemMenu(ctx, domain.UpsertSystemMenuCommand{
		Name:       fmt.Sprintf("batch.policy.shared.%d", suffix),
		Title:      "Batch Policy Shared",
		Path:       fmt.Sprintf("/batch-policy-shared-%d", suffix),
		Component:  "system/batch-policy-shared/index",
		Type:       "C",
		Permission: oldPermission,
		Sort:       901,
	})
	if err != nil {
		t.Fatalf("CreateSystemMenu(shared) error = %v", err)
	}

	roles := make([]domain.SystemRole, 0, 3)
	for index := 0; index < 3; index++ {
		role, err := repo.CreateSystemRole(ctx, domain.UpsertSystemRoleCommand{
			Name:   fmt.Sprintf("Batch Policy Role %d", index),
			Key:    fmt.Sprintf("batch_policy_role_%d_%d", suffix, index),
			Status: "1",
			Sort:   int32(910 + index),
		})
		if err != nil {
			t.Fatalf("CreateSystemRole(%d) error = %v", index, err)
		}
		roles = append(roles, role)
		menuIDs := []int64{targetMenu.ID}
		if index == 0 {
			menuIDs = append(menuIDs, sharedMenu.ID)
		}
		if _, err := repo.AssignSystemRoleMenus(ctx, role.ID, menuIDs); err != nil {
			t.Fatalf("AssignSystemRoleMenus(%d) error = %v", index, err)
		}
	}

	queryCounter := &repositoryQueryCounter{}
	countedRepo := NewRepository(repo.db.Session(&gorm.Session{Logger: queryCounter}))
	queryCounter.Reset()
	if _, err := countedRepo.UpdateSystemMenu(ctx, domain.UpsertSystemMenuCommand{
		ID:         targetMenu.ID,
		ParentID:   targetMenu.ParentID,
		Name:       targetMenu.Name,
		Title:      targetMenu.Title,
		Icon:       targetMenu.Icon,
		Path:       targetMenu.Path,
		Component:  targetMenu.Component,
		Type:       targetMenu.Type,
		Permission: newPermission,
		Status:     targetMenu.Status,
		Visible:    targetMenu.Visible,
		IsHide:     targetMenu.IsHide,
		Sort:       targetMenu.Sort,
		Remark:     targetMenu.Remark,
	}); err != nil {
		t.Fatalf("UpdateSystemMenu() error = %v", err)
	}
	if queryCounter.Count() > 8 {
		t.Fatalf("UpdateSystemMenu() executed %d queries, want at most 8", queryCounter.Count())
	}

	rules, err := repo.CasbinRules(ctx)
	if err != nil {
		t.Fatalf("CasbinRules() after update error = %v", err)
	}
	assertPolicy := func(roleKey string, permission string, want bool) {
		resource, action, ok := splitPermission(permission)
		if !ok {
			t.Fatalf("invalid test permission %q", permission)
		}
		found := false
		for _, rule := range rules {
			if rule.Ptype == "p" && rule.V0 == roleKey && rule.V1 == resource && rule.V2 == action {
				found = true
				break
			}
		}
		if found != want {
			t.Fatalf("policy %s:%s for role %q found=%t, want %t", resource, action, roleKey, found, want)
		}
	}
	assertPolicy(roles[0].Key, oldPermission, true)
	for _, role := range roles {
		assertPolicy(role.Key, newPermission, true)
	}
	for _, role := range roles[1:] {
		assertPolicy(role.Key, oldPermission, false)
	}

	queryCounter.Reset()
	if err := countedRepo.DeleteSystemMenu(ctx, targetMenu.ID); err != nil {
		t.Fatalf("DeleteSystemMenu() error = %v", err)
	}
	if queryCounter.Count() > 10 {
		t.Fatalf("DeleteSystemMenu() executed %d queries, want at most 10", queryCounter.Count())
	}

	rules, err = repo.CasbinRules(ctx)
	if err != nil {
		t.Fatalf("CasbinRules() after delete error = %v", err)
	}
	assertPolicy(roles[0].Key, oldPermission, true)
	for _, role := range roles {
		assertPolicy(role.Key, newPermission, false)
	}
}

func containsListTestString(values []string, target string) bool {
	for _, value := range values {
		if value == target {
			return true
		}
	}
	return false
}

func repositoryForTemporarySchemaTest(t *testing.T, ctx context.Context, dsn string) (*Repository, func()) {
	t.Helper()
	db := openPostgresForTest(t, testDSNWithSearchPath(t, dsn, "pg_temp,bbs_admin"))
	sqlDB, err := db.DB()
	if err != nil {
		t.Fatalf("open temporary postgres test database: %v", err)
	}
	sqlDB.SetMaxIdleConns(1)
	sqlDB.SetMaxOpenConns(1)
	if err := db.WithContext(ctx).Exec("CREATE TEMP TABLE bbs_admin_list_batch_test_connection(id INT)").Error; err != nil {
		_ = sqlDB.Close()
		t.Fatalf("initialize postgres temporary schema: %v", err)
	}
	repo := NewRepository(db)
	if err := repo.EnsureSchema(ctx); err != nil {
		_ = sqlDB.Close()
		t.Fatalf("EnsureSchema() in temporary schema error = %v", err)
	}
	if err := repo.SeedDefaults(ctx, []string{"admin"}, "Admin123!"); err != nil {
		_ = sqlDB.Close()
		t.Fatalf("SeedDefaults() in temporary schema error = %v", err)
	}
	return repo, func() { _ = sqlDB.Close() }
}

type repositoryQueryCounter struct {
	queries int
}

func (c *repositoryQueryCounter) LogMode(gormlogger.LogLevel) gormlogger.Interface {
	return c
}

func (*repositoryQueryCounter) Info(context.Context, string, ...interface{}) {}

func (*repositoryQueryCounter) Warn(context.Context, string, ...interface{}) {}

func (*repositoryQueryCounter) Error(context.Context, string, ...interface{}) {}

func (c *repositoryQueryCounter) Trace(context.Context, time.Time, func() (string, int64), error) {
	c.queries++
}

func (c *repositoryQueryCounter) Reset() {
	c.queries = 0
}

func (c *repositoryQueryCounter) Count() int {
	return c.queries
}
