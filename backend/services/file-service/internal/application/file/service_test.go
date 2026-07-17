package file

import (
	"context"
	"strconv"
	"testing"
	"time"

	domain "file-service/internal/domain/file"
)

func TestAuthorizeDownloadChargesOnlyOnce(t *testing.T) {
	repo := newMemoryRepository(activeAttachment(101, 9, 7))
	charger := &captureCharger{}
	service := NewService(repo, charger)
	service.now = func() time.Time { return time.Date(2026, 7, 18, 9, 0, 0, 0, time.UTC) }

	first, err := service.AuthorizeDownload(context.Background(), 101, 42)
	if err != nil {
		t.Fatalf("AuthorizeDownload() error = %v", err)
	}
	if first.AlreadyAuthorized || first.ChargedCredits != 7 {
		t.Fatalf("first authorization = %+v, want a new 7-credit authorization", first)
	}
	if len(charger.commands) != 1 {
		t.Fatalf("credit debits = %d, want 1", len(charger.commands))
	}
	if command := charger.commands[0]; command.SourceEventID != "attachment-download:101:42" || command.Amount != 7 || command.UserID != 42 {
		t.Fatalf("unexpected debit command: %+v", command)
	}

	second, err := service.AuthorizeDownload(context.Background(), 101, 42)
	if err != nil {
		t.Fatalf("second AuthorizeDownload() error = %v", err)
	}
	if !second.AlreadyAuthorized || second.ChargedCredits != 0 {
		t.Fatalf("second authorization = %+v, want an already-authorized free retry", second)
	}
	if len(charger.commands) != 1 {
		t.Fatalf("credit debits after retry = %d, want 1", len(charger.commands))
	}
}

func TestAuthorizeDownloadKeepsPendingRecordForStableRetry(t *testing.T) {
	repo := newMemoryRepository(activeAttachment(102, 9, 11))
	charger := &captureCharger{errors: []error{domain.ErrInsufficientCredits}}
	service := NewService(repo, charger)

	_, err := service.AuthorizeDownload(context.Background(), 102, 42)
	if err != domain.ErrInsufficientCredits {
		t.Fatalf("first AuthorizeDownload() error = %v, want insufficient credits", err)
	}
	pending := repo.downloads[downloadKey(102, 42)]
	if pending.Status != domain.DownloadStatusPending || pending.SourceEventID != "attachment-download:102:42" {
		t.Fatalf("pending download = %+v", pending)
	}

	authorization, err := service.AuthorizeDownload(context.Background(), 102, 42)
	if err != nil {
		t.Fatalf("retry AuthorizeDownload() error = %v", err)
	}
	if authorization.AlreadyAuthorized || authorization.ChargedCredits != 11 {
		t.Fatalf("retry authorization = %+v", authorization)
	}
	if len(charger.commands) != 2 {
		t.Fatalf("credit debits = %d, want 2 attempts", len(charger.commands))
	}
	if charger.commands[0].SourceEventID != charger.commands[1].SourceEventID {
		t.Fatalf("retry event ids differ: %q and %q", charger.commands[0].SourceEventID, charger.commands[1].SourceEventID)
	}
}

func TestAuthorizeDownloadAllowsOwnerWithoutDebit(t *testing.T) {
	repo := newMemoryRepository(activeAttachment(103, 42, 13))
	charger := &captureCharger{}
	service := NewService(repo, charger)

	authorization, err := service.AuthorizeDownload(context.Background(), 103, 42)
	if err != nil {
		t.Fatalf("AuthorizeDownload() error = %v", err)
	}
	if authorization.AlreadyAuthorized || authorization.ChargedCredits != 0 {
		t.Fatalf("owner authorization = %+v", authorization)
	}
	if len(charger.commands) != 0 {
		t.Fatalf("owner debit count = %d, want 0", len(charger.commands))
	}
}

func TestCreateAttachmentRejectsUnsafeOriginalName(t *testing.T) {
	service := NewService(newMemoryRepository(domain.Attachment{}), &captureCharger{})
	_, err := service.CreateAttachment(context.Background(), CreateAttachmentCommand{
		TopicID:      1,
		OwnerID:      2,
		ObjectKey:    "topics/1/report.pdf",
		OriginalName: "../report.pdf",
		ContentType:  "application/pdf",
		SizeBytes:    1,
	})
	if err != domain.ErrInvalidAttachment {
		t.Fatalf("CreateAttachment() error = %v, want invalid attachment", err)
	}
}

func TestGetAttachmentRejectsArchivedAttachment(t *testing.T) {
	attachment := activeAttachment(104, 7, 0)
	attachment.Status = domain.AttachmentStatusArchived
	service := NewService(newMemoryRepository(attachment), &captureCharger{})

	_, err := service.GetAttachment(context.Background(), attachment.ID)
	if err != domain.ErrAttachmentArchived {
		t.Fatalf("GetAttachment() error = %v, want archived attachment", err)
	}
}

type captureCharger struct {
	commands []CreditDebitCommand
	errors   []error
}

func (c *captureCharger) DebitCredits(_ context.Context, command CreditDebitCommand) error {
	c.commands = append(c.commands, command)
	if len(c.errors) == 0 {
		return nil
	}
	err := c.errors[0]
	c.errors = c.errors[1:]
	return err
}

type memoryRepository struct {
	attachment domain.Attachment
	downloads  map[string]domain.Download
}

func newMemoryRepository(attachment domain.Attachment) *memoryRepository {
	return &memoryRepository{attachment: attachment, downloads: make(map[string]domain.Download)}
}

func (r *memoryRepository) EnsureSchema(context.Context) error { return nil }

func (r *memoryRepository) CreateAttachment(_ context.Context, attachment domain.Attachment) (domain.Attachment, error) {
	r.attachment = attachment
	return attachment, nil
}

func (r *memoryRepository) ListTopicAttachments(_ context.Context, topicID int64) ([]domain.Attachment, error) {
	if r.attachment.TopicID != topicID || r.attachment.Status != domain.AttachmentStatusActive {
		return nil, nil
	}
	return []domain.Attachment{r.attachment}, nil
}

func (r *memoryRepository) GetAttachment(context.Context, int64) (domain.Attachment, error) {
	if r.attachment.ID == 0 {
		return domain.Attachment{}, domain.ErrAttachmentNotFound
	}
	return r.attachment, nil
}

func (r *memoryRepository) ArchiveAttachment(_ context.Context, attachmentID, ownerID int64, archivedAt time.Time) (domain.Attachment, error) {
	if r.attachment.ID != attachmentID {
		return domain.Attachment{}, domain.ErrAttachmentNotFound
	}
	if r.attachment.OwnerID != ownerID {
		return domain.Attachment{}, domain.ErrAttachmentOwnerMismatch
	}
	r.attachment.Status = domain.AttachmentStatusArchived
	r.attachment.ArchivedAt = &archivedAt
	return r.attachment, nil
}

func (r *memoryRepository) EnsureDownload(_ context.Context, attachmentID, userID int64, sourceEventID string, chargedCredits int64, createdAt time.Time) (domain.Download, error) {
	key := downloadKey(attachmentID, userID)
	if existing, ok := r.downloads[key]; ok {
		return existing, nil
	}
	download := domain.Download{
		AttachmentID:   attachmentID,
		UserID:         userID,
		Status:         domain.DownloadStatusPending,
		SourceEventID:  sourceEventID,
		ChargedCredits: chargedCredits,
		CreatedAt:      createdAt,
	}
	r.downloads[key] = download
	return download, nil
}

func (r *memoryRepository) AuthorizeDownload(_ context.Context, attachmentID, userID int64, authorizedAt time.Time) (domain.Download, error) {
	key := downloadKey(attachmentID, userID)
	download, ok := r.downloads[key]
	if !ok {
		return domain.Download{}, domain.ErrDownloadRecordMismatch
	}
	download.Status = domain.DownloadStatusAuthorized
	download.AuthorizedAt = &authorizedAt
	r.downloads[key] = download
	return download, nil
}

func activeAttachment(id, ownerID, priceCredits int64) domain.Attachment {
	return domain.Attachment{
		ID:           id,
		TopicID:      8,
		OwnerID:      ownerID,
		ObjectKey:    "topics/8/attachment",
		OriginalName: "guide.pdf",
		ContentType:  "application/pdf",
		SizeBytes:    256,
		PriceCredits: priceCredits,
		Status:       domain.AttachmentStatusActive,
	}
}

func downloadKey(attachmentID, userID int64) string {
	return strconv.FormatInt(attachmentID, 10) + ":" + strconv.FormatInt(userID, 10)
}
