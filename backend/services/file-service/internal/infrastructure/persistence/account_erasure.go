package persistence

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"sort"
	"time"

	domain "file-service/internal/domain/file"

	"github.com/jackc/pgx/v5"
)

type accountErasureReceipt struct {
	ArchivedAttachments int64
	DeletedDownloads    int64
	DeletedObjects      int64
	PolicyVersion       int32
	CompletedAt         sql.NullTime
}

func (r *PostgresRepository) BeginAccountErasure(ctx context.Context, userID, deletionJobID int64, policyVersion int32) (domain.AccountErasureResult, []domain.ErasureObject, error) {
	if r == nil || r.pool == nil || userID <= 0 || deletionJobID <= 0 || policyVersion <= 0 {
		return domain.AccountErasureResult{}, nil, domain.ErrInvalidAccountErasure
	}
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return domain.AccountErasureResult{}, nil, err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	if err := lockFileUser(ctx, tx, userID); err != nil {
		return domain.AccountErasureResult{}, nil, err
	}
	if _, err := tx.Exec(ctx, `DELETE FROM file_user_capacity_overrides WHERE user_id = $1`, userID); err != nil {
		return domain.AccountErasureResult{}, nil, err
	}

	receipt, found, err := loadAccountErasureReceipt(ctx, tx, userID)
	if err != nil {
		return domain.AccountErasureResult{}, nil, err
	}
	if found && receipt.CompletedAt.Valid && policyVersion <= receipt.PolicyVersion {
		if err := tx.Commit(ctx); err != nil {
			return domain.AccountErasureResult{}, nil, err
		}
		return receipt.result(), nil, nil
	}
	if !found {
		if _, err := tx.Exec(ctx, `
INSERT INTO file_erased_users(user_id, deletion_job_id, policy_version)
VALUES($1, $2, $3)
`, userID, deletionJobID, policyVersion); err != nil {
			return domain.AccountErasureResult{}, nil, err
		}
		receipt.PolicyVersion = policyVersion
	} else if policyVersion > receipt.PolicyVersion {
		if _, err := tx.Exec(ctx, `
UPDATE file_erased_users
SET deletion_job_id = $2, policy_version = $3, completed_at = NULL
WHERE user_id = $1
`, userID, deletionJobID, policyVersion); err != nil {
			return domain.AccountErasureResult{}, nil, err
		}
		receipt.PolicyVersion = policyVersion
		receipt.CompletedAt = sql.NullTime{}
	}

	if _, err := tx.Exec(ctx, `
INSERT INTO file_erased_attachment_objects(user_id, attachment_id, object_key)
SELECT $1, id, object_key
FROM attachments
WHERE owner_id = $1
ON CONFLICT(user_id, attachment_id) DO NOTHING
`, userID); err != nil {
		return domain.AccountErasureResult{}, nil, err
	}
	if _, err := tx.Exec(ctx, `
INSERT INTO file_erased_file_objects(user_id, file_id, object_key)
SELECT $1, id, object_key
FROM files
WHERE owner_user_id = $1 AND status IN ('ACTIVE', 'DELETING')
ON CONFLICT(user_id, file_id) DO NOTHING
`, userID); err != nil {
		return domain.AccountErasureResult{}, nil, err
	}
	deletedDownloads, err := tx.Exec(ctx, `
DELETE FROM attachment_downloads
WHERE user_id = $1
   OR attachment_id IN (
        SELECT attachment_id FROM file_erased_attachment_objects WHERE user_id = $1
      )
`, userID)
	if err != nil {
		return domain.AccountErasureResult{}, nil, err
	}
	archivedAttachments, err := tx.Exec(ctx, `
UPDATE attachments
SET owner_id = 0,
    original_name = 'erased-attachment',
    content_type = 'application/octet-stream',
    size_bytes = 0,
    price_credits = 0,
    status = $2,
    archived_at = COALESCE(archived_at, NOW()),
    updated_at = NOW()
WHERE owner_id = $1
`, userID, domain.AttachmentStatusArchived)
	if err != nil {
		return domain.AccountErasureResult{}, nil, err
	}
	if _, err := tx.Exec(ctx, `
UPDATE files
SET owner_user_id = 0,
    original_name = 'erased-file',
    content_type = 'application/octet-stream',
    size_bytes = 0,
    status = 'ERASED',
    deleted_at = COALESCE(deleted_at, NOW()),
    updated_at = NOW()
WHERE owner_user_id = $1
`, userID); err != nil {
		return domain.AccountErasureResult{}, nil, err
	}
	receipt.ArchivedAttachments += archivedAttachments.RowsAffected()
	receipt.DeletedDownloads += deletedDownloads.RowsAffected()
	if _, err := tx.Exec(ctx, `
UPDATE file_erased_users
SET archived_attachments = $2, deleted_downloads = $3
WHERE user_id = $1
`, userID, receipt.ArchivedAttachments, receipt.DeletedDownloads); err != nil {
		return domain.AccountErasureResult{}, nil, err
	}
	objects, err := listPendingErasureObjects(ctx, tx, userID)
	if err != nil {
		return domain.AccountErasureResult{}, nil, err
	}
	if err := tx.Commit(ctx); err != nil {
		return domain.AccountErasureResult{}, nil, err
	}
	return receipt.result(), objects, nil
}

func (r *PostgresRepository) CompleteAccountErasureObject(ctx context.Context, userID, attachmentID int64, deletedAt time.Time) error {
	if r == nil || r.pool == nil || userID <= 0 || attachmentID <= 0 || deletedAt.IsZero() {
		return domain.ErrInvalidAccountErasure
	}
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	if err := lockFileUser(ctx, tx, userID); err != nil {
		return err
	}
	result, err := tx.Exec(ctx, `
UPDATE file_erased_attachment_objects
SET deleted_at = $3
WHERE user_id = $1 AND attachment_id = $2 AND deleted_at IS NULL
`, userID, attachmentID, deletedAt)
	if err != nil {
		return err
	}
	if result.RowsAffected() == 0 {
		var exists bool
		if err := tx.QueryRow(ctx, `
SELECT EXISTS (
  SELECT 1 FROM file_erased_attachment_objects
  WHERE user_id = $1 AND attachment_id = $2 AND deleted_at IS NOT NULL
)
`, userID, attachmentID).Scan(&exists); err != nil {
			return err
		}
		if !exists {
			return domain.ErrInvalidAccountErasure
		}
		return tx.Commit(ctx)
	}
	if _, err := tx.Exec(ctx, `
UPDATE attachments
SET object_key = $2, updated_at = $3
WHERE id = $1
`, attachmentID, fmt.Sprintf("erased/%d", attachmentID), deletedAt); err != nil {
		return err
	}
	if _, err := tx.Exec(ctx, `
UPDATE file_erased_users
SET deleted_objects = deleted_objects + 1
WHERE user_id = $1
`, userID); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

func (r *PostgresRepository) CompleteAccountErasureFileObject(ctx context.Context, userID, fileID int64, deletedAt time.Time) error {
	if r == nil || r.pool == nil || userID <= 0 || fileID <= 0 || deletedAt.IsZero() {
		return domain.ErrInvalidAccountErasure
	}
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	if err := lockFileUser(ctx, tx, userID); err != nil {
		return err
	}
	result, err := tx.Exec(ctx, `
UPDATE file_erased_file_objects
SET deleted_at = $3
WHERE user_id = $1 AND file_id = $2 AND deleted_at IS NULL
`, userID, fileID, deletedAt)
	if err != nil {
		return err
	}
	if result.RowsAffected() == 0 {
		var exists bool
		if err := tx.QueryRow(ctx, `
SELECT EXISTS (
  SELECT 1 FROM file_erased_file_objects
  WHERE user_id = $1 AND file_id = $2 AND deleted_at IS NOT NULL
)
`, userID, fileID).Scan(&exists); err != nil {
			return err
		}
		if !exists {
			return domain.ErrInvalidAccountErasure
		}
		return tx.Commit(ctx)
	}
	if _, err := tx.Exec(ctx, `
UPDATE files
SET object_key = $2, updated_at = $3
WHERE id = $1
`, fileID, fmt.Sprintf("erased/files/%d", fileID), deletedAt); err != nil {
		return err
	}
	if _, err := tx.Exec(ctx, `
UPDATE file_erased_users
SET deleted_objects = deleted_objects + 1
WHERE user_id = $1
`, userID); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

func (r *PostgresRepository) CompleteAccountErasure(ctx context.Context, userID int64, completedAt time.Time) (domain.AccountErasureResult, error) {
	if r == nil || r.pool == nil || userID <= 0 || completedAt.IsZero() {
		return domain.AccountErasureResult{}, domain.ErrInvalidAccountErasure
	}
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return domain.AccountErasureResult{}, err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	if err := lockFileUser(ctx, tx, userID); err != nil {
		return domain.AccountErasureResult{}, err
	}
	var pending int64
	if err := tx.QueryRow(ctx, `
SELECT (
  SELECT COUNT(*) FROM file_erased_attachment_objects WHERE user_id = $1 AND deleted_at IS NULL
) + (
  SELECT COUNT(*) FROM file_erased_file_objects WHERE user_id = $1 AND deleted_at IS NULL
)
`, userID).Scan(&pending); err != nil {
		return domain.AccountErasureResult{}, err
	}
	if pending != 0 {
		return domain.AccountErasureResult{}, domain.ErrAccountErasureUnavailable
	}
	if _, err := tx.Exec(ctx, `
UPDATE file_erased_users SET completed_at = COALESCE(completed_at, $2) WHERE user_id = $1
`, userID, completedAt); err != nil {
		return domain.AccountErasureResult{}, err
	}
	receipt, found, err := loadAccountErasureReceipt(ctx, tx, userID)
	if err != nil {
		return domain.AccountErasureResult{}, err
	}
	if !found {
		return domain.AccountErasureResult{}, domain.ErrInvalidAccountErasure
	}
	if err := tx.Commit(ctx); err != nil {
		return domain.AccountErasureResult{}, err
	}
	return receipt.result(), nil
}

func loadAccountErasureReceipt(ctx context.Context, tx pgx.Tx, userID int64) (accountErasureReceipt, bool, error) {
	var receipt accountErasureReceipt
	err := tx.QueryRow(ctx, `
SELECT archived_attachments, deleted_downloads, deleted_objects, policy_version, completed_at
FROM file_erased_users
WHERE user_id = $1
FOR UPDATE
`, userID).Scan(&receipt.ArchivedAttachments, &receipt.DeletedDownloads, &receipt.DeletedObjects, &receipt.PolicyVersion, &receipt.CompletedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return accountErasureReceipt{}, false, nil
	}
	return receipt, err == nil, err
}

func listPendingErasureObjects(ctx context.Context, tx pgx.Tx, userID int64) ([]domain.ErasureObject, error) {
	rows, err := tx.Query(ctx, `
SELECT attachment_id, 0::BIGINT AS file_id, object_key
FROM file_erased_attachment_objects
WHERE user_id = $1 AND deleted_at IS NULL
UNION ALL
SELECT 0::BIGINT AS attachment_id, file_id, object_key
FROM file_erased_file_objects
WHERE user_id = $1 AND deleted_at IS NULL
ORDER BY attachment_id, file_id
`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	objects := make([]domain.ErasureObject, 0)
	for rows.Next() {
		var object domain.ErasureObject
		if err := rows.Scan(&object.AttachmentID, &object.FileID, &object.ObjectKey); err != nil {
			return nil, err
		}
		objects = append(objects, object)
	}
	return objects, rows.Err()
}

func (r accountErasureReceipt) result() domain.AccountErasureResult {
	return domain.AccountErasureResult{
		ArchivedAttachments: r.ArchivedAttachments,
		DeletedDownloads:    r.DeletedDownloads,
		DeletedObjects:      r.DeletedObjects,
	}
}

func lockFileUser(ctx context.Context, tx pgx.Tx, userID int64) error {
	_, err := tx.Exec(ctx, `SELECT pg_advisory_xact_lock($1)`, userID)
	return err
}

func ensureFileUserActive(ctx context.Context, tx pgx.Tx, userID int64) error {
	return ensureFileUsersActive(ctx, tx, userID)
}

func ensureFileUsersActive(ctx context.Context, tx pgx.Tx, userIDs ...int64) error {
	unique := make(map[int64]struct{}, len(userIDs))
	ids := make([]int64, 0, len(userIDs))
	for _, userID := range userIDs {
		if userID <= 0 {
			return domain.ErrInvalidAttachment
		}
		if _, exists := unique[userID]; !exists {
			unique[userID] = struct{}{}
			ids = append(ids, userID)
		}
	}
	sort.Slice(ids, func(i, j int) bool { return ids[i] < ids[j] })
	for _, userID := range ids {
		if err := lockFileUser(ctx, tx, userID); err != nil {
			return err
		}
	}
	var erased bool
	if err := tx.QueryRow(ctx, `SELECT EXISTS (SELECT 1 FROM file_erased_users WHERE user_id = ANY($1::BIGINT[]))`, ids).Scan(&erased); err != nil {
		return err
	}
	if erased {
		return domain.ErrAccountErased
	}
	return nil
}

var _ domain.AccountErasureRepository = (*PostgresRepository)(nil)
