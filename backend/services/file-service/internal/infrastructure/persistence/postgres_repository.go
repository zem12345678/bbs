package persistence

import (
	"context"
	"errors"
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

func (r *PostgresRepository) CreateAttachment(ctx context.Context, attachment domain.Attachment) (domain.Attachment, error) {
	err := scanAttachment(r.pool.QueryRow(ctx, `
INSERT INTO attachments(topic_id, owner_id, object_key, original_name, content_type, size_bytes, price_credits, status, created_at, updated_at)
VALUES($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
RETURNING id, topic_id, owner_id, object_key, original_name, content_type, size_bytes, price_credits, status, created_at, updated_at, archived_at
`, attachment.TopicID, attachment.OwnerID, attachment.ObjectKey, attachment.OriginalName, attachment.ContentType, attachment.SizeBytes, attachment.PriceCredits, attachment.Status, attachment.CreatedAt, attachment.UpdatedAt), &attachment)
	if isUniqueViolation(err) {
		return domain.Attachment{}, domain.ErrAttachmentObjectKeyTaken
	}
	return attachment, err
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

func (r *PostgresRepository) ListUserAttachmentDownloads(ctx context.Context, userID int64, limit, offset int32) ([]domain.AttachmentDownload, error) {
	rows, err := r.pool.Query(ctx, `
SELECT a.id, a.topic_id, a.owner_id, a.object_key, a.original_name, a.content_type, a.size_bytes, a.price_credits, a.status, a.created_at, a.updated_at, a.archived_at,
	       d.status, d.charged_credits, d.created_at, d.authorized_at
FROM attachment_downloads d
JOIN attachments a ON a.id = d.attachment_id
WHERE d.user_id = $1 AND d.status = $2
ORDER BY d.created_at DESC, d.attachment_id DESC
LIMIT $3 OFFSET $4
`, userID, domain.DownloadStatusAuthorized, limit, offset)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	downloads := make([]domain.AttachmentDownload, 0)
	for rows.Next() {
		var download domain.AttachmentDownload
		if err := scanAttachmentDownload(rows, &download); err != nil {
			return nil, err
		}
		downloads = append(downloads, download)
	}
	return downloads, rows.Err()
}

func (r *PostgresRepository) ListUserAttachmentSales(ctx context.Context, userID int64, limit, offset int32) ([]domain.AttachmentSale, error) {
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
		return nil, err
	}
	defer rows.Close()

	sales := make([]domain.AttachmentSale, 0)
	for rows.Next() {
		var sale domain.AttachmentSale
		if err := scanAttachmentSale(rows, &sale); err != nil {
			return nil, err
		}
		sales = append(sales, sale)
	}
	return sales, rows.Err()
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
	var attachment domain.Attachment
	err := scanAttachment(r.pool.QueryRow(ctx, `
UPDATE attachments
SET status = $3, archived_at = $4, updated_at = $4
WHERE id = $1 AND owner_id = $2 AND status = $5
RETURNING id, topic_id, owner_id, object_key, original_name, content_type, size_bytes, price_credits, status, created_at, updated_at, archived_at
`, attachmentID, ownerID, domain.AttachmentStatusArchived, archivedAt, domain.AttachmentStatusActive), &attachment)
	if !errors.Is(err, pgx.ErrNoRows) {
		return attachment, err
	}

	existing, getErr := r.GetAttachment(ctx, attachmentID)
	if getErr != nil {
		return domain.Attachment{}, getErr
	}
	if existing.OwnerID != ownerID {
		return domain.Attachment{}, domain.ErrAttachmentOwnerMismatch
	}
	return domain.Attachment{}, domain.ErrAttachmentArchived
}

func (r *PostgresRepository) UpdateAttachmentPrice(ctx context.Context, attachmentID, ownerID, priceCredits int64, updatedAt time.Time) (domain.Attachment, error) {
	var attachment domain.Attachment
	err := scanAttachment(r.pool.QueryRow(ctx, `
UPDATE attachments
SET price_credits = $3, updated_at = $4
WHERE id = $1 AND owner_id = $2 AND status = $5
RETURNING id, topic_id, owner_id, object_key, original_name, content_type, size_bytes, price_credits, status, created_at, updated_at, archived_at
`, attachmentID, ownerID, priceCredits, updatedAt, domain.AttachmentStatusActive), &attachment)
	if !errors.Is(err, pgx.ErrNoRows) {
		return attachment, err
	}

	existing, getErr := r.GetAttachment(ctx, attachmentID)
	if getErr != nil {
		return domain.Attachment{}, getErr
	}
	if existing.OwnerID != ownerID {
		return domain.Attachment{}, domain.ErrAttachmentOwnerMismatch
	}
	return domain.Attachment{}, domain.ErrAttachmentArchived
}

func (r *PostgresRepository) EnsureDownload(ctx context.Context, attachmentID, userID int64, sourceEventID string, chargedCredits int64, createdAt time.Time) (domain.Download, error) {
	var download domain.Download
	err := scanDownload(r.pool.QueryRow(ctx, `
INSERT INTO attachment_downloads(attachment_id, user_id, status, source_event_id, charged_credits, created_at)
VALUES($1, $2, $3, $4, $5, $6)
ON CONFLICT(attachment_id, user_id) DO UPDATE SET attachment_id = attachment_downloads.attachment_id
RETURNING attachment_id, user_id, status, source_event_id, charged_credits, created_at, authorized_at
`, attachmentID, userID, domain.DownloadStatusPending, sourceEventID, chargedCredits, createdAt), &download)
	if isUniqueViolation(err) {
		return domain.Download{}, domain.ErrDownloadRecordMismatch
	}
	return download, err
}

func (r *PostgresRepository) CompleteDownloadAuthorization(ctx context.Context, attachmentID, userID int64, authorizedAt time.Time, settle func(context.Context) error) (_ domain.Download, _ bool, err error) {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return domain.Download{}, false, err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	var attachment domain.Attachment
	err = scanAttachment(tx.QueryRow(ctx, `
SELECT id, topic_id, owner_id, object_key, original_name, content_type, size_bytes, price_credits, status, created_at, updated_at, archived_at
FROM attachments
WHERE id = $1
FOR UPDATE
`, attachmentID), &attachment)
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.Download{}, false, domain.ErrAttachmentNotFound
	}
	if err != nil {
		return domain.Download{}, false, err
	}
	if attachment.Status != domain.AttachmentStatusActive {
		return domain.Download{}, false, domain.ErrAttachmentArchived
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

type rowScanner interface {
	Scan(...any) error
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
}

var _ domain.Repository = (*PostgresRepository)(nil)
