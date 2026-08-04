package persistence

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strings"
	"testing"
	"time"

	domain "file-service/internal/domain/file"

	"github.com/jackc/pgx/v5/pgxpool"
)

func TestGenericFilePostgresIntegration(t *testing.T) {
	dsn := strings.TrimSpace(os.Getenv("BBS_FILE_POSTGRES_TEST_DSN"))
	if dsn == "" {
		t.Skip("BBS_FILE_POSTGRES_TEST_DSN is not set")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	t.Cleanup(cancel)
	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		t.Fatalf("open postgres pool: %v", err)
	}
	t.Cleanup(pool.Close)
	if err := pool.Ping(ctx); err != nil {
		t.Fatalf("ping postgres: %v", err)
	}

	repo := NewPostgresRepository(pool)
	if err := repo.EnsureSchema(ctx); err != nil {
		t.Fatalf("ensure file schema: %v", err)
	}

	seed := time.Now().UnixNano()
	ownerID := seed
	otherUserID := seed + 1
	now := time.Now().UTC().Truncate(time.Microsecond)
	objectPrefix := fmt.Sprintf("integration-files/%d", seed)
	fileIDs := make([]int64, 0, 3)
	t.Cleanup(func() {
		cleanupCtx, cleanupCancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cleanupCancel()
		if len(fileIDs) != 0 {
			_, _ = pool.Exec(cleanupCtx, `DELETE FROM files WHERE id = ANY($1::BIGINT[])`, fileIDs)
		}
	})

	created := make([]domain.File, 0, 3)
	for index := 1; index <= 3; index++ {
		item, createErr := repo.CreateFile(ctx, domain.File{
			OwnerID:      ownerID,
			BizType:      "drive",
			ObjectKey:    fmt.Sprintf("%s/file-%d.bin", objectPrefix, index),
			OriginalName: fmt.Sprintf("file-%d.bin", index),
			ContentType:  "application/octet-stream",
			SizeBytes:    int64(index * 10),
			Status:       domain.FileStatusActive,
			CreatedAt:    now.Add(time.Duration(index) * time.Second),
			UpdatedAt:    now.Add(time.Duration(index) * time.Second),
		}, 1<<30)
		if createErr != nil {
			t.Fatalf("create file %d: %v", index, createErr)
		}
		created = append(created, item)
		fileIDs = append(fileIDs, item.ID)
	}

	if _, err := repo.CreateFile(ctx, domain.File{
		OwnerID:      ownerID,
		BizType:      "drive",
		ObjectKey:    created[0].ObjectKey,
		OriginalName: "duplicate.bin",
		ContentType:  "application/octet-stream",
		SizeBytes:    1,
		Status:       domain.FileStatusActive,
		CreatedAt:    now,
		UpdatedAt:    now,
	}, 1<<30); !errors.Is(err, domain.ErrFileObjectKeyTaken) {
		t.Fatalf("duplicate object key error = %v, want ErrFileObjectKeyTaken", err)
	}

	firstPage, total, err := repo.ListUserFiles(ctx, ownerID, 2, 0)
	if err != nil {
		t.Fatalf("list first page: %v", err)
	}
	if total != 3 || len(firstPage) != 2 || firstPage[0].ID != created[2].ID || firstPage[1].ID != created[1].ID {
		t.Fatalf("first page = %+v, total = %d; want newest two of three", firstPage, total)
	}
	secondPage, total, err := repo.ListUserFiles(ctx, ownerID, 2, 2)
	if err != nil {
		t.Fatalf("list second page: %v", err)
	}
	if total != 3 || len(secondPage) != 1 || secondPage[0].ID != created[0].ID {
		t.Fatalf("second page = %+v, total = %d; want oldest file", secondPage, total)
	}
	otherFiles, otherTotal, err := repo.ListUserFiles(ctx, otherUserID, 20, 0)
	if err != nil || otherTotal != 0 || len(otherFiles) != 0 {
		t.Fatalf("other user's list = %+v, total = %d, err = %v", otherFiles, otherTotal, err)
	}

	loaded, err := repo.GetFile(ctx, ownerID, created[1].ID)
	if err != nil || loaded.ID != created[1].ID || loaded.OwnerID != ownerID {
		t.Fatalf("get owned file = %+v, err = %v", loaded, err)
	}
	if _, err := repo.GetFile(ctx, otherUserID, created[1].ID); !errors.Is(err, domain.ErrFileOwnerMismatch) {
		t.Fatalf("get as other user error = %v, want ErrFileOwnerMismatch", err)
	}
	if _, err := repo.BeginFileDeletion(ctx, otherUserID, created[1].ID, now.Add(10*time.Second)); !errors.Is(err, domain.ErrFileOwnerMismatch) {
		t.Fatalf("delete as other user error = %v, want ErrFileOwnerMismatch", err)
	}

	deletingAt := now.Add(20 * time.Second)
	deleting, err := repo.BeginFileDeletion(ctx, ownerID, created[1].ID, deletingAt)
	if err != nil {
		t.Fatalf("begin file deletion: %v", err)
	}
	if deleting.Status != domain.FileStatusDeleting || deleting.ObjectKey != created[1].ObjectKey || deleting.DeletedAt != nil {
		t.Fatalf("deleting file = %+v", deleting)
	}
	deletingRetry, err := repo.BeginFileDeletion(ctx, ownerID, created[1].ID, deletingAt.Add(time.Second))
	if err != nil || deletingRetry.Status != domain.FileStatusDeleting || deletingRetry.ObjectKey != created[1].ObjectKey {
		t.Fatalf("replayed begin deletion = %+v, err = %v", deletingRetry, err)
	}
	loaded, err = repo.GetFile(ctx, ownerID, created[1].ID)
	if err != nil || loaded.Status != domain.FileStatusDeleting {
		t.Fatalf("get deleting file = %+v, err = %v", loaded, err)
	}

	deletedAt := now.Add(30 * time.Second)
	deleted, err := repo.CompleteFileDeletion(ctx, ownerID, created[1].ID, deletedAt)
	if err != nil {
		t.Fatalf("complete file deletion: %v", err)
	}
	wantDeletedKey := fmt.Sprintf("deleted/files/%d", created[1].ID)
	if deleted.Status != domain.FileStatusDeleted || deleted.ObjectKey != wantDeletedKey || deleted.DeletedAt == nil || !deleted.DeletedAt.Equal(deletedAt) {
		t.Fatalf("deleted file = %+v, want key %q and deleted timestamp %v", deleted, wantDeletedKey, deletedAt)
	}
	replayed, err := repo.CompleteFileDeletion(ctx, ownerID, created[1].ID, deletedAt.Add(time.Minute))
	if err != nil || replayed.Status != domain.FileStatusDeleted || replayed.ObjectKey != wantDeletedKey || replayed.DeletedAt == nil || !replayed.DeletedAt.Equal(deletedAt) {
		t.Fatalf("replayed completion = %+v, err = %v", replayed, err)
	}
	if _, err := repo.GetFile(ctx, ownerID, created[1].ID); !errors.Is(err, domain.ErrFileDeleted) {
		t.Fatalf("get deleted file error = %v, want ErrFileDeleted", err)
	}
	if _, err := repo.BeginFileDeletion(ctx, ownerID, created[1].ID, deletedAt.Add(time.Minute)); !errors.Is(err, domain.ErrFileDeleted) {
		t.Fatalf("begin deletion after completion error = %v, want ErrFileDeleted", err)
	}
	remaining, total, err := repo.ListUserFiles(ctx, ownerID, 20, 0)
	if err != nil || total != 2 || len(remaining) != 2 {
		t.Fatalf("list after deletion = %+v, total = %d, err = %v", remaining, total, err)
	}
	usedBytes, err := repo.GetFileUsage(ctx, ownerID)
	if err != nil || usedBytes != 40 {
		t.Fatalf("usage after deletion = %d, err = %v; want 40", usedBytes, err)
	}
	boundary, err := repo.CreateFile(ctx, domain.File{
		OwnerID:      ownerID,
		BizType:      "drive",
		ObjectKey:    objectPrefix + "/boundary.bin",
		OriginalName: "boundary.bin",
		ContentType:  "application/octet-stream",
		SizeBytes:    60,
		Status:       domain.FileStatusActive,
		CreatedAt:    now.Add(time.Minute),
		UpdatedAt:    now.Add(time.Minute),
	}, 100)
	if err != nil {
		t.Fatalf("create at capacity boundary: %v", err)
	}
	fileIDs = append(fileIDs, boundary.ID)
	if _, err := repo.CreateFile(ctx, domain.File{
		OwnerID:      ownerID,
		BizType:      "drive",
		ObjectKey:    objectPrefix + "/over-capacity.bin",
		OriginalName: "over-capacity.bin",
		ContentType:  "application/octet-stream",
		SizeBytes:    1,
		Status:       domain.FileStatusActive,
		CreatedAt:    now.Add(2 * time.Minute),
		UpdatedAt:    now.Add(2 * time.Minute),
	}, 100); !errors.Is(err, domain.ErrFileCapacityExceeded) {
		t.Fatalf("create over capacity error = %v, want ErrFileCapacityExceeded", err)
	}
}

func TestCreateFileCapacityIsConcurrencySafe(t *testing.T) {
	dsn := strings.TrimSpace(os.Getenv("BBS_FILE_POSTGRES_TEST_DSN"))
	if dsn == "" {
		t.Skip("BBS_FILE_POSTGRES_TEST_DSN is not set")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	t.Cleanup(cancel)
	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		t.Fatalf("open postgres pool: %v", err)
	}
	t.Cleanup(pool.Close)
	repo := NewPostgresRepository(pool)
	if err := repo.EnsureSchema(ctx); err != nil {
		t.Fatalf("ensure file schema: %v", err)
	}

	seed := time.Now().UnixNano()
	ownerID := seed
	objectPrefix := fmt.Sprintf("integration-capacity/%d", seed)
	t.Cleanup(func() {
		cleanupCtx, cleanupCancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cleanupCancel()
		_, _ = pool.Exec(cleanupCtx, `DELETE FROM files WHERE owner_user_id = $1`, ownerID)
	})

	start := make(chan struct{})
	results := make(chan error, 2)
	for index := 0; index < 2; index++ {
		index := index
		go func() {
			<-start
			_, createErr := repo.CreateFile(ctx, domain.File{
				OwnerID:      ownerID,
				BizType:      "drive",
				ObjectKey:    fmt.Sprintf("%s/file-%d.bin", objectPrefix, index),
				OriginalName: fmt.Sprintf("file-%d.bin", index),
				ContentType:  "application/octet-stream",
				SizeBytes:    60,
				Status:       domain.FileStatusActive,
				CreatedAt:    time.Now().UTC(),
				UpdatedAt:    time.Now().UTC(),
			}, 100)
			results <- createErr
		}()
	}
	close(start)

	var succeeded, exhausted int
	for index := 0; index < 2; index++ {
		switch err := <-results; {
		case err == nil:
			succeeded++
		case errors.Is(err, domain.ErrFileCapacityExceeded):
			exhausted++
		default:
			t.Fatalf("concurrent create error = %v", err)
		}
	}
	if succeeded != 1 || exhausted != 1 {
		t.Fatalf("concurrent creates: succeeded=%d exhausted=%d, want 1/1", succeeded, exhausted)
	}
	usedBytes, err := repo.GetFileUsage(ctx, ownerID)
	if err != nil || usedBytes != 60 {
		t.Fatalf("concurrent usage = %d, err = %v; want 60", usedBytes, err)
	}
}
