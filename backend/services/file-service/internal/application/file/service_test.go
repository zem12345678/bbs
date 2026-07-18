package file

import (
	"context"
	"sort"
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
	if _, err := service.UpdateAttachmentPrice(context.Background(), 102, 9, 17); err != nil {
		t.Fatalf("UpdateAttachmentPrice() error = %v", err)
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

func TestUpdateAttachmentPriceKeepsAuthorizedDownloadAvailable(t *testing.T) {
	repo := newMemoryRepository(activeAttachment(107, 9, 7))
	charger := &captureCharger{}
	service := NewService(repo, charger)

	first, err := service.AuthorizeDownload(context.Background(), 107, 42)
	if err != nil {
		t.Fatalf("first AuthorizeDownload() error = %v", err)
	}
	if first.ChargedCredits != 7 {
		t.Fatalf("first authorization charge = %d, want 7", first.ChargedCredits)
	}
	updated, err := service.UpdateAttachmentPrice(context.Background(), 107, 9, 13)
	if err != nil {
		t.Fatalf("UpdateAttachmentPrice() error = %v", err)
	}
	if updated.PriceCredits != 13 {
		t.Fatalf("updated attachment price = %d, want 13", updated.PriceCredits)
	}

	retry, err := service.AuthorizeDownload(context.Background(), 107, 42)
	if err != nil {
		t.Fatalf("authorized buyer retry error = %v", err)
	}
	if !retry.AlreadyAuthorized || retry.ChargedCredits != 0 {
		t.Fatalf("authorized buyer retry = %+v", retry)
	}
	newBuyer, err := service.AuthorizeDownload(context.Background(), 107, 43)
	if err != nil {
		t.Fatalf("new buyer authorization error = %v", err)
	}
	if newBuyer.AlreadyAuthorized || newBuyer.ChargedCredits != 13 {
		t.Fatalf("new buyer authorization = %+v, want a 13-credit authorization", newBuyer)
	}
	if got := repo.downloads[downloadKey(107, 42)].ChargedCredits; got != 7 {
		t.Fatalf("authorized buyer charge snapshot = %d, want 7", got)
	}
	if len(charger.commands) != 2 {
		t.Fatalf("credit debits = %d, want 2", len(charger.commands))
	}
}

func TestUpdateAttachmentPriceRequiresActiveOwner(t *testing.T) {
	repo := newMemoryRepository(activeAttachment(108, 9, 7))
	service := NewService(repo, &captureCharger{})
	if _, err := service.UpdateAttachmentPrice(context.Background(), 108, 42, 13); err != domain.ErrAttachmentOwnerMismatch {
		t.Fatalf("non-owner UpdateAttachmentPrice() error = %v, want owner mismatch", err)
	}
	repo.attachment.Status = domain.AttachmentStatusArchived
	if _, err := service.UpdateAttachmentPrice(context.Background(), 108, 9, 13); err != domain.ErrAttachmentArchived {
		t.Fatalf("archived UpdateAttachmentPrice() error = %v, want archived attachment", err)
	}
	if _, err := service.UpdateAttachmentPrice(context.Background(), 108, 9, -1); err != domain.ErrInvalidAttachment {
		t.Fatalf("negative UpdateAttachmentPrice() error = %v, want invalid attachment", err)
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

func TestListUserAttachmentDownloadsReturnsOnlyAuthorizedDownloads(t *testing.T) {
	repo := newMemoryRepository(activeAttachment(105, 9, 7))
	authorizedAt := time.Date(2026, 7, 18, 9, 0, 0, 0, time.UTC)
	repo.downloads[downloadKey(105, 42)] = domain.Download{
		AttachmentID:   105,
		UserID:         42,
		Status:         domain.DownloadStatusAuthorized,
		ChargedCredits: 7,
		CreatedAt:      authorizedAt.Add(-time.Minute),
		AuthorizedAt:   &authorizedAt,
	}
	repo.downloads[downloadKey(106, 42)] = domain.Download{
		AttachmentID: 106,
		UserID:       42,
		Status:       domain.DownloadStatusPending,
		CreatedAt:    authorizedAt,
	}
	repo.downloads[downloadKey(105, 99)] = domain.Download{
		AttachmentID: 105,
		UserID:       99,
		Status:       domain.DownloadStatusAuthorized,
		CreatedAt:    authorizedAt,
		AuthorizedAt: &authorizedAt,
	}

	downloads, err := NewService(repo, &captureCharger{}).ListUserAttachmentDownloads(context.Background(), 42, 20, 0)
	if err != nil {
		t.Fatalf("ListUserAttachmentDownloads() error = %v", err)
	}
	if len(downloads) != 1 {
		t.Fatalf("downloads = %+v, want one authorized current-user record", downloads)
	}
	if downloads[0].Attachment.ID != 105 || downloads[0].ChargedCredits != 7 || downloads[0].Status != domain.DownloadStatusAuthorized {
		t.Fatalf("download = %+v", downloads[0])
	}
}

func TestListUserAttachmentDownloadsRejectsInvalidPage(t *testing.T) {
	service := NewService(newMemoryRepository(activeAttachment(106, 9, 7)), &captureCharger{})
	if _, err := service.ListUserAttachmentDownloads(context.Background(), 42, 0, 0); err != domain.ErrInvalidDownload {
		t.Fatalf("ListUserAttachmentDownloads() error = %v, want invalid attachment download", err)
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

func (r *memoryRepository) ListUserAttachmentDownloads(_ context.Context, userID int64, limit, offset int32) ([]domain.AttachmentDownload, error) {
	downloads := make([]domain.AttachmentDownload, 0)
	for _, download := range r.downloads {
		if download.UserID != userID || download.Status != domain.DownloadStatusAuthorized {
			continue
		}
		downloads = append(downloads, domain.AttachmentDownload{
			Attachment:     r.attachment,
			Status:         download.Status,
			ChargedCredits: download.ChargedCredits,
			CreatedAt:      download.CreatedAt,
			AuthorizedAt:   download.AuthorizedAt,
		})
	}
	sort.Slice(downloads, func(i, j int) bool {
		return downloads[i].CreatedAt.After(downloads[j].CreatedAt)
	})
	start := int(offset)
	if start >= len(downloads) {
		return []domain.AttachmentDownload{}, nil
	}
	end := start + int(limit)
	if end > len(downloads) {
		end = len(downloads)
	}
	return downloads[start:end], nil
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

func (r *memoryRepository) UpdateAttachmentPrice(_ context.Context, attachmentID, ownerID, priceCredits int64, updatedAt time.Time) (domain.Attachment, error) {
	if r.attachment.ID != attachmentID {
		return domain.Attachment{}, domain.ErrAttachmentNotFound
	}
	if r.attachment.OwnerID != ownerID {
		return domain.Attachment{}, domain.ErrAttachmentOwnerMismatch
	}
	if r.attachment.Status != domain.AttachmentStatusActive {
		return domain.Attachment{}, domain.ErrAttachmentArchived
	}
	r.attachment.PriceCredits = priceCredits
	r.attachment.UpdatedAt = updatedAt
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
