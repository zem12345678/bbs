package file

import (
	"context"
	"fmt"
	"strings"
	"time"

	domain "file-service/internal/domain/file"
)

const (
	maxObjectKeyLength      = 512
	maxOriginalNameLength   = 255
	maxContentTypeLength    = 255
	maxDownloadHistoryLimit = 100
)

type CreditCommand struct {
	UserID        int64
	Amount        int64
	Reason        string
	Description   string
	SourceEventID string
	SourceType    string
	SourceID      int64
}

type CreditCharger interface {
	DebitCredits(ctx context.Context, command CreditCommand) error
	CreditCredits(ctx context.Context, command CreditCommand) error
}

type Service struct {
	repo    domain.Repository
	charger CreditCharger
	now     func() time.Time
}

type CreateAttachmentCommand struct {
	TopicID      int64
	OwnerID      int64
	ObjectKey    string
	OriginalName string
	ContentType  string
	SizeBytes    int64
	PriceCredits int64
}

type DownloadAuthorization struct {
	Attachment        domain.Attachment
	AlreadyAuthorized bool
	ChargedCredits    int64
}

func NewService(repo domain.Repository, charger CreditCharger) *Service {
	return &Service{repo: repo, charger: charger, now: time.Now}
}

func (s *Service) CreateAttachment(ctx context.Context, command CreateAttachmentCommand) (domain.Attachment, error) {
	attachment, err := normalizeAttachment(command, s.now())
	if err != nil {
		return domain.Attachment{}, err
	}
	return s.repo.CreateAttachment(ctx, attachment)
}

func (s *Service) ListTopicAttachments(ctx context.Context, topicID int64) ([]domain.Attachment, error) {
	if topicID <= 0 {
		return nil, domain.ErrInvalidAttachment
	}
	return s.repo.ListTopicAttachments(ctx, topicID)
}

func (s *Service) ListUserAttachmentDownloads(ctx context.Context, userID int64, limit, offset int32) ([]domain.AttachmentDownload, error) {
	if userID <= 0 || limit <= 0 || limit > maxDownloadHistoryLimit || offset < 0 {
		return nil, domain.ErrInvalidDownload
	}
	return s.repo.ListUserAttachmentDownloads(ctx, userID, limit, offset)
}

func (s *Service) GetAttachment(ctx context.Context, attachmentID int64) (domain.Attachment, error) {
	if attachmentID <= 0 {
		return domain.Attachment{}, domain.ErrInvalidAttachment
	}
	return s.repo.GetAttachment(ctx, attachmentID)
}

func (s *Service) ArchiveAttachment(ctx context.Context, attachmentID, ownerID int64) (domain.Attachment, error) {
	if attachmentID <= 0 || ownerID <= 0 {
		return domain.Attachment{}, domain.ErrInvalidAttachment
	}
	return s.repo.ArchiveAttachment(ctx, attachmentID, ownerID, s.now())
}

func (s *Service) UpdateAttachmentPrice(ctx context.Context, attachmentID, ownerID, priceCredits int64) (domain.Attachment, error) {
	if attachmentID <= 0 || ownerID <= 0 || priceCredits < 0 {
		return domain.Attachment{}, domain.ErrInvalidAttachment
	}
	return s.repo.UpdateAttachmentPrice(ctx, attachmentID, ownerID, priceCredits, s.now())
}

func (s *Service) AuthorizeDownload(ctx context.Context, attachmentID, userID int64) (DownloadAuthorization, error) {
	if attachmentID <= 0 || userID <= 0 {
		return DownloadAuthorization{}, domain.ErrInvalidDownload
	}
	attachment, err := s.repo.GetAttachment(ctx, attachmentID)
	if err != nil {
		return DownloadAuthorization{}, err
	}
	if attachment.Status != domain.AttachmentStatusActive {
		// Archiving removes the public listing but does not revoke a completed purchase.
		download, found, err := s.repo.GetDownload(ctx, attachment.ID, userID)
		if err != nil {
			return DownloadAuthorization{}, err
		}
		if found && download.Status == domain.DownloadStatusAuthorized && download.SourceEventID == attachmentDownloadEventID(attachment.ID, userID) {
			return DownloadAuthorization{Attachment: attachment, AlreadyAuthorized: true}, nil
		}
		return DownloadAuthorization{}, domain.ErrAttachmentArchived
	}

	charge := attachment.PriceCredits
	if attachment.OwnerID == userID {
		charge = 0
	}
	sourceEventID := attachmentDownloadEventID(attachment.ID, userID)
	download, err := s.repo.EnsureDownload(ctx, attachment.ID, userID, sourceEventID, charge, s.now())
	if err != nil {
		return DownloadAuthorization{}, err
	}
	if download.SourceEventID != sourceEventID {
		return DownloadAuthorization{}, domain.ErrDownloadRecordMismatch
	}
	if download.Status == domain.DownloadStatusAuthorized {
		return DownloadAuthorization{Attachment: attachment, AlreadyAuthorized: true}, nil
	}
	if download.Status != domain.DownloadStatusPending {
		return DownloadAuthorization{}, domain.ErrDownloadRecordMismatch
	}
	charge = download.ChargedCredits
	if charge < 0 || (attachment.OwnerID == userID && charge != 0) {
		return DownloadAuthorization{}, domain.ErrDownloadRecordMismatch
	}

	_, alreadyAuthorized, err := s.repo.CompleteDownloadAuthorization(ctx, attachment.ID, userID, s.now(), func(ctx context.Context) error {
		if charge == 0 {
			return nil
		}
		if s.charger == nil {
			return domain.ErrCreditServiceUnavailable
		}
		if err := s.charger.DebitCredits(ctx, CreditCommand{
			UserID:        userID,
			Amount:        charge,
			Reason:        "attachment_download",
			Description:   fmt.Sprintf("下载付费附件《%s》", attachment.OriginalName),
			SourceEventID: sourceEventID,
			SourceType:    "attachment",
			SourceID:      attachment.ID,
		}); err != nil {
			return err
		}
		return s.charger.CreditCredits(ctx, CreditCommand{
			UserID:        attachment.OwnerID,
			Amount:        charge,
			Reason:        "attachment_sale",
			Description:   fmt.Sprintf("售卖付费附件《%s》", attachment.OriginalName),
			SourceEventID: sourceEventID,
			SourceType:    "attachment",
			SourceID:      attachment.ID,
		})
	})
	if err != nil {
		return DownloadAuthorization{}, err
	}
	if alreadyAuthorized {
		return DownloadAuthorization{Attachment: attachment, AlreadyAuthorized: true}, nil
	}
	return DownloadAuthorization{Attachment: attachment, ChargedCredits: charge}, nil
}

func attachmentDownloadEventID(attachmentID, userID int64) string {
	return fmt.Sprintf("attachment-download:%d:%d", attachmentID, userID)
}

func normalizeAttachment(command CreateAttachmentCommand, now time.Time) (domain.Attachment, error) {
	attachment := domain.Attachment{
		TopicID:      command.TopicID,
		OwnerID:      command.OwnerID,
		ObjectKey:    strings.TrimSpace(command.ObjectKey),
		OriginalName: strings.TrimSpace(command.OriginalName),
		ContentType:  strings.TrimSpace(command.ContentType),
		SizeBytes:    command.SizeBytes,
		PriceCredits: command.PriceCredits,
		Status:       domain.AttachmentStatusActive,
		CreatedAt:    now,
		UpdatedAt:    now,
	}
	if attachment.TopicID <= 0 || attachment.OwnerID <= 0 || attachment.SizeBytes < 0 || attachment.PriceCredits < 0 {
		return domain.Attachment{}, domain.ErrInvalidAttachment
	}
	if attachment.ObjectKey == "" || len(attachment.ObjectKey) > maxObjectKeyLength ||
		attachment.OriginalName == "" || len(attachment.OriginalName) > maxOriginalNameLength ||
		attachment.ContentType == "" || len(attachment.ContentType) > maxContentTypeLength ||
		strings.ContainsAny(attachment.OriginalName, "\\\\/") {
		return domain.Attachment{}, domain.ErrInvalidAttachment
	}
	return attachment, nil
}
