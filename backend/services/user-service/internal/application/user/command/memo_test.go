package command

import (
	"context"
	"errors"
	"strings"
	"testing"

	domain "user-service/internal/domain/user"
)

type userMemoMemoryRepo struct {
	*memoryRepo
	memos map[[2]int64]string
}

func (r *userMemoMemoryRepo) UpdateUserMemo(_ context.Context, userID, targetUserID int64, memo string) error {
	key := [2]int64{userID, targetUserID}
	if memo == "" {
		delete(r.memos, key)
		return nil
	}
	r.memos[key] = memo
	return nil
}

func (r *userMemoMemoryRepo) GetUserMemo(_ context.Context, userID, targetUserID int64) (string, error) {
	return r.memos[[2]int64{userID, targetUserID}], nil
}

func TestUpdateUserMemoStoresReadsAndDeletesPrivateMemo(t *testing.T) {
	base := newMemoryRepo()
	base.users[1] = &domain.User{ID: 1}
	base.users[2] = &domain.User{ID: 2}
	repo := &userMemoMemoryRepo{memoryRepo: base, memos: map[[2]int64]string{}}
	service := NewService(repo, nil, nil, nil, "test-secret", 0, 8, nil, nil, nil)

	if err := service.UpdateUserMemo(context.Background(), 1, 2, "  project lead  "); err != nil {
		t.Fatalf("UpdateUserMemo() error = %v", err)
	}
	memo, err := service.GetUserMemo(context.Background(), 1, 2)
	if err != nil {
		t.Fatalf("GetUserMemo() error = %v", err)
	}
	if memo != "project lead" {
		t.Fatalf("GetUserMemo() = %q, want trimmed memo", memo)
	}
	if err := service.UpdateUserMemo(context.Background(), 1, 2, ""); err != nil {
		t.Fatalf("delete UpdateUserMemo() error = %v", err)
	}
	if memo, _ := service.GetUserMemo(context.Background(), 1, 2); memo != "" {
		t.Fatalf("memo after delete = %q, want empty", memo)
	}
}

func TestUpdateUserMemoValidatesTargetAndLength(t *testing.T) {
	base := newMemoryRepo()
	base.users[1] = &domain.User{ID: 1}
	repo := &userMemoMemoryRepo{memoryRepo: base, memos: map[[2]int64]string{}}
	service := NewService(repo, nil, nil, nil, "test-secret", 0, 8, nil, nil, nil)

	if err := service.UpdateUserMemo(context.Background(), 1, 2, "missing"); !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("missing target error = %v, want ErrNotFound", err)
	}
	base.users[2] = &domain.User{ID: 2}
	if err := service.UpdateUserMemo(context.Background(), 1, 2, strings.Repeat("界", domain.MaxUserMemoRunes+1)); !errors.Is(err, domain.ErrUserMemoTooLong) {
		t.Fatalf("oversized memo error = %v, want ErrUserMemoTooLong", err)
	}
}
