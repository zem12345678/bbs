package persistence

import (
	"context"
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
