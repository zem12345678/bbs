package persistence

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"os"
	"strings"
	"testing"
	"time"

	domain "file-service/internal/domain/file"

	"github.com/jackc/pgx/v5/pgxpool"
)

// This test is opt-in because it requires a PostgreSQL database with the file
// service role/schema. It exercises the repository transaction boundaries that
// cannot be covered by the in-memory application tests.
func TestAccountErasurePostgresIntegration(t *testing.T) {
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
	targetUserID := seed
	otherUserID := seed + 1
	observerUserID := seed + 2
	jobID := seed + 100
	policyVersion := int32(3)
	now := time.Now().UTC().Truncate(time.Microsecond)
	objectPrefix := fmt.Sprintf("integration-erasure/%d", seed)

	var targetAttachments []domain.Attachment
	for index := 1; index <= 2; index++ {
		attachment, createErr := repo.CreateAttachment(ctx, domain.Attachment{
			TopicID:      seed + int64(index),
			OwnerID:      targetUserID,
			ObjectKey:    fmt.Sprintf("%s/target-%d.bin", objectPrefix, index),
			OriginalName: fmt.Sprintf("target-%d.bin", index),
			ContentType:  "application/octet-stream",
			SizeBytes:    int64(index * 10),
			PriceCredits: int64(index),
			Status:       domain.AttachmentStatusActive,
			CreatedAt:    now.Add(time.Duration(index) * time.Second),
			UpdatedAt:    now.Add(time.Duration(index) * time.Second),
		}, 1<<30)
		if createErr != nil {
			t.Fatalf("create target attachment %d: %v", index, createErr)
		}
		targetAttachments = append(targetAttachments, attachment)
	}
	otherAttachment, err := repo.CreateAttachment(ctx, domain.Attachment{
		TopicID:      seed + 10,
		OwnerID:      otherUserID,
		ObjectKey:    objectPrefix + "/other.bin",
		OriginalName: "other.bin",
		ContentType:  "application/octet-stream",
		SizeBytes:    20,
		PriceCredits: 0,
		Status:       domain.AttachmentStatusActive,
		CreatedAt:    now.Add(3 * time.Second),
		UpdatedAt:    now.Add(3 * time.Second),
	}, 1<<30)
	if err != nil {
		t.Fatalf("create unrelated attachment: %v", err)
	}
	var targetFiles []domain.File
	managedMediaBizTypes := []string{"avatars", "images"}
	for index := 1; index <= 2; index++ {
		item, createErr := repo.CreateFile(ctx, domain.File{
			OwnerID:      targetUserID,
			BizType:      managedMediaBizTypes[index-1],
			ObjectKey:    fmt.Sprintf("%s/target-file-%d.bin", objectPrefix, index),
			OriginalName: fmt.Sprintf("target-file-%d.bin", index),
			ContentType:  "application/octet-stream",
			SizeBytes:    int64(index * 100),
			Status:       domain.FileStatusActive,
			CreatedAt:    now.Add(time.Duration(index+3) * time.Second),
			UpdatedAt:    now.Add(time.Duration(index+3) * time.Second),
		}, 1<<30)
		if createErr != nil {
			t.Fatalf("create target file %d: %v", index, createErr)
		}
		targetFiles = append(targetFiles, item)
	}
	if _, err := repo.BeginFileDeletion(ctx, targetUserID, targetFiles[1].ID, now.Add(10*time.Second)); err != nil {
		t.Fatalf("mark second target file deleting: %v", err)
	}
	otherFile, err := repo.CreateFile(ctx, domain.File{
		OwnerID:      otherUserID,
		BizType:      "drive",
		ObjectKey:    objectPrefix + "/other-file.bin",
		OriginalName: "other-file.bin",
		ContentType:  "application/octet-stream",
		SizeBytes:    30,
		Status:       domain.FileStatusActive,
		CreatedAt:    now.Add(6 * time.Second),
		UpdatedAt:    now.Add(6 * time.Second),
	}, 1<<30)
	if err != nil {
		t.Fatalf("create unrelated file: %v", err)
	}
	overrideBytes := int64(2 << 30)
	if err := repo.SetFileCapacityOverride(ctx, targetUserID, &overrideBytes, now); err != nil {
		t.Fatalf("set target capacity override: %v", err)
	}

	// Two downloads of the target's attachments are removed by attachment ID,
	// one download made by the target is removed by user ID, and this unrelated
	// download must survive the erasure.
	for index, attachment := range targetAttachments {
		if _, err := repo.EnsureDownload(ctx, attachment.ID, otherUserID, fmt.Sprintf("%s/target-download-%d", objectPrefix, index), 0, now); err != nil {
			t.Fatalf("create target download %d: %v", index, err)
		}
	}
	if _, err := repo.EnsureDownload(ctx, otherAttachment.ID, targetUserID, objectPrefix+"/target-user-download", 0, now); err != nil {
		t.Fatalf("create target user download: %v", err)
	}
	if _, err := repo.EnsureDownload(ctx, otherAttachment.ID, observerUserID, objectPrefix+"/unrelated-download", 0, now); err != nil {
		t.Fatalf("create unrelated download: %v", err)
	}

	t.Cleanup(func() {
		cleanupCtx, cleanupCancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cleanupCancel()
		ids := make([]int64, 0, len(targetAttachments)+1)
		for _, attachment := range targetAttachments {
			ids = append(ids, attachment.ID)
		}
		ids = append(ids, otherAttachment.ID)
		fileIDs := []int64{targetFiles[0].ID, targetFiles[1].ID, otherFile.ID}
		_, _ = pool.Exec(cleanupCtx, `DELETE FROM file_erased_attachment_objects WHERE user_id = $1`, targetUserID)
		_, _ = pool.Exec(cleanupCtx, `DELETE FROM file_erased_file_objects WHERE user_id = $1`, targetUserID)
		_, _ = pool.Exec(cleanupCtx, `DELETE FROM attachment_downloads WHERE attachment_id = ANY($1::BIGINT[]) OR user_id = ANY($2::BIGINT[])`, ids, []int64{targetUserID, otherUserID, observerUserID})
		_, _ = pool.Exec(cleanupCtx, `DELETE FROM file_erased_users WHERE user_id = $1`, targetUserID)
		_, _ = pool.Exec(cleanupCtx, `DELETE FROM attachments WHERE id = ANY($1::BIGINT[])`, ids)
		_, _ = pool.Exec(cleanupCtx, `DELETE FROM files WHERE id = ANY($1::BIGINT[])`, fileIDs)
		_, _ = pool.Exec(cleanupCtx, `DELETE FROM file_user_capacity_overrides WHERE user_id = $1`, targetUserID)
	})

	result, objects, err := repo.BeginAccountErasure(ctx, targetUserID, jobID, policyVersion)
	if err != nil {
		t.Fatalf("begin account erasure: %v", err)
	}
	if result != (domain.AccountErasureResult{ArchivedAttachments: 2, DeletedDownloads: 3}) {
		t.Fatalf("begin result = %+v, want two attachments and three downloads", result)
	}
	attachmentObjects, fileObjects := splitErasureObjects(objects)
	if len(attachmentObjects) != 2 || attachmentObjects[0].AttachmentID != targetAttachments[0].ID || attachmentObjects[1].AttachmentID != targetAttachments[1].ID {
		t.Fatalf("pending attachment objects = %+v, want both target attachments in ID order", attachmentObjects)
	}
	if len(fileObjects) != 2 || fileObjects[0].FileID != targetFiles[0].ID || fileObjects[1].FileID != targetFiles[1].ID {
		t.Fatalf("pending file objects = %+v, want both target files in ID order", fileObjects)
	}

	for _, attachment := range targetAttachments {
		assertErasedAttachment(t, ctx, pool, attachment.ID)
	}
	for _, item := range targetFiles {
		assertErasedFile(t, ctx, pool, item.ID)
	}
	assertCount(t, ctx, pool, 0, `SELECT COUNT(*) FROM attachment_downloads WHERE user_id = $1 OR attachment_id = ANY($2::BIGINT[])`, targetUserID, []int64{targetAttachments[0].ID, targetAttachments[1].ID})
	assertCount(t, ctx, pool, 1, `SELECT COUNT(*) FROM attachment_downloads WHERE attachment_id = $1 AND user_id = $2`, otherAttachment.ID, observerUserID)
	assertCount(t, ctx, pool, 2, `SELECT COUNT(*) FROM file_erased_attachment_objects WHERE user_id = $1 AND deleted_at IS NULL`, targetUserID)
	assertCount(t, ctx, pool, 2, `SELECT COUNT(*) FROM file_erased_file_objects WHERE user_id = $1 AND deleted_at IS NULL`, targetUserID)
	assertCount(t, ctx, pool, 1, `SELECT COUNT(*) FROM files WHERE id = $1 AND owner_user_id = $2 AND status = 'ACTIVE'`, otherFile.ID, otherUserID)
	assertCount(t, ctx, pool, 0, `SELECT COUNT(*) FROM file_user_capacity_overrides WHERE user_id = $1`, targetUserID)

	if _, err := repo.CompleteAccountErasure(ctx, targetUserID, now.Add(time.Minute)); !errors.Is(err, domain.ErrAccountErasureUnavailable) {
		t.Fatalf("complete with pending objects error = %v, want ErrAccountErasureUnavailable", err)
	}

	firstDeletedAt := now.Add(2 * time.Minute)
	if err := repo.CompleteAccountErasureObject(ctx, targetUserID, attachmentObjects[0].AttachmentID, firstDeletedAt); err != nil {
		t.Fatalf("complete first object: %v", err)
	}
	if err := repo.CompleteAccountErasureObject(ctx, targetUserID, attachmentObjects[0].AttachmentID, firstDeletedAt.Add(time.Minute)); err != nil {
		t.Fatalf("idempotent completion of first object: %v", err)
	}
	if err := repo.CompleteAccountErasureFileObject(ctx, targetUserID, fileObjects[0].FileID, firstDeletedAt); err != nil {
		t.Fatalf("complete first file object: %v", err)
	}
	if err := repo.CompleteAccountErasureFileObject(ctx, targetUserID, fileObjects[0].FileID, firstDeletedAt.Add(time.Minute)); err != nil {
		t.Fatalf("idempotent completion of first file object: %v", err)
	}
	assertCount(t, ctx, pool, 1, `SELECT COUNT(*) FROM file_erased_attachment_objects WHERE user_id = $1 AND deleted_at IS NOT NULL`, targetUserID)
	assertCount(t, ctx, pool, 1, `SELECT COUNT(*) FROM file_erased_file_objects WHERE user_id = $1 AND deleted_at IS NOT NULL`, targetUserID)
	assertCount(t, ctx, pool, 2, `SELECT deleted_objects FROM file_erased_users WHERE user_id = $1`, targetUserID)
	assertObjectKey(t, ctx, pool, targetAttachments[0].ID, fmt.Sprintf("erased/%d", targetAttachments[0].ID))
	assertFileObjectKey(t, ctx, pool, targetFiles[0].ID, fmt.Sprintf("erased/files/%d", targetFiles[0].ID))

	retryResult, retryObjects, err := repo.BeginAccountErasure(ctx, targetUserID, jobID, policyVersion)
	if err != nil {
		t.Fatalf("retry account erasure: %v", err)
	}
	if retryResult != (domain.AccountErasureResult{ArchivedAttachments: 2, DeletedDownloads: 3, DeletedObjects: 2}) {
		t.Fatalf("retry result = %+v, want two object completions carried forward", retryResult)
	}
	retryAttachmentObjects, retryFileObjects := splitErasureObjects(retryObjects)
	if len(retryAttachmentObjects) != 1 || retryAttachmentObjects[0].AttachmentID != attachmentObjects[1].AttachmentID ||
		len(retryFileObjects) != 1 || retryFileObjects[0].FileID != fileObjects[1].FileID {
		t.Fatalf("retry pending objects = %+v, want second attachment and second file", retryObjects)
	}

	if err := repo.CompleteAccountErasureObject(ctx, targetUserID, retryAttachmentObjects[0].AttachmentID, now.Add(3*time.Minute)); err != nil {
		t.Fatalf("complete second object: %v", err)
	}
	if err := repo.CompleteAccountErasureObject(ctx, targetUserID, retryAttachmentObjects[0].AttachmentID, now.Add(4*time.Minute)); err != nil {
		t.Fatalf("idempotent completion of second object: %v", err)
	}
	if err := repo.CompleteAccountErasureFileObject(ctx, targetUserID, retryFileObjects[0].FileID, now.Add(3*time.Minute)); err != nil {
		t.Fatalf("complete second file object: %v", err)
	}
	if err := repo.CompleteAccountErasureFileObject(ctx, targetUserID, retryFileObjects[0].FileID, now.Add(4*time.Minute)); err != nil {
		t.Fatalf("idempotent completion of second file object: %v", err)
	}
	completed, err := repo.CompleteAccountErasure(ctx, targetUserID, now.Add(5*time.Minute))
	if err != nil {
		t.Fatalf("complete account erasure: %v", err)
	}
	if completed != (domain.AccountErasureResult{ArchivedAttachments: 2, DeletedDownloads: 3, DeletedObjects: 4}) {
		t.Fatalf("completed result = %+v, want all attachment and file objects deleted", completed)
	}
	assertCount(t, ctx, pool, 0, `SELECT COUNT(*) FROM file_erased_attachment_objects WHERE user_id = $1 AND deleted_at IS NULL`, targetUserID)
	assertCount(t, ctx, pool, 0, `SELECT COUNT(*) FROM file_erased_file_objects WHERE user_id = $1 AND deleted_at IS NULL`, targetUserID)
	assertCount(t, ctx, pool, 1, `SELECT COUNT(*) FROM file_erased_users WHERE user_id = $1 AND completed_at IS NOT NULL`, targetUserID)

	replayed, replayObjects, err := repo.BeginAccountErasure(ctx, targetUserID, jobID, policyVersion)
	if err != nil {
		t.Fatalf("replay account erasure: %v", err)
	}
	if replayed != completed || len(replayObjects) != 0 {
		t.Fatalf("replay result = %+v, objects = %+v; want completed result and no work", replayed, replayObjects)
	}

	lateAttachment := domain.Attachment{
		TopicID:      seed + 20,
		OwnerID:      targetUserID,
		ObjectKey:    objectPrefix + "/late.bin",
		OriginalName: "late.bin",
		ContentType:  "application/octet-stream",
		SizeBytes:    1,
		Status:       domain.AttachmentStatusActive,
		CreatedAt:    now,
		UpdatedAt:    now,
	}
	if _, err := repo.CreateAttachment(ctx, lateAttachment, 1<<30); !errors.Is(err, domain.ErrAccountErased) {
		t.Fatalf("late attachment error = %v, want ErrAccountErased", err)
	}
	lateFile := domain.File{
		OwnerID:      targetUserID,
		BizType:      "drive",
		ObjectKey:    objectPrefix + "/late-file.bin",
		OriginalName: "late-file.bin",
		ContentType:  "application/octet-stream",
		SizeBytes:    1,
		Status:       domain.FileStatusActive,
		CreatedAt:    now,
		UpdatedAt:    now,
	}
	if _, err := repo.CreateFile(ctx, lateFile, 1<<30); !errors.Is(err, domain.ErrAccountErased) {
		t.Fatalf("late file error = %v, want ErrAccountErased", err)
	}
	if _, err := repo.EnsureDownload(ctx, otherAttachment.ID, targetUserID, objectPrefix+"/late-download", 0, now); !errors.Is(err, domain.ErrAccountErased) {
		t.Fatalf("late download error = %v, want ErrAccountErased", err)
	}
}

func splitErasureObjects(objects []domain.ErasureObject) ([]domain.ErasureObject, []domain.ErasureObject) {
	attachments := make([]domain.ErasureObject, 0)
	files := make([]domain.ErasureObject, 0)
	for _, object := range objects {
		if object.FileID > 0 {
			files = append(files, object)
		} else {
			attachments = append(attachments, object)
		}
	}
	return attachments, files
}

func assertErasedAttachment(t *testing.T, ctx context.Context, pool *pgxpool.Pool, attachmentID int64) {
	t.Helper()
	var ownerID, sizeBytes, priceCredits int64
	var objectKey, originalName, contentType, status string
	var archivedAt sql.NullTime
	if err := pool.QueryRow(ctx, `
SELECT owner_id, object_key, original_name, content_type, size_bytes, price_credits, status, archived_at
FROM attachments WHERE id = $1
`, attachmentID).Scan(&ownerID, &objectKey, &originalName, &contentType, &sizeBytes, &priceCredits, &status, &archivedAt); err != nil {
		t.Fatalf("read erased attachment %d: %v", attachmentID, err)
	}
	if ownerID != 0 || objectKey == "" || originalName != "erased-attachment" || contentType != "application/octet-stream" || sizeBytes != 0 || priceCredits != 0 || status != string(domain.AttachmentStatusArchived) || !archivedAt.Valid {
		t.Fatalf("attachment %d after erasure = owner=%d key=%q name=%q type=%q size=%d price=%d status=%q archived=%v", attachmentID, ownerID, objectKey, originalName, contentType, sizeBytes, priceCredits, status, archivedAt.Valid)
	}
}

func assertErasedFile(t *testing.T, ctx context.Context, pool *pgxpool.Pool, fileID int64) {
	t.Helper()
	var ownerID, sizeBytes int64
	var objectKey, originalName, contentType, status string
	var deletedAt sql.NullTime
	if err := pool.QueryRow(ctx, `
SELECT owner_user_id, object_key, original_name, content_type, size_bytes, status, deleted_at
FROM files WHERE id = $1
`, fileID).Scan(&ownerID, &objectKey, &originalName, &contentType, &sizeBytes, &status, &deletedAt); err != nil {
		t.Fatalf("read erased file %d: %v", fileID, err)
	}
	if ownerID != 0 || objectKey == "" || originalName != "erased-file" || contentType != "application/octet-stream" || sizeBytes != 0 || status != string(domain.FileStatusErased) || !deletedAt.Valid {
		t.Fatalf("file %d after erasure = owner=%d key=%q name=%q type=%q size=%d status=%q deleted=%v", fileID, ownerID, objectKey, originalName, contentType, sizeBytes, status, deletedAt.Valid)
	}
}

func assertObjectKey(t *testing.T, ctx context.Context, pool *pgxpool.Pool, attachmentID int64, want string) {
	t.Helper()
	var got string
	if err := pool.QueryRow(ctx, `SELECT object_key FROM attachments WHERE id = $1`, attachmentID).Scan(&got); err != nil {
		t.Fatalf("read object key for attachment %d: %v", attachmentID, err)
	}
	if got != want {
		t.Fatalf("attachment %d object key = %q, want %q", attachmentID, got, want)
	}
}

func assertFileObjectKey(t *testing.T, ctx context.Context, pool *pgxpool.Pool, fileID int64, want string) {
	t.Helper()
	var got string
	if err := pool.QueryRow(ctx, `SELECT object_key FROM files WHERE id = $1`, fileID).Scan(&got); err != nil {
		t.Fatalf("read object key for file %d: %v", fileID, err)
	}
	if got != want {
		t.Fatalf("file %d object key = %q, want %q", fileID, got, want)
	}
}

func assertCount(t *testing.T, ctx context.Context, pool *pgxpool.Pool, want int64, query string, args ...any) {
	t.Helper()
	var got int64
	if err := pool.QueryRow(ctx, query, args...).Scan(&got); err != nil {
		t.Fatalf("count query failed: %v", err)
	}
	if got != want {
		t.Fatalf("count = %d, want %d (query: %s)", got, want, query)
	}
}
