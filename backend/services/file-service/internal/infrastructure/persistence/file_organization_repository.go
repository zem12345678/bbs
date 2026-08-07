package persistence

import (
	"context"
	"errors"
	"time"

	domain "file-service/internal/domain/file"

	"github.com/jackc/pgx/v5"
)

func (r *PostgresRepository) ListFolders(ctx context.Context, query domain.FolderListQuery) ([]domain.Folder, int64, error) {
	if err := ensureOwnedFolder(ctx, r.pool, query.OwnerID, query.ParentID); err != nil {
		return nil, 0, err
	}
	var total int64
	if err := r.pool.QueryRow(ctx, `
SELECT COUNT(*)
FROM file_folders
WHERE owner_user_id = $1
  AND parent_id IS NOT DISTINCT FROM NULLIF($2, 0)
  AND ($3 = '' OR STRPOS(LOWER(name), LOWER($3)) > 0)
`, query.OwnerID, query.ParentID, query.SearchQuery).Scan(&total); err != nil {
		return nil, 0, err
	}
	rows, err := r.pool.Query(ctx, `
SELECT folder.id, folder.owner_user_id, folder.name, COALESCE(folder.parent_id, 0), folder.created_at, folder.updated_at,
       (SELECT COUNT(*) FROM file_folders child WHERE child.parent_id = folder.id),
       (SELECT COUNT(*) FROM files item WHERE item.folder_id = folder.id AND item.status IN ('ACTIVE', 'DELETING'))
FROM file_folders folder
WHERE folder.owner_user_id = $1
  AND folder.parent_id IS NOT DISTINCT FROM NULLIF($2, 0)
  AND ($3 = '' OR STRPOS(LOWER(folder.name), LOWER($3)) > 0)
ORDER BY folder.created_at DESC, folder.id DESC
LIMIT $4 OFFSET $5
`, query.OwnerID, query.ParentID, query.SearchQuery, query.Limit, query.Offset)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()
	items := make([]domain.Folder, 0)
	for rows.Next() {
		var item domain.Folder
		if err := scanFolder(rows, &item); err != nil {
			return nil, 0, err
		}
		items = append(items, item)
	}
	return items, total, rows.Err()
}

func (r *PostgresRepository) CreateFolder(ctx context.Context, folder domain.Folder) (domain.Folder, error) {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return domain.Folder{}, err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	if err := ensureFileUserActive(ctx, tx, folder.OwnerID); err != nil {
		return domain.Folder{}, err
	}
	if err := ensureOwnedFolder(ctx, tx, folder.OwnerID, folder.ParentID); err != nil {
		return domain.Folder{}, err
	}
	err = scanFolder(tx.QueryRow(ctx, `
INSERT INTO file_folders(owner_user_id, name, parent_id, created_at, updated_at)
VALUES($1, $2, NULLIF($3, 0), $4, $5)
RETURNING id, owner_user_id, name, COALESCE(parent_id, 0), created_at, updated_at, 0::BIGINT, 0::BIGINT
`, folder.OwnerID, folder.Name, folder.ParentID, folder.CreatedAt, folder.UpdatedAt), &folder)
	if err != nil {
		return domain.Folder{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return domain.Folder{}, err
	}
	return folder, nil
}

func (r *PostgresRepository) UpdateFolder(ctx context.Context, ownerID, folderID int64, update domain.FolderUpdate, updatedAt time.Time) (domain.Folder, error) {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return domain.Folder{}, err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	if err := ensureFileUserActive(ctx, tx, ownerID); err != nil {
		return domain.Folder{}, err
	}
	var existingID int64
	if err := tx.QueryRow(ctx, `SELECT id FROM file_folders WHERE id = $1 AND owner_user_id = $2 FOR UPDATE`, folderID, ownerID).Scan(&existingID); errors.Is(err, pgx.ErrNoRows) {
		return domain.Folder{}, domain.ErrFolderNotFound
	} else if err != nil {
		return domain.Folder{}, err
	}
	if update.ParentID != nil {
		if *update.ParentID == folderID {
			return domain.Folder{}, domain.ErrFolderRecursive
		}
		if err := ensureOwnedFolder(ctx, tx, ownerID, *update.ParentID); err != nil {
			return domain.Folder{}, err
		}
		if *update.ParentID > 0 {
			var recursive bool
			if err := tx.QueryRow(ctx, `
WITH RECURSIVE ancestors AS (
  SELECT id, parent_id FROM file_folders WHERE id = $1 AND owner_user_id = $2
  UNION ALL
  SELECT parent.id, parent.parent_id
  FROM file_folders parent
  JOIN ancestors child ON parent.id = child.parent_id
  WHERE parent.owner_user_id = $2
)
SELECT EXISTS(SELECT 1 FROM ancestors WHERE id = $3)
`, *update.ParentID, ownerID, folderID).Scan(&recursive); err != nil {
				return domain.Folder{}, err
			}
			if recursive {
				return domain.Folder{}, domain.ErrFolderRecursive
			}
		}
	}
	var name string
	if update.Name != nil {
		name = *update.Name
	}
	var parentID int64
	if update.ParentID != nil {
		parentID = *update.ParentID
	}
	var folder domain.Folder
	err = scanFolder(tx.QueryRow(ctx, `
UPDATE file_folders
SET name = CASE WHEN $3 THEN $4 ELSE name END,
    parent_id = CASE WHEN $5 THEN NULLIF($6, 0) ELSE parent_id END,
    updated_at = CASE WHEN $3 OR $5 THEN $7 ELSE updated_at END
WHERE id = $1 AND owner_user_id = $2
RETURNING id, owner_user_id, name, COALESCE(parent_id, 0), created_at, updated_at,
          (SELECT COUNT(*) FROM file_folders child WHERE child.parent_id = file_folders.id),
          (SELECT COUNT(*) FROM files item WHERE item.folder_id = file_folders.id AND item.status IN ('ACTIVE', 'DELETING'))
`, folderID, ownerID, update.Name != nil, name, update.ParentID != nil, parentID, updatedAt), &folder)
	if err != nil {
		return domain.Folder{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return domain.Folder{}, err
	}
	return folder, nil
}

func (r *PostgresRepository) DeleteFolder(ctx context.Context, ownerID, folderID int64) (domain.Folder, error) {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return domain.Folder{}, err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	if err := ensureFileUserActive(ctx, tx, ownerID); err != nil {
		return domain.Folder{}, err
	}
	var folder domain.Folder
	err = scanFolder(tx.QueryRow(ctx, `
SELECT folder.id, folder.owner_user_id, folder.name, COALESCE(folder.parent_id, 0), folder.created_at, folder.updated_at,
       (SELECT COUNT(*) FROM file_folders child WHERE child.parent_id = folder.id),
       (SELECT COUNT(*) FROM files item WHERE item.folder_id = folder.id AND item.status IN ('ACTIVE', 'DELETING'))
FROM file_folders folder
WHERE folder.id = $1 AND folder.owner_user_id = $2
FOR UPDATE
`, folderID, ownerID), &folder)
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.Folder{}, domain.ErrFolderNotFound
	}
	if err != nil {
		return domain.Folder{}, err
	}
	if folder.FoldersCount != 0 || folder.FilesCount != 0 {
		return domain.Folder{}, domain.ErrFolderNotEmpty
	}
	if _, err := tx.Exec(ctx, `DELETE FROM file_folders WHERE id = $1 AND owner_user_id = $2`, folderID, ownerID); err != nil {
		return domain.Folder{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return domain.Folder{}, err
	}
	return folder, nil
}

func (r *PostgresRepository) ListUserFilesByFolder(ctx context.Context, userID, folderID int64, limit, offset int32) ([]domain.File, int64, error) {
	if err := ensureOwnedFolder(ctx, r.pool, userID, folderID); err != nil {
		return nil, 0, err
	}
	var total int64
	if err := r.pool.QueryRow(ctx, `
SELECT COUNT(*) FROM files
WHERE owner_user_id = $1 AND folder_id IS NOT DISTINCT FROM NULLIF($2, 0) AND status IN ('ACTIVE', 'DELETING')
`, userID, folderID).Scan(&total); err != nil {
		return nil, 0, err
	}
	rows, err := r.pool.Query(ctx, `
SELECT id, owner_user_id, biz_type, object_key, original_name, content_type, size_bytes, status, created_at, updated_at, deleted_at,
       COALESCE(folder_id, 0), is_sensitive, comment
FROM files
WHERE owner_user_id = $1 AND folder_id IS NOT DISTINCT FROM NULLIF($2, 0) AND status IN ('ACTIVE', 'DELETING')
ORDER BY created_at DESC, id DESC
LIMIT $3 OFFSET $4
`, userID, folderID, limit, offset)
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

func (r *PostgresRepository) UpdateFile(ctx context.Context, userID, fileID int64, update domain.FileUpdate, updatedAt time.Time) (domain.File, error) {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return domain.File{}, err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	if err := ensureFileUserActive(ctx, tx, userID); err != nil {
		return domain.File{}, err
	}
	if update.FolderID != nil {
		if err := ensureOwnedFolder(ctx, tx, userID, *update.FolderID); err != nil {
			return domain.File{}, err
		}
	}
	var name, comment string
	var folderID int64
	var isSensitive bool
	if update.Name != nil {
		name = *update.Name
	}
	if update.FolderID != nil {
		folderID = *update.FolderID
	}
	if update.IsSensitive != nil {
		isSensitive = *update.IsSensitive
	}
	if update.Comment != nil {
		comment = *update.Comment
	}
	var item domain.File
	err = scanFile(tx.QueryRow(ctx, `
UPDATE files
SET original_name = CASE WHEN $3 THEN $4 ELSE original_name END,
    folder_id = CASE WHEN $5 THEN NULLIF($6, 0) ELSE folder_id END,
    is_sensitive = CASE WHEN $7 THEN $8 ELSE is_sensitive END,
    comment = CASE WHEN $9 THEN $10 ELSE comment END,
    updated_at = CASE WHEN $3 OR $5 OR $7 OR $9 THEN $11 ELSE updated_at END
WHERE id = $1 AND owner_user_id = $2 AND status IN ('ACTIVE', 'DELETING')
RETURNING id, owner_user_id, biz_type, object_key, original_name, content_type, size_bytes, status, created_at, updated_at, deleted_at,
          COALESCE(folder_id, 0), is_sensitive, comment
`, fileID, userID, update.Name != nil, name, update.FolderID != nil, folderID, update.IsSensitive != nil, isSensitive, update.Comment != nil, comment, updatedAt), &item)
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
		return domain.File{}, domain.ErrFileNotFound
	}
	if err != nil {
		return domain.File{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return domain.File{}, err
	}
	return item, nil
}

func ensureOwnedFolder(ctx context.Context, querier rowQuerier, ownerID, folderID int64) error {
	if folderID < 0 {
		return domain.ErrInvalidFolder
	}
	if folderID == 0 {
		return nil
	}
	var exists bool
	if err := querier.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM file_folders WHERE id = $1 AND owner_user_id = $2)`, folderID, ownerID).Scan(&exists); err != nil {
		return err
	}
	if !exists {
		return domain.ErrFolderNotFound
	}
	return nil
}

func scanFolder(row rowScanner, folder *domain.Folder) error {
	return row.Scan(
		&folder.ID,
		&folder.OwnerID,
		&folder.Name,
		&folder.ParentID,
		&folder.CreatedAt,
		&folder.UpdatedAt,
		&folder.FoldersCount,
		&folder.FilesCount,
	)
}

var _ domain.FileOrganizationRepository = (*PostgresRepository)(nil)
