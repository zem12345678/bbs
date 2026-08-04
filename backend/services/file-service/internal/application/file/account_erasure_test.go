package file

import (
	"context"
	"errors"
	"testing"
	"time"

	domain "file-service/internal/domain/file"
)

func TestEraseUserDataRetriesOnlyPendingObjects(t *testing.T) {
	repository := &memoryErasureRepository{
		result:  domain.AccountErasureResult{ArchivedAttachments: 2, DeletedDownloads: 3},
		pending: map[int64]string{11: "attachments/11", 12: "attachments/12"},
	}
	deleteFailure := errors.New("storage unavailable")
	deleter := &recordingObjectDeleter{failures: map[string]error{"attachments/12": deleteFailure}}
	service := NewService(nil, nil, nil, nil, WithAccountErasure(repository, deleter))
	service.now = func() time.Time { return time.Unix(1_800_000_000, 0) }

	if _, err := service.EraseUserData(t.Context(), 42, 9001, 3); !errors.Is(err, deleteFailure) {
		t.Fatalf("first EraseUserData() error = %v, want %v", err, deleteFailure)
	}
	delete(deleter.failures, "attachments/12")
	result, err := service.EraseUserData(t.Context(), 42, 9001, 3)
	if err != nil {
		t.Fatalf("retry EraseUserData() error = %v", err)
	}
	if result != (domain.AccountErasureResult{ArchivedAttachments: 2, DeletedDownloads: 3, DeletedObjects: 2}) {
		t.Fatalf("result = %+v", result)
	}
	if got := deleter.calls["attachments/11"]; got != 1 {
		t.Fatalf("first object delete calls = %d, want 1", got)
	}
	if got := deleter.calls["attachments/12"]; got != 2 {
		t.Fatalf("retried object delete calls = %d, want 2", got)
	}
}

func TestEraseUserDataValidatesAndRequiresRepository(t *testing.T) {
	service := NewService(nil, nil, nil, nil)
	if _, err := service.EraseUserData(t.Context(), 0, 1, 1); !errors.Is(err, domain.ErrInvalidAccountErasure) {
		t.Fatalf("invalid request error = %v", err)
	}
	if _, err := service.EraseUserData(t.Context(), 1, 1, 1); !errors.Is(err, domain.ErrAccountErasureUnavailable) {
		t.Fatalf("missing repository error = %v", err)
	}
}

type memoryErasureRepository struct {
	result  domain.AccountErasureResult
	pending map[int64]string
}

func (r *memoryErasureRepository) BeginAccountErasure(context.Context, int64, int64, int32) (domain.AccountErasureResult, []domain.ErasureObject, error) {
	objects := make([]domain.ErasureObject, 0, len(r.pending))
	for attachmentID, objectKey := range r.pending {
		objects = append(objects, domain.ErasureObject{AttachmentID: attachmentID, ObjectKey: objectKey})
	}
	return r.result, objects, nil
}

func (r *memoryErasureRepository) CompleteAccountErasureObject(_ context.Context, _ int64, attachmentID int64, _ time.Time) error {
	if _, exists := r.pending[attachmentID]; !exists {
		return domain.ErrInvalidAccountErasure
	}
	delete(r.pending, attachmentID)
	r.result.DeletedObjects++
	return nil
}

func (r *memoryErasureRepository) CompleteAccountErasureFileObject(_ context.Context, _ int64, fileID int64, _ time.Time) error {
	if _, exists := r.pending[fileID]; !exists {
		return domain.ErrInvalidAccountErasure
	}
	delete(r.pending, fileID)
	r.result.DeletedObjects++
	return nil
}

func (r *memoryErasureRepository) CompleteAccountErasure(context.Context, int64, time.Time) (domain.AccountErasureResult, error) {
	if len(r.pending) != 0 {
		return domain.AccountErasureResult{}, domain.ErrAccountErasureUnavailable
	}
	return r.result, nil
}

type recordingObjectDeleter struct {
	calls    map[string]int
	failures map[string]error
}

func (d *recordingObjectDeleter) Delete(_ context.Context, objectKey string) error {
	if d.calls == nil {
		d.calls = make(map[string]int)
	}
	d.calls[objectKey]++
	return d.failures[objectKey]
}
