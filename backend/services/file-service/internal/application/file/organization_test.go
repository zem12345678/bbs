package file

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	domain "file-service/internal/domain/file"
)

func TestFileOrganizationServiceRoutesFolderOperations(t *testing.T) {
	repo := &organizationRepositoryStub{memoryRepository: newMemoryRepository(domain.Attachment{})}
	service := NewService(repo, nil, nil, nil)
	now := time.Date(2026, 8, 7, 1, 2, 3, 0, time.UTC)
	service.now = func() time.Time { return now }

	created, err := service.CreateFolder(t.Context(), CreateFolderCommand{OwnerID: 9, Name: "  Projects  ", ParentID: 0})
	if err != nil {
		t.Fatalf("CreateFolder() error = %v", err)
	}
	if created.Name != "Projects" || created.OwnerID != 9 || !created.CreatedAt.Equal(now) {
		t.Fatalf("CreateFolder() = %+v", created)
	}

	parentID := int64(12)
	name := " Renamed "
	updatedFolder, err := service.UpdateFolder(t.Context(), 9, created.ID, domain.FolderUpdate{Name: &name, ParentID: &parentID})
	if err != nil {
		t.Fatalf("UpdateFolder() error = %v", err)
	}
	if updatedFolder.Name != "Renamed" || updatedFolder.ParentID != parentID {
		t.Fatalf("UpdateFolder() = %+v", updatedFolder)
	}

	file, err := service.CreateFile(t.Context(), CreateFileCommand{
		OwnerID: 9, BizType: "drive", ObjectKey: "objects/1", OriginalName: "one.txt", SizeBytes: 1, FolderID: created.ID,
	})
	if err != nil {
		t.Fatalf("CreateFile() error = %v", err)
	}
	if file.FolderID != created.ID {
		t.Fatalf("CreateFile() folder = %d, want %d", file.FolderID, created.ID)
	}

	rootID := int64(0)
	comment := "private note"
	sensitive := true
	fileName := " two.txt "
	updatedFile, err := service.UpdateFile(t.Context(), 9, file.ID, domain.FileUpdate{
		Name: &fileName, FolderID: &rootID, IsSensitive: &sensitive, Comment: &comment,
	})
	if err != nil {
		t.Fatalf("UpdateFile() error = %v", err)
	}
	if updatedFile.OriginalName != "two.txt" || updatedFile.FolderID != 0 || !updatedFile.IsSensitive || updatedFile.Comment != comment {
		t.Fatalf("UpdateFile() = %+v", updatedFile)
	}

	if _, _, err := service.ListFilesInFolder(t.Context(), 9, 20, 0, nil); err != nil {
		t.Fatalf("ListFilesInFolder(omitted) error = %v", err)
	}
	if repo.folderListCalls != 0 {
		t.Fatalf("omitted folder filter called folder repository %d times", repo.folderListCalls)
	}
	if _, _, err := service.ListFilesInFolder(t.Context(), 9, 20, 0, &rootID); err != nil {
		t.Fatalf("ListFilesInFolder(root) error = %v", err)
	}
	if repo.folderListCalls != 1 || repo.listedFolderID != 0 {
		t.Fatalf("root folder calls = %d, folder = %d", repo.folderListCalls, repo.listedFolderID)
	}
}

func TestFileOrganizationServiceRejectsInvalidInputs(t *testing.T) {
	repo := &organizationRepositoryStub{memoryRepository: newMemoryRepository(domain.Attachment{})}
	service := NewService(repo, nil, nil, nil)
	tooLongFolder := strings.Repeat("a", maxFolderNameLength+1)
	tooLongFile := strings.Repeat("a", maxOriginalNameLength+1)
	tooLongComment := strings.Repeat("a", maxFileCommentLength+1)
	negative := int64(-1)

	for _, test := range []struct {
		name string
		run  func() error
		want error
	}{
		{name: "empty folder", run: func() error {
			_, err := service.CreateFolder(t.Context(), CreateFolderCommand{OwnerID: 1, Name: " "})
			return err
		}, want: domain.ErrInvalidFolder},
		{name: "folder separator", run: func() error {
			_, err := service.CreateFolder(t.Context(), CreateFolderCommand{OwnerID: 1, Name: "a/b"})
			return err
		}, want: domain.ErrInvalidFolder},
		{name: "long folder", run: func() error {
			_, err := service.CreateFolder(t.Context(), CreateFolderCommand{OwnerID: 1, Name: tooLongFolder})
			return err
		}, want: domain.ErrInvalidFolder},
		{name: "negative parent", run: func() error {
			_, err := service.CreateFolder(t.Context(), CreateFolderCommand{OwnerID: 1, Name: "a", ParentID: -1})
			return err
		}, want: domain.ErrInvalidFolder},
		{name: "invalid list", run: func() error {
			_, _, err := service.ListFolders(t.Context(), domain.FolderListQuery{OwnerID: 1, Limit: 0})
			return err
		}, want: domain.ErrInvalidFolder},
		{name: "long file name", run: func() error {
			_, err := service.UpdateFile(t.Context(), 1, 1, domain.FileUpdate{Name: &tooLongFile})
			return err
		}, want: domain.ErrInvalidFile},
		{name: "long comment", run: func() error {
			_, err := service.UpdateFile(t.Context(), 1, 1, domain.FileUpdate{Comment: &tooLongComment})
			return err
		}, want: domain.ErrInvalidFile},
		{name: "negative file folder", run: func() error {
			_, err := service.UpdateFile(t.Context(), 1, 1, domain.FileUpdate{FolderID: &negative})
			return err
		}, want: domain.ErrInvalidFile},
		{name: "negative upload folder", run: func() error {
			_, err := service.CreateFile(t.Context(), CreateFileCommand{OwnerID: 1, BizType: "drive", ObjectKey: "key", OriginalName: "name", SizeBytes: 1, FolderID: -1})
			return err
		}, want: domain.ErrInvalidFile},
	} {
		t.Run(test.name, func(t *testing.T) {
			if err := test.run(); !errors.Is(err, test.want) {
				t.Fatalf("error = %v, want %v", err, test.want)
			}
		})
	}
}

type organizationRepositoryStub struct {
	*memoryRepository
	folders         []domain.Folder
	folderListCalls int
	listedFolderID  int64
}

func (r *organizationRepositoryStub) ListFolders(_ context.Context, query domain.FolderListQuery) ([]domain.Folder, int64, error) {
	return r.folders, int64(len(r.folders)), nil
}

func (r *organizationRepositoryStub) CreateFolder(_ context.Context, folder domain.Folder) (domain.Folder, error) {
	folder.ID = int64(len(r.folders) + 1)
	r.folders = append(r.folders, folder)
	return folder, nil
}

func (r *organizationRepositoryStub) UpdateFolder(_ context.Context, ownerID, folderID int64, update domain.FolderUpdate, updatedAt time.Time) (domain.Folder, error) {
	for index := range r.folders {
		if r.folders[index].ID == folderID && r.folders[index].OwnerID == ownerID {
			if update.Name != nil {
				r.folders[index].Name = *update.Name
			}
			if update.ParentID != nil {
				r.folders[index].ParentID = *update.ParentID
			}
			r.folders[index].UpdatedAt = updatedAt
			return r.folders[index], nil
		}
	}
	return domain.Folder{}, domain.ErrFolderNotFound
}

func (r *organizationRepositoryStub) DeleteFolder(_ context.Context, ownerID, folderID int64) (domain.Folder, error) {
	for index := range r.folders {
		if r.folders[index].ID == folderID && r.folders[index].OwnerID == ownerID {
			folder := r.folders[index]
			r.folders = append(r.folders[:index], r.folders[index+1:]...)
			return folder, nil
		}
	}
	return domain.Folder{}, domain.ErrFolderNotFound
}

func (r *organizationRepositoryStub) ListUserFilesByFolder(ctx context.Context, userID, folderID int64, limit, offset int32) ([]domain.File, int64, error) {
	r.folderListCalls++
	r.listedFolderID = folderID
	items, _, err := r.ListUserFiles(ctx, userID, limit, offset)
	filtered := make([]domain.File, 0)
	for _, item := range items {
		if item.FolderID == folderID {
			filtered = append(filtered, item)
		}
	}
	return filtered, int64(len(filtered)), err
}

func (r *organizationRepositoryStub) UpdateFile(_ context.Context, userID, fileID int64, update domain.FileUpdate, updatedAt time.Time) (domain.File, error) {
	for index := range r.files {
		if r.files[index].ID == fileID && r.files[index].OwnerID == userID {
			if update.Name != nil {
				r.files[index].OriginalName = *update.Name
			}
			if update.FolderID != nil {
				r.files[index].FolderID = *update.FolderID
			}
			if update.IsSensitive != nil {
				r.files[index].IsSensitive = *update.IsSensitive
			}
			if update.Comment != nil {
				r.files[index].Comment = *update.Comment
			}
			r.files[index].UpdatedAt = updatedAt
			return r.files[index], nil
		}
	}
	return domain.File{}, domain.ErrFileNotFound
}
