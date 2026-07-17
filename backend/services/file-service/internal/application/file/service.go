package file

import (
	"context"
	"fmt"
	"strings"
	"time"

	domain "file-service/internal/domain/file"
)

const (
	maxObjectKeyLength    = 512
	maxOriginalNameLength = 255
	maxContentTypeLength  = 255
)

type CreditDebitCommand struct {
	UserID        int64
	Amount        int64
	Reason        string
	Description   string
	SourceEventID string
	SourceType    string
	SourceID      int64
}

type CreditCharger interface {
	DebitCredits(ctx context.Context, command CreditDebitCommand) error
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

func (s *Service) GetAttachment(ctx context.Context, attachmentID int64) (domain.Attachment, error) {
	if attachmentID <= 0 {
		return domain.Attachment{}, domain.ErrInvalidAttachment
	}
	attachment, err := s.repo.GetAttachment(ctx, attachmentID)
	if err != nil {
		return domain.Attachment{}, err
	}
	if attachment.Status != domain.AttachmentStatusActive {
		return domain.Attachment{}, domain.ErrAttachmentArchived
	}
	return attachment, nil
}

func (s *Service) ArchiveAttachment(ctx context.Context, attachmentID, ownerID int64) (domain.Attachment, error) {
	if attachmentID <= 0 || ownerID <= 0 {
		return domain.Attachment{}, domain.ErrInvalidAttachment
	}
	return s.repo.ArchiveAttachment(ctx, attachmentID, ownerID, s.now())
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
	if download.SourceEventID != sourceEventID || download.ChargedCredits != charge {
		return DownloadAuthorization{}, domain.ErrDownloadRecordMismatch
	}
	if download.Status == domain.DownloadStatusAuthorized {
		return DownloadAuthorization{Attachment: attachment, AlreadyAuthorized: true}, nil
	}
	if download.Status != domain.DownloadStatusPending {
		return DownloadAuthorization{}, domain.ErrDownloadRecordMismatch
	}

	if charge > 0 {
		if s.charger == nil {
			return DownloadAuthorization{}, domain.ErrCreditServiceUnavailable
		}
		if err := s.charger.DebitCredits(ctx, CreditDebitCommand{
			UserID:        userID,
			Amount:        charge,
			Reason:        "attachment_download",
			Description:   fmt.Sprintf("下载付费附件《%s》", attachment.OriginalName),
			SourceEventID: sourceEventID,
			SourceType:    "attachment",
			SourceID:      attachment.ID,
		}); err != nil {
			return DownloadAuthorization{}, err
		}
	}

	if _, err := s.repo.AuthorizeDownload(ctx, attachment.ID, userID, s.now()); err != nil {
		return DownloadAuthorization{}, err
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
