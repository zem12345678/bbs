package persistence

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	domain "file-service/internal/domain/file"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

type PostgresRepository struct {
	pool *pgxpool.Pool
}

func NewPostgresRepository(pool *pgxpool.Pool) *PostgresRepository {
	return &PostgresRepository{pool: pool}
}

func (r *PostgresRepository) EnsureSchema(ctx context.Context) error {
	for _, statement := range schemaStatements {
		if _, err := r.pool.Exec(ctx, statement); err != nil {
			return err
		}
	}
	return nil
}

func (r *PostgresRepository) CreateFile(ctx context.Context, file domain.File, capacityBytes int64) (domain.File, error) {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return domain.File{}, err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	if err := ensureFileUserActive(ctx, tx, file.OwnerID); err != nil {
		return domain.File{}, err
	}
	if err := ensureOwnedFolder(ctx, tx, file.OwnerID, file.FolderID); err != nil {
		return domain.File{}, err
	}
	capacityBytes, err = queryEffectiveFileCapacity(ctx, tx, file.OwnerID, capacityBytes)
	if err != nil {
		return domain.File{}, err
	}
	if capacityBytes <= 0 || file.SizeBytes > capacityBytes {
		return domain.File{}, domain.ErrFileCapacityExceeded
	}
	usedBytes, err := queryUserStorageUsage(ctx, tx, file.OwnerID)
	if err != nil {
		return domain.File{}, err
	}
	if usedBytes > capacityBytes-file.SizeBytes {
		return domain.File{}, domain.ErrFileCapacityExceeded
	}
	err = scanFile(tx.QueryRow(ctx, `
INSERT INTO files(owner_user_id, biz_type, object_key, original_name, content_type, size_bytes, status, created_at, updated_at, folder_id, is_sensitive, comment)
VALUES($1, $2, $3, $4, $5, $6, $7, $8, $9, NULLIF($10, 0), $11, $12)
RETURNING id, owner_user_id, biz_type, object_key, original_name, content_type, size_bytes, status, created_at, updated_at, deleted_at,
          COALESCE(folder_id, 0), is_sensitive, comment
`, file.OwnerID, file.BizType, file.ObjectKey, file.OriginalName, file.ContentType, file.SizeBytes, file.Status, file.CreatedAt, file.UpdatedAt, file.FolderID, file.IsSensitive, file.Comment), &file)
	if isUniqueViolation(err) {
		return domain.File{}, domain.ErrFileObjectKeyTaken
	}
	if err != nil {
		return domain.File{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return domain.File{}, err
	}
	return file, nil
}

func (r *PostgresRepository) GetFileUsage(ctx context.Context, userID int64) (int64, error) {
	return queryUserStorageUsage(ctx, r.pool, userID)
}

func (r *PostgresRepository) GetFileCount(ctx context.Context, userID int64) (int64, error) {
	var count int64
	err := r.pool.QueryRow(ctx, `
SELECT COUNT(*)::BIGINT
FROM (
	SELECT id
	FROM files
	WHERE owner_user_id = $1 AND status IN ('ACTIVE', 'DELETING')
	UNION ALL
	SELECT id
	FROM attachments
	WHERE owner_id = $1 AND status IN ('ACTIVE', 'ARCHIVED')
) AS capacity_items
`, userID).Scan(&count)
	return count, err
}

func (r *PostgresRepository) GetFileCapacityOverride(ctx context.Context, userID int64) (*int64, error) {
	return queryFileCapacityOverride(ctx, r.pool, userID)
}

func (r *PostgresRepository) SetFileCapacityOverride(ctx context.Context, userID int64, overrideBytes *int64, updatedAt time.Time) error {
	if userID <= 0 || (overrideBytes != nil && *overrideBytes < 0) {
		return domain.ErrInvalidFileCapacity
	}
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	if err := ensureFileUserActive(ctx, tx, userID); err != nil {
		return err
	}
	if overrideBytes == nil {
		_, err = tx.Exec(ctx, `DELETE FROM file_user_capacity_overrides WHERE user_id = $1`, userID)
	} else {
		_, err = tx.Exec(ctx, `
INSERT INTO file_user_capacity_overrides(user_id, capacity_bytes, updated_at)
VALUES($1, $2, $3)
ON CONFLICT(user_id) DO UPDATE
SET capacity_bytes = EXCLUDED.capacity_bytes,
    updated_at = EXCLUDED.updated_at
`, userID, *overrideBytes, updatedAt)
	}
	if err != nil {
		return err
	}
	return tx.Commit(ctx)
}

func (r *PostgresRepository) ListUserFiles(ctx context.Context, userID int64, limit, offset int32) ([]domain.File, int64, error) {
	var total int64
	if err := r.pool.QueryRow(ctx, `
SELECT COUNT(*) FROM files
WHERE owner_user_id = $1 AND status IN ('ACTIVE', 'DELETING')
`, userID).Scan(&total); err != nil {
		return nil, 0, err
	}
	rows, err := r.pool.Query(ctx, `
SELECT id, owner_user_id, biz_type, object_key, original_name, content_type, size_bytes, status, created_at, updated_at, deleted_at,
       COALESCE(folder_id, 0), is_sensitive, comment
FROM files
WHERE owner_user_id = $1 AND status IN ('ACTIVE', 'DELETING')
ORDER BY created_at DESC, id DESC
LIMIT $2 OFFSET $3
`, userID, limit, offset)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()
	items := make([]domain.File, 0)
	for rows.Next() {
		var item domain.File
		if err := scanFile(rows, &item); err != nil {
			return nil, 0, err
		}
		items = append(items, item)
	}
	return items, total, rows.Err()
}

func (r *PostgresRepository) GetFile(ctx context.Context, userID, fileID int64) (domain.File, error) {
	var item domain.File
	err := scanFile(r.pool.QueryRow(ctx, `
SELECT id, owner_user_id, biz_type, object_key, original_name, content_type, size_bytes, status, created_at, updated_at, deleted_at,
       COALESCE(folder_id, 0), is_sensitive, comment
FROM files
WHERE id = $1 AND owner_user_id = $2 AND status IN ('ACTIVE', 'DELETING')
`, fileID, userID), &item)
	if errors.Is(err, pgx.ErrNoRows) {
		var ownerID int64
		var status string
		if ownerErr := r.pool.QueryRow(ctx, `SELECT owner_user_id, status FROM files WHERE id = $1`, fileID).Scan(&ownerID, &status); errors.Is(ownerErr, pgx.ErrNoRows) {
			return domain.File{}, domain.ErrFileNotFound
		} else if ownerErr != nil {
			return domain.File{}, ownerErr
		} else if ownerID != userID {
			return domain.File{}, domain.ErrFileOwnerMismatch
		} else if status == string(domain.FileStatusDeleted) || status == string(domain.FileStatusErased) {
			return domain.File{}, domain.ErrFileDeleted
		}
		return domain.File{}, domain.ErrFileNotFound
	}
	return item, err
}

func (r *PostgresRepository) BeginFileDeletion(ctx context.Context, userID, fileID int64, updatedAt time.Time) (domain.File, error) {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return domain.File{}, err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	if err := ensureFileUserActive(ctx, tx, userID); err != nil {
		return domain.File{}, err
	}
	var item domain.File
	err = scanFile(tx.QueryRow(ctx, `
UPDATE files
SET status = $3, updated_at = $4
WHERE id = $1 AND owner_user_id = $2 AND status IN ('ACTIVE', 'DELETING')
RETURNING id, owner_user_id, biz_type, object_key, original_name, content_type, size_bytes, status, created_at, updated_at, deleted_at,
          COALESCE(folder_id, 0), is_sensitive, comment
`, fileID, userID, domain.FileStatusDeleting, updatedAt), &item)
	if errors.Is(err, pgx.ErrNoRows) {
		var ownerID int64
		var status string
		if lookupErr := tx.QueryRow(ctx, `SELECT owner_user_id, status FROM files WHERE id = $1`, fileID).Scan(&ownerID, &status); errors.Is(lookupErr, pgx.ErrNoRows) {
			return domain.File{}, domain.ErrFileNotFound
		} else if lookupErr != nil {
			return domain.File{}, lookupErr
		} else if ownerID != userID {
			return domain.File{}, domain.ErrFileOwnerMismatch
		} else if status == string(domain.FileStatusDeleted) || status == string(domain.FileStatusErased) {
			return domain.File{}, domain.ErrFileDeleted
		}
		return domain.File{}, domain.ErrInvalidFile
	}
	if err != nil {
		return domain.File{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return domain.File{}, err
	}
	return item, nil
}

func (r *PostgresRepository) CompleteFileDeletion(ctx context.Context, userID, fileID int64, deletedAt time.Time) (domain.File, error) {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return domain.File{}, err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	if err := ensureFileUserActive(ctx, tx, userID); err != nil {
		return domain.File{}, err
	}
	var item domain.File
	err = scanFile(tx.QueryRow(ctx, `
UPDATE files
SET object_key = $3, folder_id = NULL, status = $4, deleted_at = $5, updated_at = $5
WHERE id = $1 AND owner_user_id = $2 AND status = 'DELETING'
RETURNING id, owner_user_id, biz_type, object_key, original_name, content_type, size_bytes, status, created_at, updated_at, deleted_at,
          COALESCE(folder_id, 0), is_sensitive, comment
`, fileID, userID, fmt.Sprintf("deleted/files/%d", fileID), domain.FileStatusDeleted, deletedAt), &item)
	if errors.Is(err, pgx.ErrNoRows) {
		if lookupErr := scanFile(tx.QueryRow(ctx, `
SELECT id, owner_user_id, biz_type, object_key, original_name, content_type, size_bytes, status, created_at, updated_at, deleted_at,
       COALESCE(folder_id, 0), is_sensitive, comment
FROM files WHERE id = $1 AND owner_user_id = $2
`, fileID, userID), &item); lookupErr != nil {
			if errors.Is(lookupErr, pgx.ErrNoRows) {
				return domain.File{}, domain.ErrFileNotFound
			}
			return domain.File{}, lookupErr
		}
		if item.Status == domain.FileStatusDeleted {
			if err := tx.Commit(ctx); err != nil {
				return domain.File{}, err
			}
			return item, nil
		}
		return domain.File{}, domain.ErrInvalidFile
	}
	if err != nil {
		return domain.File{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return domain.File{}, err
	}
	return item, nil
}

func (r *PostgresRepository) CreateAttachment(ctx context.Context, attachment domain.Attachment, capacityBytes int64) (domain.Attachment, error) {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return domain.Attachment{}, err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	if err := ensureFileUserActive(ctx, tx, attachment.OwnerID); err != nil {
		return domain.Attachment{}, err
	}
	capacityBytes, err = queryEffectiveFileCapacity(ctx, tx, attachment.OwnerID, capacityBytes)
	if err != nil {
		return domain.Attachment{}, err
	}
	if capacityBytes <= 0 || attachment.SizeBytes > capacityBytes {
		return domain.Attachment{}, domain.ErrFileCapacityExceeded
	}
	usedBytes, err := queryUserStorageUsage(ctx, tx, attachment.OwnerID)
	if err != nil {
		return domain.Attachment{}, err
	}
	if usedBytes > capacityBytes-attachment.SizeBytes {
		return domain.Attachment{}, domain.ErrFileCapacityExceeded
	}
	err = scanAttachment(tx.QueryRow(ctx, `
INSERT INTO attachments(topic_id, owner_id, object_key, original_name, content_type, size_bytes, price_credits, status, created_at, updated_at)
VALUES($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
RETURNING id, topic_id, owner_id, object_key, original_name, content_type, size_bytes, price_credits, status, created_at, updated_at, archived_at
`, attachment.TopicID, attachment.OwnerID, attachment.ObjectKey, attachment.OriginalName, attachment.ContentType, attachment.SizeBytes, attachment.PriceCredits, attachment.Status, attachment.CreatedAt, attachment.UpdatedAt), &attachment)
	if isUniqueViolation(err) {
		return domain.Attachment{}, domain.ErrAttachmentObjectKeyTaken
	}
	if err != nil {
		return domain.Attachment{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return domain.Attachment{}, err
	}
	return attachment, nil
}

func (r *PostgresRepository) ListTopicAttachments(ctx context.Context, topicID int64) ([]domain.Attachment, error) {
	rows, err := r.pool.Query(ctx, `
SELECT id, topic_id, owner_id, object_key, original_name, content_type, size_bytes, price_credits, status, created_at, updated_at, archived_at
FROM attachments
WHERE topic_id = $1 AND status = $2
ORDER BY created_at ASC, id ASC
`, topicID, domain.AttachmentStatusActive)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	attachments := make([]domain.Attachment, 0)
	for rows.Next() {
		var attachment domain.Attachment
		if err := scanAttachment(rows, &attachment); err != nil {
			return nil, err
		}
		attachments = append(attachments, attachment)
	}
	return attachments, rows.Err()
}

func (r *PostgresRepository) ListUserAttachmentDownloads(ctx context.Context, userID, topicID int64, limit, offset int32) (domain.AttachmentDownloadList, error) {
	result := domain.AttachmentDownloadList{}
	if err := r.pool.QueryRow(ctx, `
SELECT COUNT(*)
FROM attachment_downloads d
JOIN attachments a ON a.id = d.attachment_id
WHERE d.user_id = $1 AND d.status = $2 AND ($3::BIGINT = 0 OR a.topic_id = $3::BIGINT)
`, userID, domain.DownloadStatusAuthorized, topicID).Scan(&result.Total); err != nil {
		return domain.AttachmentDownloadList{}, err
	}

	rows, err := r.pool.Query(ctx, `
SELECT a.id, a.topic_id, a.owner_id, a.object_key, a.original_name, a.content_type, a.size_bytes, a.price_credits, a.status, a.created_at, a.updated_at, a.archived_at,
	       d.status, d.charged_credits, d.created_at, d.authorized_at
FROM attachment_downloads d
JOIN attachments a ON a.id = d.attachment_id
WHERE d.user_id = $1 AND d.status = $2 AND ($3::BIGINT = 0 OR a.topic_id = $3::BIGINT)
ORDER BY d.created_at DESC, d.attachment_id DESC
LIMIT $4 OFFSET $5
`, userID, domain.DownloadStatusAuthorized, topicID, limit, offset)
	if err != nil {
		return domain.AttachmentDownloadList{}, err
	}
	defer rows.Close()

	result.Items = make([]domain.AttachmentDownload, 0)
	for rows.Next() {
		var download domain.AttachmentDownload
		if err := scanAttachmentDownload(rows, &download); err != nil {
			return domain.AttachmentDownloadList{}, err
		}
		result.Items = append(result.Items, download)
	}
	return result, rows.Err()
}

func (r *PostgresRepository) ListUserAttachmentSales(ctx context.Context, userID int64, limit, offset int32) (domain.AttachmentSaleList, error) {
	result := domain.AttachmentSaleList{}
	if err := r.pool.QueryRow(ctx, `
SELECT COUNT(*), COALESCE(SUM(d.charged_credits), 0)::BIGINT
FROM attachment_downloads d
JOIN attachments a ON a.id = d.attachment_id
WHERE a.owner_id = $1 AND d.status = $2 AND d.charged_credits > 0
`, userID, domain.DownloadStatusAuthorized).Scan(&result.Total, &result.TotalEarnedCredits); err != nil {
		return domain.AttachmentSaleList{}, err
	}

	rows, err := r.pool.Query(ctx, `
SELECT a.id, a.topic_id, a.owner_id, a.object_key, a.original_name, a.content_type, a.size_bytes, a.price_credits, a.status, a.created_at, a.updated_at, a.archived_at,
	       d.charged_credits, d.authorized_at
FROM attachment_downloads d
JOIN attachments a ON a.id = d.attachment_id
WHERE a.owner_id = $1 AND d.status = $2 AND d.charged_credits > 0
ORDER BY d.authorized_at DESC, d.attachment_id DESC
LIMIT $3 OFFSET $4
`, userID, domain.DownloadStatusAuthorized, limit, offset)
	if err != nil {
		return domain.AttachmentSaleList{}, err
	}
	defer rows.Close()

	result.Items = make([]domain.AttachmentSale, 0)
	for rows.Next() {
		var sale domain.AttachmentSale
		if err := scanAttachmentSale(rows, &sale); err != nil {
			return domain.AttachmentSaleList{}, err
		}
		result.Items = append(result.Items, sale)
	}
	return result, rows.Err()
}

func (r *PostgresRepository) GetAttachment(ctx context.Context, attachmentID int64) (domain.Attachment, error) {
	var attachment domain.Attachment
	err := scanAttachment(r.pool.QueryRow(ctx, `
SELECT id, topic_id, owner_id, object_key, original_name, content_type, size_bytes, price_credits, status, created_at, updated_at, archived_at
FROM attachments WHERE id = $1
`, attachmentID), &attachment)
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.Attachment{}, domain.ErrAttachmentNotFound
	}
	return attachment, err
}

func (r *PostgresRepository) GetDownload(ctx context.Context, attachmentID, userID int64) (domain.Download, bool, error) {
	var download domain.Download
	err := scanDownload(r.pool.QueryRow(ctx, `
SELECT attachment_id, user_id, status, source_event_id, charged_credits, created_at, authorized_at
FROM attachment_downloads
WHERE attachment_id = $1 AND user_id = $2
`, attachmentID, userID), &download)
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.Download{}, false, nil
	}
	if err != nil {
		return domain.Download{}, false, err
	}
	return download, true, nil
}

func (r *PostgresRepository) ArchiveAttachment(ctx context.Context, attachmentID, ownerID int64, archivedAt time.Time) (domain.Attachment, error) {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return domain.Attachment{}, err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	if err := ensureFileUserActive(ctx, tx, ownerID); err != nil {
		return domain.Attachment{}, err
	}
	var attachment domain.Attachment
	err = scanAttachment(tx.QueryRow(ctx, `
UPDATE attachments
SET status = $3, archived_at = $4, updated_at = $4
WHERE id = $1 AND owner_id = $2 AND status = $5
RETURNING id, topic_id, owner_id, object_key, original_name, content_type, size_bytes, price_credits, status, created_at, updated_at, archived_at
`, attachmentID, ownerID, domain.AttachmentStatusArchived, archivedAt, domain.AttachmentStatusActive), &attachment)
	if !errors.Is(err, pgx.ErrNoRows) {
		if err != nil {
			return domain.Attachment{}, err
		}
		if err := tx.Commit(ctx); err != nil {
			return domain.Attachment{}, err
		}
		return attachment, nil
	}

	var existing domain.Attachment
	if err := scanAttachment(tx.QueryRow(ctx, `
SELECT id, topic_id, owner_id, object_key, original_name, content_type, size_bytes, price_credits, status, created_at, updated_at, archived_at
FROM attachments WHERE id = $1
`, attachmentID), &existing); errors.Is(err, pgx.ErrNoRows) {
		return domain.Attachment{}, domain.ErrAttachmentNotFound
	} else if err != nil {
		return domain.Attachment{}, err
	}
	if existing.OwnerID != ownerID {
		return domain.Attachment{}, domain.ErrAttachmentOwnerMismatch
	}
	return domain.Attachment{}, domain.ErrAttachmentArchived
}

func (r *PostgresRepository) UpdateAttachmentPrice(ctx context.Context, attachmentID, ownerID, priceCredits int64, updatedAt time.Time) (domain.Attachment, error) {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return domain.Attachment{}, err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	if err := ensureFileUserActive(ctx, tx, ownerID); err != nil {
		return domain.Attachment{}, err
	}
	var attachment domain.Attachment
	err = scanAttachment(tx.QueryRow(ctx, `
UPDATE attachments
SET price_credits = $3, updated_at = $4
WHERE id = $1 AND owner_id = $2 AND status = $5
RETURNING id, topic_id, owner_id, object_key, original_name, content_type, size_bytes, price_credits, status, created_at, updated_at, archived_at
`, attachmentID, ownerID, priceCredits, updatedAt, domain.AttachmentStatusActive), &attachment)
	if !errors.Is(err, pgx.ErrNoRows) {
		if err != nil {
			return domain.Attachment{}, err
		}
		if err := tx.Commit(ctx); err != nil {
			return domain.Attachment{}, err
		}
		return attachment, nil
	}

	var existing domain.Attachment
	if err := scanAttachment(tx.QueryRow(ctx, `
SELECT id, topic_id, owner_id, object_key, original_name, content_type, size_bytes, price_credits, status, created_at, updated_at, archived_at
FROM attachments WHERE id = $1
`, attachmentID), &existing); errors.Is(err, pgx.ErrNoRows) {
		return domain.Attachment{}, domain.ErrAttachmentNotFound
	} else if err != nil {
		return domain.Attachment{}, err
	}
	if existing.OwnerID != ownerID {
		return domain.Attachment{}, domain.ErrAttachmentOwnerMismatch
	}
	return domain.Attachment{}, domain.ErrAttachmentArchived
}

func (r *PostgresRepository) EnsureDownload(ctx context.Context, attachmentID, userID int64, sourceEventID string, chargedCredits int64, createdAt time.Time) (domain.Download, error) {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return domain.Download{}, err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	_, err = lockActiveAttachmentForDownload(ctx, tx, attachmentID, userID)
	if err != nil {
		return domain.Download{}, err
	}
	var download domain.Download
	err = scanDownload(tx.QueryRow(ctx, `
INSERT INTO attachment_downloads(attachment_id, user_id, status, source_event_id, charged_credits, created_at)
VALUES($1, $2, $3, $4, $5, $6)
ON CONFLICT(attachment_id, user_id) DO UPDATE SET attachment_id = attachment_downloads.attachment_id
RETURNING attachment_id, user_id, status, source_event_id, charged_credits, created_at, authorized_at
`, attachmentID, userID, domain.DownloadStatusPending, sourceEventID, chargedCredits, createdAt), &download)
	if isUniqueViolation(err) {
		return domain.Download{}, domain.ErrDownloadRecordMismatch
	}
	if err != nil {
		return domain.Download{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return domain.Download{}, err
	}
	return download, nil
}

func (r *PostgresRepository) CompleteDownloadAuthorization(ctx context.Context, attachmentID, userID int64, authorizedAt time.Time, settle func(context.Context) error) (_ domain.Download, _ bool, err error) {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return domain.Download{}, false, err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	_, err = lockActiveAttachmentForDownload(ctx, tx, attachmentID, userID)
	if err != nil {
		return domain.Download{}, false, err
	}

	var download domain.Download
	err = scanDownload(tx.QueryRow(ctx, `
SELECT attachment_id, user_id, status, source_event_id, charged_credits, created_at, authorized_at
FROM attachment_downloads
WHERE attachment_id = $1 AND user_id = $2
FOR UPDATE
`, attachmentID, userID), &download)
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.Download{}, false, domain.ErrDownloadRecordMismatch
	}
	if err != nil {
		return domain.Download{}, false, err
	}
	if download.Status == domain.DownloadStatusAuthorized {
		if err := tx.Commit(ctx); err != nil {
			return domain.Download{}, false, err
		}
		return download, true, nil
	}
	if download.Status != domain.DownloadStatusPending {
		return domain.Download{}, false, domain.ErrDownloadRecordMismatch
	}
	if settle == nil {
		return domain.Download{}, false, domain.ErrCreditServiceUnavailable
	}
	if err := settle(ctx); err != nil {
		return domain.Download{}, false, err
	}

	err = scanDownload(tx.QueryRow(ctx, `
UPDATE attachment_downloads
SET status = $3, authorized_at = $4
WHERE attachment_id = $1 AND user_id = $2 AND status = $5
RETURNING attachment_id, user_id, status, source_event_id, charged_credits, created_at, authorized_at
`, attachmentID, userID, domain.DownloadStatusAuthorized, authorizedAt, domain.DownloadStatusPending), &download)
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.Download{}, false, domain.ErrDownloadRecordMismatch
	}
	if err != nil {
		return domain.Download{}, false, err
	}
	if err := tx.Commit(ctx); err != nil {
		return domain.Download{}, false, err
	}
	return download, false, nil
}

func lockActiveAttachmentForDownload(ctx context.Context, tx pgx.Tx, attachmentID, userID int64) (domain.Attachment, error) {
	var ownerID int64
	if err := tx.QueryRow(ctx, `SELECT owner_id FROM attachments WHERE id = $1`, attachmentID).Scan(&ownerID); errors.Is(err, pgx.ErrNoRows) {
		return domain.Attachment{}, domain.ErrAttachmentNotFound
	} else if err != nil {
		return domain.Attachment{}, err
	}
	if err := ensureFileUsersActive(ctx, tx, userID, ownerID); err != nil {
		return domain.Attachment{}, err
	}
	var attachment domain.Attachment
	err := scanAttachment(tx.QueryRow(ctx, `
SELECT id, topic_id, owner_id, object_key, original_name, content_type, size_bytes, price_credits, status, created_at, updated_at, archived_at
FROM attachments
WHERE id = $1
FOR UPDATE
`, attachmentID), &attachment)
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.Attachment{}, domain.ErrAttachmentNotFound
	}
	if err != nil {
		return domain.Attachment{}, err
	}
	if attachment.OwnerID != ownerID {
		return domain.Attachment{}, domain.ErrDownloadRecordMismatch
	}
	if attachment.Status != domain.AttachmentStatusActive {
		return domain.Attachment{}, domain.ErrAttachmentArchived
	}
	return attachment, nil
}

type rowScanner interface {
	Scan(...any) error
}

type rowQuerier interface {
	QueryRow(context.Context, string, ...any) pgx.Row
}

func queryUserStorageUsage(ctx context.Context, querier rowQuerier, userID int64) (int64, error) {
	var usedBytes int64
	err := querier.QueryRow(ctx, `
SELECT COALESCE(SUM(size_bytes), 0)::BIGINT
FROM (
	SELECT size_bytes
	FROM files
	WHERE owner_user_id = $1 AND status IN ('ACTIVE', 'DELETING')
	UNION ALL
	SELECT size_bytes
	FROM attachments
	WHERE owner_id = $1 AND status IN ('ACTIVE', 'ARCHIVED')
) AS capacity_items
`, userID).Scan(&usedBytes)
	return usedBytes, err
}

func queryFileCapacityOverride(ctx context.Context, querier rowQuerier, userID int64) (*int64, error) {
	var overrideBytes int64
	err := querier.QueryRow(ctx, `
SELECT capacity_bytes
FROM file_user_capacity_overrides
WHERE user_id = $1
`, userID).Scan(&overrideBytes)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &overrideBytes, nil
}

func queryEffectiveFileCapacity(ctx context.Context, querier rowQuerier, userID, policyBytes int64) (int64, error) {
	overrideBytes, err := queryFileCapacityOverride(ctx, querier, userID)
	if err != nil {
		return 0, err
	}
	if overrideBytes != nil && *overrideBytes > policyBytes {
		return *overrideBytes, nil
	}
	return policyBytes, nil
}

func scanAttachment(row rowScanner, attachment *domain.Attachment) error {
	var status string
	err := row.Scan(
		&attachment.ID,
		&attachment.TopicID,
		&attachment.OwnerID,
		&attachment.ObjectKey,
		&attachment.OriginalName,
		&attachment.ContentType,
		&attachment.SizeBytes,
		&attachment.PriceCredits,
		&status,
		&attachment.CreatedAt,
		&attachment.UpdatedAt,
		&attachment.ArchivedAt,
	)
	attachment.Status = domain.AttachmentStatus(status)
	return err
}

func scanFile(row rowScanner, item *domain.File) error {
	var status string
	err := row.Scan(
		&item.ID,
		&item.OwnerID,
		&item.BizType,
		&item.ObjectKey,
		&item.OriginalName,
		&item.ContentType,
		&item.SizeBytes,
		&status,
		&item.CreatedAt,
		&item.UpdatedAt,
		&item.DeletedAt,
		&item.FolderID,
		&item.IsSensitive,
		&item.Comment,
	)
	item.Status = domain.FileStatus(status)
	return err
}

func scanDownload(row rowScanner, download *domain.Download) error {
	return row.Scan(
		&download.AttachmentID,
		&download.UserID,
		&download.Status,
		&download.SourceEventID,
		&download.ChargedCredits,
		&download.CreatedAt,
		&download.AuthorizedAt,
	)
}

func scanAttachmentDownload(row rowScanner, download *domain.AttachmentDownload) error {
	var attachmentStatus string
	var downloadStatus string
	err := row.Scan(
		&download.Attachment.ID,
		&download.Attachment.TopicID,
		&download.Attachment.OwnerID,
		&download.Attachment.ObjectKey,
		&download.Attachment.OriginalName,
		&download.Attachment.ContentType,
		&download.Attachment.SizeBytes,
		&download.Attachment.PriceCredits,
		&attachmentStatus,
		&download.Attachment.CreatedAt,
		&download.Attachment.UpdatedAt,
		&download.Attachment.ArchivedAt,
		&downloadStatus,
		&download.ChargedCredits,
		&download.CreatedAt,
		&download.AuthorizedAt,
	)
	download.Attachment.Status = domain.AttachmentStatus(attachmentStatus)
	download.Status = downloadStatus
	return err
}

func scanAttachmentSale(row rowScanner, sale *domain.AttachmentSale) error {
	var attachmentStatus string
	err := row.Scan(
		&sale.Attachment.ID,
		&sale.Attachment.TopicID,
		&sale.Attachment.OwnerID,
		&sale.Attachment.ObjectKey,
		&sale.Attachment.OriginalName,
		&sale.Attachment.ContentType,
		&sale.Attachment.SizeBytes,
		&sale.Attachment.PriceCredits,
		&attachmentStatus,
		&sale.Attachment.CreatedAt,
		&sale.Attachment.UpdatedAt,
		&sale.Attachment.ArchivedAt,
		&sale.EarnedCredits,
		&sale.SoldAt,
	)
	sale.Attachment.Status = domain.AttachmentStatus(attachmentStatus)
	return err
}

func isUniqueViolation(err error) bool {
	var pgErr *pgconn.PgError
	return errors.As(err, &pgErr) && strings.TrimSpace(pgErr.Code) == "23505"
}

var schemaStatements = []string{
	`CREATE TABLE IF NOT EXISTS file_user_capacity_overrides (
		user_id BIGINT PRIMARY KEY,
		capacity_bytes BIGINT NOT NULL,
		updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
		CONSTRAINT file_user_capacity_overrides_identity_check CHECK (user_id > 0),
		CONSTRAINT file_user_capacity_overrides_capacity_check CHECK (capacity_bytes >= 0)
	)`,
	`CREATE TABLE IF NOT EXISTS file_folders (
		id BIGSERIAL PRIMARY KEY,
		owner_user_id BIGINT NOT NULL,
		name VARCHAR(200) NOT NULL,
		parent_id BIGINT REFERENCES file_folders(id) ON DELETE RESTRICT,
		created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
		updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
		CONSTRAINT file_folders_snapshot_check CHECK (
			owner_user_id > 0 AND BTRIM(name) <> ''
		)
	)`,
	`CREATE INDEX IF NOT EXISTS idx_file_folders_owner_parent_created
		ON file_folders(owner_user_id, parent_id, created_at DESC, id DESC)`,
	`CREATE TABLE IF NOT EXISTS files (
		id BIGSERIAL PRIMARY KEY,
		owner_user_id BIGINT NOT NULL,
		biz_type VARCHAR(64) NOT NULL,
		object_key VARCHAR(512) NOT NULL,
		original_name VARCHAR(255) NOT NULL,
		content_type VARCHAR(255) NOT NULL,
		size_bytes BIGINT NOT NULL,
		status VARCHAR(16) NOT NULL DEFAULT 'ACTIVE',
		created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
		updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
		deleted_at TIMESTAMPTZ,
		folder_id BIGINT REFERENCES file_folders(id) ON DELETE RESTRICT,
		is_sensitive BOOLEAN NOT NULL DEFAULT FALSE,
		comment VARCHAR(512) NOT NULL DEFAULT '',
		CONSTRAINT files_object_key_unique UNIQUE(object_key),
		CONSTRAINT files_snapshot_check CHECK (
			owner_user_id >= 0 AND BTRIM(biz_type) <> '' AND BTRIM(object_key) <> ''
			AND BTRIM(original_name) <> '' AND BTRIM(content_type) <> '' AND size_bytes >= 0
			AND (
				(status IN ('ACTIVE', 'DELETING') AND owner_user_id > 0 AND deleted_at IS NULL)
				OR (status = 'DELETED' AND deleted_at IS NOT NULL)
				OR (status = 'ERASED' AND owner_user_id = 0)
			)
		)
	)`,
	`ALTER TABLE files ADD COLUMN IF NOT EXISTS folder_id BIGINT REFERENCES file_folders(id) ON DELETE RESTRICT`,
	`ALTER TABLE files ADD COLUMN IF NOT EXISTS is_sensitive BOOLEAN NOT NULL DEFAULT FALSE`,
	`ALTER TABLE files ADD COLUMN IF NOT EXISTS comment VARCHAR(512) NOT NULL DEFAULT ''`,
	`CREATE INDEX IF NOT EXISTS idx_files_owner_created
		ON files(owner_user_id, created_at DESC, id DESC)`,
	`CREATE INDEX IF NOT EXISTS idx_files_owner_folder_created
		ON files(owner_user_id, folder_id, created_at DESC, id DESC)`,
	`CREATE INDEX IF NOT EXISTS idx_files_owner_deleted
		ON files(owner_user_id, deleted_at) WHERE deleted_at IS NOT NULL`,
	`CREATE INDEX IF NOT EXISTS idx_files_chart_created ON files(created_at)`,
	`CREATE INDEX IF NOT EXISTS idx_files_chart_deleted
		ON files(deleted_at) WHERE deleted_at IS NOT NULL`,
	`CREATE TABLE IF NOT EXISTS attachments (
		id BIGSERIAL PRIMARY KEY,
		topic_id BIGINT NOT NULL,
		owner_id BIGINT NOT NULL,
		object_key VARCHAR(512) NOT NULL,
		original_name VARCHAR(255) NOT NULL,
		content_type VARCHAR(255) NOT NULL,
		size_bytes BIGINT NOT NULL,
		price_credits BIGINT NOT NULL DEFAULT 0,
		status VARCHAR(16) NOT NULL DEFAULT 'ACTIVE',
		created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
		updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
		archived_at TIMESTAMPTZ,
		CONSTRAINT attachments_object_key_unique UNIQUE(object_key),
		CONSTRAINT attachments_snapshot_check CHECK (
			topic_id > 0 AND owner_id > 0 AND BTRIM(object_key) <> '' AND BTRIM(original_name) <> ''
			AND BTRIM(content_type) <> '' AND size_bytes >= 0 AND price_credits >= 0
			AND (
				(status = 'ACTIVE' AND archived_at IS NULL)
				OR (status = 'ARCHIVED' AND archived_at IS NOT NULL)
			)
		)
	)`,
	`CREATE INDEX IF NOT EXISTS idx_attachments_topic_active
		ON attachments(topic_id, status, created_at ASC, id ASC)`,
	`CREATE INDEX IF NOT EXISTS idx_attachments_owner_created
		ON attachments(owner_id, created_at DESC, id DESC)`,
	`CREATE INDEX IF NOT EXISTS idx_attachments_owner_archived
		ON attachments(owner_id, archived_at) WHERE archived_at IS NOT NULL`,
	`CREATE INDEX IF NOT EXISTS idx_attachments_chart_created ON attachments(created_at)`,
	`CREATE INDEX IF NOT EXISTS idx_attachments_chart_archived
		ON attachments(archived_at) WHERE archived_at IS NOT NULL`,
	`CREATE TABLE IF NOT EXISTS attachment_downloads (
		attachment_id BIGINT NOT NULL REFERENCES attachments(id),
		user_id BIGINT NOT NULL,
		status VARCHAR(16) NOT NULL,
		source_event_id VARCHAR(192) NOT NULL,
		charged_credits BIGINT NOT NULL DEFAULT 0,
		created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
		authorized_at TIMESTAMPTZ,
		PRIMARY KEY(attachment_id, user_id),
		CONSTRAINT attachment_downloads_source_event_unique UNIQUE(source_event_id),
		CONSTRAINT attachment_downloads_lifecycle_check CHECK (
			attachment_id > 0 AND user_id > 0 AND BTRIM(source_event_id) <> '' AND charged_credits >= 0
			AND (
				(status = 'PENDING' AND authorized_at IS NULL)
				OR (status = 'AUTHORIZED' AND authorized_at IS NOT NULL)
			)
		)
	)`,
	`CREATE INDEX IF NOT EXISTS idx_attachment_downloads_user_created
		ON attachment_downloads(user_id, created_at DESC, attachment_id DESC)`,
	`ALTER TABLE attachments DROP CONSTRAINT IF EXISTS attachments_snapshot_check`,
	`ALTER TABLE attachments ADD CONSTRAINT attachments_snapshot_check CHECK (
		topic_id > 0 AND owner_id >= 0 AND BTRIM(object_key) <> '' AND BTRIM(original_name) <> ''
		AND BTRIM(content_type) <> '' AND size_bytes >= 0 AND price_credits >= 0
		AND (
			(status = 'ACTIVE' AND archived_at IS NULL AND owner_id > 0)
			OR (status = 'ARCHIVED' AND archived_at IS NOT NULL)
		)
	)`,
	`CREATE TABLE IF NOT EXISTS file_erased_users (
		user_id BIGINT PRIMARY KEY,
		deletion_job_id BIGINT NOT NULL,
		policy_version INTEGER NOT NULL,
		archived_attachments BIGINT NOT NULL DEFAULT 0,
		deleted_downloads BIGINT NOT NULL DEFAULT 0,
		deleted_objects BIGINT NOT NULL DEFAULT 0,
		erased_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
		completed_at TIMESTAMPTZ,
		CONSTRAINT file_erased_users_identity_check CHECK (user_id > 0 AND deletion_job_id > 0 AND policy_version > 0),
		CONSTRAINT file_erased_users_counts_check CHECK (archived_attachments >= 0 AND deleted_downloads >= 0 AND deleted_objects >= 0)
	)`,
	`CREATE INDEX IF NOT EXISTS idx_file_erased_users_job ON file_erased_users(deletion_job_id)`,
	`CREATE TABLE IF NOT EXISTS file_erased_attachment_objects (
		user_id BIGINT NOT NULL REFERENCES file_erased_users(user_id) ON DELETE CASCADE,
		attachment_id BIGINT NOT NULL REFERENCES attachments(id),
		object_key VARCHAR(512) NOT NULL,
		deleted_at TIMESTAMPTZ,
		PRIMARY KEY(user_id, attachment_id),
		CONSTRAINT file_erased_attachment_objects_attachment_unique UNIQUE(attachment_id),
		CONSTRAINT file_erased_attachment_objects_key_check CHECK (BTRIM(object_key) <> '')
	)`,
	`CREATE INDEX IF NOT EXISTS idx_file_erased_attachment_objects_pending
		ON file_erased_attachment_objects(user_id, attachment_id) WHERE deleted_at IS NULL`,
	`CREATE TABLE IF NOT EXISTS file_erased_file_objects (
		user_id BIGINT NOT NULL REFERENCES file_erased_users(user_id) ON DELETE CASCADE,
		file_id BIGINT NOT NULL REFERENCES files(id),
		object_key VARCHAR(512) NOT NULL,
		deleted_at TIMESTAMPTZ,
		PRIMARY KEY(user_id, file_id),
		CONSTRAINT file_erased_file_objects_file_unique UNIQUE(file_id),
		CONSTRAINT file_erased_file_objects_key_check CHECK (BTRIM(object_key) <> '')
	)`,
	`CREATE INDEX IF NOT EXISTS idx_file_erased_file_objects_pending
		ON file_erased_file_objects(user_id, file_id) WHERE deleted_at IS NULL`,
}

var _ domain.Repository = (*PostgresRepository)(nil)
