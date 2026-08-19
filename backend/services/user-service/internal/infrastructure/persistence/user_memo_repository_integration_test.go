package persistence

import (
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

func TestUserMemoRepoPostgresViewerIsolationUpdateAndDelete(t *testing.T) {
	db, repo := openUserMemoPostgres(t)
	ctx := context.Background()
	base := time.Now().UnixNano()
	viewer := createUserMemoOwner(t, repo, base, "viewer")
	otherViewer := createUserMemoOwner(t, repo, base+1, "other")
	target := createUserMemoOwner(t, repo, base+2, "target")
	t.Cleanup(func() {
		_ = db.Exec("DELETE FROM users WHERE id IN ?", []int64{viewer.ID, otherViewer.ID, target.ID}).Error
	})

	if err := repo.UpdateUserMemo(ctx, viewer.ID, target.ID, "  project lead  "); err != nil {
		t.Fatalf("create viewer memo: %v", err)
	}
	if err := repo.UpdateUserMemo(ctx, otherViewer.ID, target.ID, "other viewer memo"); err != nil {
		t.Fatalf("create other viewer memo: %v", err)
	}
	assertUserMemo(t, repo, viewer.ID, target.ID, "project lead")
	assertUserMemo(t, repo, otherViewer.ID, target.ID, "other viewer memo")

	if err := repo.UpdateUserMemo(ctx, viewer.ID, target.ID, "replacement"); err != nil {
		t.Fatalf("replace viewer memo: %v", err)
	}
	assertUserMemo(t, repo, viewer.ID, target.ID, "replacement")
	assertUserMemo(t, repo, otherViewer.ID, target.ID, "other viewer memo")

	if err := repo.UpdateUserMemo(ctx, viewer.ID, target.ID, "  "); err != nil {
		t.Fatalf("delete viewer memo: %v", err)
	}
	assertUserMemo(t, repo, viewer.ID, target.ID, "")
	assertUserMemo(t, repo, otherViewer.ID, target.ID, "other viewer memo")

	err := repo.UpdateUserMemo(ctx, viewer.ID, target.ID, strings.Repeat("界", domain.MaxUserMemoRunes+1))
	if !errors.Is(err, domain.ErrUserMemoTooLong) {
		t.Fatalf("oversized memo error = %v, want ErrUserMemoTooLong", err)
	}
}

func openUserMemoPostgres(t *testing.T) (*gorm.DB, *Repo) {
	t.Helper()
	if os.Getenv("BBS_PG_SMOKE") != "1" {
		t.Skip("set BBS_PG_SMOKE=1 to run PostgreSQL user memo repository tests")
	}
	dsn := os.Getenv("BBS_USER_PG_DSN")
	if dsn == "" {
		dsn = "postgres://bbs_user_app:local_user_pass@127.0.0.1:5432/bbs?sslmode=disable&search_path=bbs_user"
	}
	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Fatalf("open postgres: %v", err)
	}
	var tableExists bool
	if err := db.Raw("SELECT to_regclass(current_schema() || '.user_memos') IS NOT NULL").Scan(&tableExists).Error; err != nil {
		t.Fatalf("check user_memos table: %v", err)
	}
	if !tableExists {
		t.Fatal("user_memos table is missing; apply migrations through 0022 before running this test")
	}
	return db, NewRepo(db)
}

func createUserMemoOwner(t *testing.T, repo *Repo, id int64, label string) *domain.User {
	t.Helper()
	suffix := fmt.Sprintf("%d", id%1000000000)
	owner, err := domain.New(id, domain.RegisterCmd{
		Username: "memo_" + label + "_" + suffix,
		Email:    "memo_" + label + "_" + suffix + "@example.com",
		Password: "password123",
		Nickname: "Memo " + label,
	}, "hash")
	if err != nil {
		t.Fatalf("new %s user: %v", label, err)
	}
	if err := repo.Create(context.Background(), owner); err != nil {
		t.Fatalf("create %s user: %v", label, err)
	}
	return owner
}

func assertUserMemo(t *testing.T, repo *Repo, viewerID, targetID int64, want string) {
	t.Helper()
	got, err := repo.GetUserMemo(context.Background(), viewerID, targetID)
	if err != nil {
		t.Fatalf("GetUserMemo(%d, %d): %v", viewerID, targetID, err)
	}
	if got != want {
		t.Fatalf("GetUserMemo(%d, %d) = %q, want %q", viewerID, targetID, got, want)
	}
}
