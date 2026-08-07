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

func TestFileOrganizationPostgresIntegration(t *testing.T) {
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
	otherOwnerID := seed + 1
	now := time.Now().UTC().Truncate(time.Microsecond)
	objectPrefix := fmt.Sprintf("integration-folders/%d", seed)
	t.Cleanup(func() {
		cleanupCtx, cleanupCancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cleanupCancel()
		_, _ = pool.Exec(cleanupCtx, `DELETE FROM files WHERE owner_user_id = ANY($1::BIGINT[])`, []int64{ownerID, otherOwnerID})
		_, _ = pool.Exec(cleanupCtx, `UPDATE file_folders SET parent_id = NULL WHERE owner_user_id = ANY($1::BIGINT[])`, []int64{ownerID, otherOwnerID})
		_, _ = pool.Exec(cleanupCtx, `DELETE FROM file_folders WHERE owner_user_id = ANY($1::BIGINT[])`, []int64{ownerID, otherOwnerID})
	})

	alpha, err := repo.CreateFolder(ctx, domain.Folder{OwnerID: ownerID, Name: "Alpha", CreatedAt: now, UpdatedAt: now})
	if err != nil {
		t.Fatalf("create alpha: %v", err)
	}
	beta, err := repo.CreateFolder(ctx, domain.Folder{OwnerID: ownerID, Name: "Beta", CreatedAt: now.Add(time.Second), UpdatedAt: now.Add(time.Second)})
	if err != nil {
		t.Fatalf("create beta: %v", err)
	}
	child, err := repo.CreateFolder(ctx, domain.Folder{OwnerID: ownerID, Name: "Child", ParentID: alpha.ID, CreatedAt: now.Add(2 * time.Second), UpdatedAt: now.Add(2 * time.Second)})
	if err != nil {
		t.Fatalf("create child: %v", err)
	}
	otherFolder, err := repo.CreateFolder(ctx, domain.Folder{OwnerID: otherOwnerID, Name: "Other", CreatedAt: now, UpdatedAt: now})
	if err != nil {
		t.Fatalf("create other folder: %v", err)
	}
	if _, err := repo.CreateFolder(ctx, domain.Folder{OwnerID: ownerID, Name: "Foreign child", ParentID: otherFolder.ID, CreatedAt: now, UpdatedAt: now}); !errors.Is(err, domain.ErrFolderNotFound) {
		t.Fatalf("create under foreign folder error = %v, want ErrFolderNotFound", err)
	}

	folderFile, err := repo.CreateFile(ctx, domain.File{
		OwnerID: ownerID, BizType: "drive", ObjectKey: objectPrefix + "/folder.txt", OriginalName: "folder.txt",
		ContentType: "text/plain", SizeBytes: 10, Status: domain.FileStatusActive, CreatedAt: now, UpdatedAt: now, FolderID: alpha.ID,
	}, 1<<30)
	if err != nil {
		t.Fatalf("create folder file: %v", err)
	}
	rootFile, err := repo.CreateFile(ctx, domain.File{
		OwnerID: ownerID, BizType: "drive", ObjectKey: objectPrefix + "/root.txt", OriginalName: "root.txt",
		ContentType: "text/plain", SizeBytes: 20, Status: domain.FileStatusActive, CreatedAt: now.Add(time.Second), UpdatedAt: now.Add(time.Second),
	}, 1<<30)
	if err != nil {
		t.Fatalf("create root file: %v", err)
	}
	if _, err := repo.CreateFile(ctx, domain.File{
		OwnerID: ownerID, BizType: "drive", ObjectKey: objectPrefix + "/foreign.txt", OriginalName: "foreign.txt",
		ContentType: "text/plain", SizeBytes: 1, Status: domain.FileStatusActive, CreatedAt: now, UpdatedAt: now, FolderID: otherFolder.ID,
	}, 1<<30); !errors.Is(err, domain.ErrFolderNotFound) {
		t.Fatalf("upload to foreign folder error = %v, want ErrFolderNotFound", err)
	}

	rootFolders, total, err := repo.ListFolders(ctx, domain.FolderListQuery{OwnerID: ownerID, Limit: 20})
	if err != nil || total != 2 || len(rootFolders) != 2 {
		t.Fatalf("list root folders = %+v, total = %d, err = %v", rootFolders, total, err)
	}
	var listedAlpha domain.Folder
	for _, folder := range rootFolders {
		if folder.ID == alpha.ID {
			listedAlpha = folder
		}
	}
	if listedAlpha.FoldersCount != 1 || listedAlpha.FilesCount != 1 {
		t.Fatalf("alpha counts = folders %d files %d, want 1/1", listedAlpha.FoldersCount, listedAlpha.FilesCount)
	}
	searched, total, err := repo.ListFolders(ctx, domain.FolderListQuery{OwnerID: ownerID, Limit: 20, SearchQuery: "lPh"})
	if err != nil || total != 1 || len(searched) != 1 || searched[0].ID != alpha.ID {
		t.Fatalf("folder search = %+v, total = %d, err = %v", searched, total, err)
	}
	page, total, err := repo.ListFolders(ctx, domain.FolderListQuery{OwnerID: ownerID, Limit: 1, Offset: 1})
	if err != nil || total != 2 || len(page) != 1 || page[0].ID != alpha.ID {
		t.Fatalf("folder page = %+v, total = %d, err = %v", page, total, err)
	}
	nested, total, err := repo.ListFolders(ctx, domain.FolderListQuery{OwnerID: ownerID, ParentID: alpha.ID, Limit: 20})
	if err != nil || total != 1 || len(nested) != 1 || nested[0].ID != child.ID {
		t.Fatalf("nested folders = %+v, total = %d, err = %v", nested, total, err)
	}
	if _, _, err := repo.ListFolders(ctx, domain.FolderListQuery{OwnerID: otherOwnerID, ParentID: alpha.ID, Limit: 20}); !errors.Is(err, domain.ErrFolderNotFound) {
		t.Fatalf("cross-owner folder list error = %v, want ErrFolderNotFound", err)
	}

	allFiles, total, err := repo.ListUserFiles(ctx, ownerID, 20, 0)
	if err != nil || total != 2 || len(allFiles) != 2 {
		t.Fatalf("all files = %+v, total = %d, err = %v", allFiles, total, err)
	}
	rootFiles, total, err := repo.ListUserFilesByFolder(ctx, ownerID, 0, 20, 0)
	if err != nil || total != 1 || len(rootFiles) != 1 || rootFiles[0].ID != rootFile.ID {
		t.Fatalf("root files = %+v, total = %d, err = %v", rootFiles, total, err)
	}
	alphaFiles, total, err := repo.ListUserFilesByFolder(ctx, ownerID, alpha.ID, 20, 0)
	if err != nil || total != 1 || len(alphaFiles) != 1 || alphaFiles[0].ID != folderFile.ID {
		t.Fatalf("alpha files = %+v, total = %d, err = %v", alphaFiles, total, err)
	}
	if _, _, err := repo.ListUserFilesByFolder(ctx, otherOwnerID, alpha.ID, 20, 0); !errors.Is(err, domain.ErrFolderNotFound) {
		t.Fatalf("cross-owner file list error = %v, want ErrFolderNotFound", err)
	}

	name := "renamed.txt"
	comment := "sensitive material"
	sensitive := true
	childID := child.ID
	updatedFile, err := repo.UpdateFile(ctx, ownerID, folderFile.ID, domain.FileUpdate{
		Name: &name, FolderID: &childID, IsSensitive: &sensitive, Comment: &comment,
	}, now.Add(3*time.Second))
	if err != nil {
		t.Fatalf("update file: %v", err)
	}
	if updatedFile.OriginalName != name || updatedFile.FolderID != child.ID || !updatedFile.IsSensitive || updatedFile.Comment != comment {
		t.Fatalf("updated file = %+v", updatedFile)
	}
	if _, err := repo.UpdateFile(ctx, otherOwnerID, folderFile.ID, domain.FileUpdate{Name: &name}, now); !errors.Is(err, domain.ErrFileOwnerMismatch) {
		t.Fatalf("cross-owner file update error = %v, want ErrFileOwnerMismatch", err)
	}
	missingFolderID := seed + 999999
	if _, err := repo.UpdateFile(ctx, ownerID, folderFile.ID, domain.FileUpdate{FolderID: &missingFolderID}, now); !errors.Is(err, domain.ErrFolderNotFound) {
		t.Fatalf("move to missing folder error = %v, want ErrFolderNotFound", err)
	}

	alphaID := alpha.ID
	renamedBeta := "Moved Beta"
	movedBeta, err := repo.UpdateFolder(ctx, ownerID, beta.ID, domain.FolderUpdate{Name: &renamedBeta, ParentID: &alphaID}, now.Add(3*time.Second))
	if err != nil {
		t.Fatalf("move beta under alpha: %v", err)
	}
	if movedBeta.Name != renamedBeta || movedBeta.ParentID != alpha.ID {
		t.Fatalf("moved beta = %+v", movedBeta)
	}
	selfID := alpha.ID
	if _, err := repo.UpdateFolder(ctx, ownerID, alpha.ID, domain.FolderUpdate{ParentID: &selfID}, now); !errors.Is(err, domain.ErrFolderRecursive) {
		t.Fatalf("self-parent error = %v, want ErrFolderRecursive", err)
	}
	if _, err := repo.UpdateFolder(ctx, ownerID, alpha.ID, domain.FolderUpdate{ParentID: &childID}, now); !errors.Is(err, domain.ErrFolderRecursive) {
		t.Fatalf("descendant-parent error = %v, want ErrFolderRecursive", err)
	}
	if _, err := repo.UpdateFolder(ctx, ownerID, alpha.ID, domain.FolderUpdate{ParentID: &beta.ID}, now); !errors.Is(err, domain.ErrFolderRecursive) {
		t.Fatalf("moved descendant-parent error = %v, want ErrFolderRecursive", err)
	}
	rootParentID := int64(0)
	if _, err := repo.UpdateFolder(ctx, ownerID, beta.ID, domain.FolderUpdate{ParentID: &rootParentID}, now.Add(4*time.Second)); err != nil {
		t.Fatalf("move beta back to root: %v", err)
	}
	if _, err := repo.UpdateFolder(ctx, otherOwnerID, alpha.ID, domain.FolderUpdate{}, now); !errors.Is(err, domain.ErrFolderNotFound) {
		t.Fatalf("cross-owner folder update error = %v, want ErrFolderNotFound", err)
	}
	if _, err := repo.DeleteFolder(ctx, otherOwnerID, alpha.ID); !errors.Is(err, domain.ErrFolderNotFound) {
		t.Fatalf("cross-owner folder delete error = %v, want ErrFolderNotFound", err)
	}
	if _, err := repo.DeleteFolder(ctx, ownerID, alpha.ID); !errors.Is(err, domain.ErrFolderNotEmpty) {
		t.Fatalf("delete parent folder error = %v, want ErrFolderNotEmpty", err)
	}
	if _, err := repo.DeleteFolder(ctx, ownerID, child.ID); !errors.Is(err, domain.ErrFolderNotEmpty) {
		t.Fatalf("delete folder with file error = %v, want ErrFolderNotEmpty", err)
	}
	deletedFileFolder, err := repo.CreateFolder(ctx, domain.Folder{OwnerID: ownerID, Name: "Deleted file", CreatedAt: now, UpdatedAt: now})
	if err != nil {
		t.Fatalf("create deleted-file folder: %v", err)
	}
	fileToDelete, err := repo.CreateFile(ctx, domain.File{
		OwnerID: ownerID, BizType: "drive", ObjectKey: objectPrefix + "/deleted.txt", OriginalName: "deleted.txt",
		ContentType: "text/plain", SizeBytes: 1, Status: domain.FileStatusActive, CreatedAt: now, UpdatedAt: now, FolderID: deletedFileFolder.ID,
	}, 1<<30)
	if err != nil {
		t.Fatalf("create file to delete: %v", err)
	}
	if _, err := repo.BeginFileDeletion(ctx, ownerID, fileToDelete.ID, now.Add(4*time.Second)); err != nil {
		t.Fatalf("begin deleting folder file: %v", err)
	}
	deletedFile, err := repo.CompleteFileDeletion(ctx, ownerID, fileToDelete.ID, now.Add(5*time.Second))
	if err != nil {
		t.Fatalf("complete deleting folder file: %v", err)
	}
	if deletedFile.FolderID != 0 {
		t.Fatalf("deleted file folder = %d, want root reference cleared", deletedFile.FolderID)
	}
	if _, err := repo.DeleteFolder(ctx, ownerID, deletedFileFolder.ID); err != nil {
		t.Fatalf("delete folder after file deletion: %v", err)
	}
	rootID := int64(0)
	if _, err := repo.UpdateFile(ctx, ownerID, folderFile.ID, domain.FileUpdate{FolderID: &rootID}, now.Add(4*time.Second)); err != nil {
		t.Fatalf("move file to root: %v", err)
	}
	for _, folder := range []domain.Folder{child, alpha, beta} {
		if _, err := repo.DeleteFolder(ctx, ownerID, folder.ID); err != nil {
			t.Fatalf("delete empty folder %d: %v", folder.ID, err)
		}
	}
}

func TestEnsureSchemaUpgradesLegacyFilesTable(t *testing.T) {
	dsn := strings.TrimSpace(os.Getenv("BBS_FILE_POSTGRES_TEST_DSN"))
	if dsn == "" {
		t.Skip("BBS_FILE_POSTGRES_TEST_DSN is not set")
	}
	config, err := pgxpool.ParseConfig(dsn)
	if err != nil {
		t.Fatalf("parse postgres config: %v", err)
	}
	config.MaxConns = 1
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	t.Cleanup(cancel)
	pool, err := pgxpool.NewWithConfig(ctx, config)
	if err != nil {
		t.Fatalf("open postgres pool: %v", err)
	}
	t.Cleanup(pool.Close)
	if _, err := pool.Exec(ctx, `CREATE TEMP TABLE files (
		id BIGSERIAL PRIMARY KEY,
		owner_user_id BIGINT NOT NULL,
		biz_type VARCHAR(64) NOT NULL,
		object_key VARCHAR(512) NOT NULL UNIQUE,
		original_name VARCHAR(255) NOT NULL,
		content_type VARCHAR(255) NOT NULL,
		size_bytes BIGINT NOT NULL,
		status VARCHAR(16) NOT NULL DEFAULT 'ACTIVE',
		created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
		updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
		deleted_at TIMESTAMPTZ
	)`); err != nil {
		t.Fatalf("create legacy files table: %v", err)
	}
	if _, err := pool.Exec(ctx, `SET search_path TO pg_temp`); err != nil {
		t.Fatalf("select temporary schema: %v", err)
	}
	if _, err := pool.Exec(ctx, `
INSERT INTO files(owner_user_id, biz_type, object_key, original_name, content_type, size_bytes, status)
VALUES(1, 'drive', 'legacy/object', 'legacy.txt', 'text/plain', 1, 'ACTIVE')
`); err != nil {
		t.Fatalf("insert legacy file: %v", err)
	}
	if err := NewPostgresRepository(pool).EnsureSchema(ctx); err != nil {
		t.Fatalf("upgrade legacy schema: %v", err)
	}
	var folderID *int64
	var isSensitive bool
	var comment string
	if err := pool.QueryRow(ctx, `SELECT folder_id, is_sensitive, comment FROM files WHERE object_key = 'legacy/object'`).Scan(&folderID, &isSensitive, &comment); err != nil {
		t.Fatalf("read upgraded legacy file: %v", err)
	}
	if folderID != nil || isSensitive || comment != "" {
		t.Fatalf("upgraded defaults = folder %v sensitive %v comment %q", folderID, isSensitive, comment)
	}
}
