package persistence

import (
	"context"
	"errors"
	"fmt"
	"os"
	"sync"
	"testing"
	"time"

	domain "user-service/internal/domain/user"

	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

func TestInviteRepoPostgresAtomicConsumption(t *testing.T) {
	if os.Getenv("BBS_PG_SMOKE") != "1" {
		t.Skip("set BBS_PG_SMOKE=1 to run PostgreSQL invite transaction test")
	}
	dsn := os.Getenv("BBS_USER_PG_DSN")
	if dsn == "" {
		dsn = "postgres://bbs_user_app:local_user_pass@127.0.0.1:5432/bbs?sslmode=disable&search_path=bbs_user"
	}
	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Fatalf("open postgres: %v", err)
	}
	repo := NewRepo(db)
	ctx := context.Background()
	base := time.Now().UnixNano()
	inviteID := base
	code := fmt.Sprintf("SMOKE%X", uint64(base))
	userIDs := []int64{base + 1, base + 2, base + 3}
	defer func() {
		_ = db.Exec("DELETE FROM user_invite_codes WHERE id IN (?, ?)", inviteID, inviteID+1).Error
		_ = db.Exec("DELETE FROM users WHERE id IN ?", userIDs).Error
	}()

	if err := repo.CreateInviteCodes(ctx, []domain.InviteCode{{
		ID: inviteID, Code: code, CreatedByAdminID: 42, CreatedAt: time.Now(),
	}}); err != nil {
		t.Fatalf("create invite: %v", err)
	}
	users := make([]*domain.User, 2)
	for i := range users {
		users[i], err = domain.New(userIDs[i], domain.RegisterCmd{
			Username: fmt.Sprintf("invite_smoke_%d_%d", i, base%1000000),
			Email:    fmt.Sprintf("invite-smoke-%d-%d@example.com", i, base),
			Password: "password123",
			Nickname: fmt.Sprintf("Invite %d", i),
		}, "hash")
		if err != nil {
			t.Fatalf("new user %d: %v", i, err)
		}
	}

	start := make(chan struct{})
	errs := make(chan error, 2)
	var wg sync.WaitGroup
	for _, user := range users {
		wg.Add(1)
		go func(user *domain.User) {
			defer wg.Done()
			<-start
			errs <- repo.CreateWithInvite(ctx, user, code, true)
		}(user)
	}
	close(start)
	wg.Wait()
	close(errs)
	var successes, usedErrors int
	for err := range errs {
		switch {
		case err == nil:
			successes++
		case errors.Is(err, domain.ErrInviteCodeUsed):
			usedErrors++
		default:
			t.Fatalf("unexpected concurrent error: %v", err)
		}
	}
	if successes != 1 || usedErrors != 1 {
		t.Fatalf("successes=%d usedErrors=%d, want 1/1", successes, usedErrors)
	}
	var createdUsers int64
	if err := db.Model(&userPO{}).Where("id IN ?", userIDs[:2]).Count(&createdUsers).Error; err != nil {
		t.Fatalf("count created users: %v", err)
	}
	if createdUsers != 1 {
		t.Fatalf("created users = %d, want 1", createdUsers)
	}

	existing, err := domain.New(userIDs[2], domain.RegisterCmd{
		Username: fmt.Sprintf("invite_conflict_%d", base%1000000),
		Email:    fmt.Sprintf("invite-conflict-%d@example.com", base),
		Password: "password123", Nickname: "Existing",
	}, "hash")
	if err != nil {
		t.Fatalf("new existing user: %v", err)
	}
	if err := repo.Create(ctx, existing); err != nil {
		t.Fatalf("create existing user: %v", err)
	}
	rollbackCode := code + "B"
	if err := repo.CreateInviteCodes(ctx, []domain.InviteCode{{
		ID: inviteID + 1, Code: rollbackCode, CreatedByAdminID: 42, CreatedAt: time.Now(),
	}}); err != nil {
		t.Fatalf("create rollback invite: %v", err)
	}
	conflict, err := domain.New(base+4, domain.RegisterCmd{
		Username: existing.Username,
		Email:    fmt.Sprintf("invite-other-%d@example.com", base),
		Password: "password123", Nickname: "Conflict",
	}, "hash")
	if err != nil {
		t.Fatalf("new conflicting user: %v", err)
	}
	if err := repo.CreateWithInvite(ctx, conflict, rollbackCode, true); !errors.Is(err, domain.ErrUsernameExists) {
		t.Fatalf("conflicting registration error = %v, want ErrUsernameExists", err)
	}
	var rollbackInvite inviteCodePO
	if err := db.Where("id = ?", inviteID+1).First(&rollbackInvite).Error; err != nil {
		t.Fatalf("read rollback invite: %v", err)
	}
	if rollbackInvite.UsedAt != nil || rollbackInvite.UsedByUserID != nil {
		t.Fatalf("failed registration consumed invite: %+v", rollbackInvite)
	}
}
