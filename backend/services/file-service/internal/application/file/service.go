package file

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	domain "file-service/internal/domain/file"
)

const (
	maxObjectKeyLength       = 512
	maxOriginalNameLength    = 255
	maxContentTypeLength     = 255
	maxBizTypeLength         = 64
	MaxFileSizeBytes         = 50 << 20
	MaxFileCapacityBytes     = 10 << 40
	DefaultFileCapacityBytes = 100 << 20
	maxDownloadHistoryLimit  = 100
	topicStatusPublished     = int32(2)
)

type CreditTransferCommand struct {
	PayerUserID       int64
	PayeeUserID       int64
	Amount            int64
	DebitReason       string
	DebitDescription  string
	CreditReason      string
	CreditDescription string
	SourceEventID     string
	SourceType        string
	SourceID          int64
}

type CreditCharger interface {
	TransferCredits(ctx context.Context, command CreditTransferCommand) error
}

type MembershipEntitlementReader interface {
	HasActiveMembership(ctx context.Context, userID int64) (bool, error)
}

type Topic struct {
	ID       int64
	AuthorID int64
	Status   int32
}

type TopicReader interface {
	GetTopic(ctx context.Context, topicID int64) (Topic, error)
}

type ObjectDeleter interface {
	Delete(ctx context.Context, key string) error
}

type ServiceOption func(*Service)

func WithAccountErasure(repository domain.AccountErasureRepository, objects ObjectDeleter) ServiceOption {
	return func(service *Service) {
		service.erasureRepository = repository
		service.objects = objects
	}
}

func WithFileCapacity(capacityBytes int64) ServiceOption {
	return func(service *Service) {
		if capacityBytes > 0 {
			service.fileCapacityBytes = capacityBytes
		}
	}
}

type Service struct {
	repo                   domain.Repository
	charger                CreditCharger
	membershipEntitlements MembershipEntitlementReader
	topics                 TopicReader
	erasureRepository      domain.AccountErasureRepository
	objects                ObjectDeleter
	fileCapacityBytes      int64
	now                    func() time.Time
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

type CreateFileCommand struct {
	OwnerID      int64
	BizType      string
	ObjectKey    string
	OriginalName string
	ContentType  string
	SizeBytes    int64
}

type DownloadAuthorization struct {
	Attachment        domain.Attachment
	AlreadyAuthorized bool
	ChargedCredits    int64
}

func NewService(repo domain.Repository, charger CreditCharger, membershipEntitlements MembershipEntitlementReader, topics TopicReader, options ...ServiceOption) *Service {
	service := &Service{repo: repo, charger: charger, membershipEntitlements: membershipEntitlements, topics: topics, fileCapacityBytes: DefaultFileCapacityBytes, now: time.Now}
	for _, option := range options {
		if option != nil {
			option(service)
		}
	}
	return service
}

func (s *Service) EraseUserData(ctx context.Context, userID, deletionJobID int64, policyVersion int32) (domain.AccountErasureResult, error) {
	if userID <= 0 || deletionJobID <= 0 || policyVersion <= 0 {
		return domain.AccountErasureResult{}, domain.ErrInvalidAccountErasure
	}
	if s == nil || s.erasureRepository == nil {
		return domain.AccountErasureResult{}, domain.ErrAccountErasureUnavailable
	}
	result, objects, err := s.erasureRepository.BeginAccountErasure(ctx, userID, deletionJobID, policyVersion)
	if err != nil {
		return domain.AccountErasureResult{}, err
	}
	for _, object := range objects {
		if s.objects == nil {
			return domain.AccountErasureResult{}, domain.ErrAccountErasureUnavailable
		}
		if err := s.objects.Delete(ctx, object.ObjectKey); err != nil {
			return domain.AccountErasureResult{}, fmt.Errorf("delete erased file object %d: %w", erasureObjectID(object), err)
		}
		if object.FileID > 0 {
			if err := s.erasureRepository.CompleteAccountErasureFileObject(ctx, userID, object.FileID, s.now()); err != nil {
				return domain.AccountErasureResult{}, err
			}
		} else {
			if err := s.erasureRepository.CompleteAccountErasureObject(ctx, userID, object.AttachmentID, s.now()); err != nil {
				return domain.AccountErasureResult{}, err
			}
		}
	}
	completed, err := s.erasureRepository.CompleteAccountErasure(ctx, userID, s.now())
	if err != nil {
		return domain.AccountErasureResult{}, err
	}
	if completed.ArchivedAttachments < result.ArchivedAttachments || completed.DeletedDownloads < result.DeletedDownloads {
		return domain.AccountErasureResult{}, domain.ErrAccountErasureUnavailable
	}
	return completed, nil
}

func erasureObjectID(object domain.ErasureObject) int64 {
	if object.FileID > 0 {
		return object.FileID
	}
	return object.AttachmentID
}

func (s *Service) CreateFile(ctx context.Context, command CreateFileCommand) (domain.File, error) {
	file, err := normalizeFile(command, s.now())
	if err != nil {
		return domain.File{}, err
	}
	return s.repo.CreateFile(ctx, file, s.fileCapacityBytes)
}

func (s *Service) GetFileUsage(ctx context.Context, userID int64) (domain.FileUsage, error) {
	if userID <= 0 {
		return domain.FileUsage{}, domain.ErrInvalidFile
	}
	usedBytes, err := s.repo.GetFileUsage(ctx, userID)
	if err != nil {
		return domain.FileUsage{}, err
	}
	fileCount, err := s.repo.GetFileCount(ctx, userID)
	if err != nil {
		return domain.FileUsage{}, err
	}
	overrideBytes, err := s.repo.GetFileCapacityOverride(ctx, userID)
	if err != nil {
		return domain.FileUsage{}, err
	}
	capacityBytes := effectiveFileCapacity(s.fileCapacityBytes, overrideBytes)
	remainingBytes := capacityBytes - usedBytes
	if remainingBytes < 0 {
		remainingBytes = 0
	}
	return domain.FileUsage{
		UsedBytes:             usedBytes,
		CapacityBytes:         capacityBytes,
		RemainingBytes:        remainingBytes,
		FileCount:             fileCount,
		PolicyCapacityBytes:   s.fileCapacityBytes,
		MaxFileSizeBytes:      MaxFileSizeBytes,
		OverrideCapacityBytes: overrideBytes,
	}, nil
}

func (s *Service) GetDriveChart(ctx context.Context, query domain.DriveChartQuery) (domain.DriveChart, error) {
	query.Span = strings.ToLower(strings.TrimSpace(query.Span))
	if query.Span != domain.DriveChartSpanHour && query.Span != domain.DriveChartSpanDay {
		return domain.DriveChart{}, domain.ErrDriveChartSpanInvalid
	}
	if query.Limit < 1 || query.Limit > domain.MaxDriveChartLimit {
		return domain.DriveChart{}, domain.ErrDriveChartLimitInvalid
	}
	if query.Offset != nil && (*query.Offset < 0 || *query.Offset > domain.MaxDriveChartOffsetMillis) {
		return domain.DriveChart{}, domain.ErrDriveChartOffsetInvalid
	}
	if query.OwnerID < 0 {
		return domain.DriveChart{}, domain.ErrDriveChartOwnerInvalid
	}
	repository, ok := s.repo.(domain.DriveChartRepository)
	if !ok {
		return domain.DriveChart{}, domain.ErrDriveChartRepositoryUnavailable
	}
	return repository.GetDriveChart(ctx, query)
}

func (s *Service) SetFileCapacity(ctx context.Context, userID int64, overrideBytes *int64) (domain.FileUsage, error) {
	if userID <= 0 || (overrideBytes != nil && (*overrideBytes < 0 || *overrideBytes > MaxFileCapacityBytes)) {
		return domain.FileUsage{}, domain.ErrInvalidFileCapacity
	}
	if err := s.repo.SetFileCapacityOverride(ctx, userID, overrideBytes, s.now()); err != nil {
		return domain.FileUsage{}, err
	}
	return s.GetFileUsage(ctx, userID)
}

func effectiveFileCapacity(policyBytes int64, overrideBytes *int64) int64 {
	if overrideBytes != nil && *overrideBytes > policyBytes {
		return *overrideBytes
	}
	return policyBytes
}

func (s *Service) ListFiles(ctx context.Context, userID int64, limit, offset int32) ([]domain.File, int64, error) {
	if userID <= 0 || limit <= 0 || limit > maxDownloadHistoryLimit || offset < 0 {
		return nil, 0, domain.ErrInvalidFile
	}
	return s.repo.ListUserFiles(ctx, userID, limit, offset)
}

func (s *Service) GetFile(ctx context.Context, userID, fileID int64) (domain.File, error) {
	if userID <= 0 || fileID <= 0 {
		return domain.File{}, domain.ErrInvalidFile
	}
	return s.repo.GetFile(ctx, userID, fileID)
}

func (s *Service) DeleteFile(ctx context.Context, userID, fileID int64) (domain.File, error) {
	if userID <= 0 || fileID <= 0 {
		return domain.File{}, domain.ErrInvalidFile
	}
	file, err := s.repo.GetFile(ctx, userID, fileID)
	if err != nil {
		return domain.File{}, err
	}
	if file.BizType == "images" || file.BizType == "avatars" {
		return domain.File{}, domain.ErrManagedMediaDeletionForbidden
	}
	if s.objects == nil {
		return domain.File{}, domain.ErrFileStorageUnavailable
	}
	file, err = s.repo.BeginFileDeletion(ctx, userID, fileID, s.now())
	if err != nil {
		return domain.File{}, err
	}
	if err := s.objects.Delete(ctx, file.ObjectKey); err != nil {
		return domain.File{}, fmt.Errorf("delete file object: %w", err)
	}
	return s.repo.CompleteFileDeletion(ctx, userID, fileID, s.now())
}

func normalizeFile(command CreateFileCommand, now time.Time) (domain.File, error) {
	file := domain.File{
		OwnerID:      command.OwnerID,
		BizType:      strings.TrimSpace(command.BizType),
		ObjectKey:    strings.TrimSpace(command.ObjectKey),
		OriginalName: strings.TrimSpace(command.OriginalName),
		ContentType:  strings.TrimSpace(command.ContentType),
		SizeBytes:    command.SizeBytes,
		Status:       domain.FileStatusActive,
		CreatedAt:    now,
		UpdatedAt:    now,
	}
	if file.ContentType == "" {
		file.ContentType = "application/octet-stream"
	}
	if file.BizType == "" || len(file.BizType) > maxBizTypeLength || strings.ContainsAny(file.BizType, "/\\") || strings.ContainsRune(file.BizType, '\x00') ||
		file.OwnerID <= 0 || file.ObjectKey == "" || len(file.ObjectKey) > maxObjectKeyLength || strings.ContainsRune(file.ObjectKey, '\x00') ||
		file.OriginalName == "" || len(file.OriginalName) > maxOriginalNameLength || strings.ContainsAny(file.OriginalName, "\\/") || strings.ContainsRune(file.OriginalName, '\x00') ||
		len(file.ContentType) > maxContentTypeLength || strings.ContainsRune(file.ContentType, '\x00') || file.SizeBytes <= 0 || file.SizeBytes > MaxFileSizeBytes {
		return domain.File{}, domain.ErrInvalidFile
	}
	return file, nil
}

func (s *Service) CreateAttachment(ctx context.Context, command CreateAttachmentCommand) (domain.Attachment, error) {
	attachment, err := normalizeAttachment(command, s.now())
	if err != nil {
		return domain.Attachment{}, err
	}
	if err := s.ensureTopicOwnedAndPublished(ctx, attachment.TopicID, attachment.OwnerID); err != nil {
		return domain.Attachment{}, err
	}
	if attachment.PriceCredits > 0 {
		if err := s.ensureMembershipEntitlement(ctx, attachment.OwnerID); err != nil {
			return domain.Attachment{}, err
		}
	}
	return s.repo.CreateAttachment(ctx, attachment, s.fileCapacityBytes)
}

func (s *Service) ListTopicAttachments(ctx context.Context, topicID int64) ([]domain.Attachment, error) {
	if topicID <= 0 {
		return nil, domain.ErrInvalidAttachment
	}
	if _, err := s.ensureTopicPublished(ctx, topicID); err != nil {
		return nil, err
	}
	return s.repo.ListTopicAttachments(ctx, topicID)
}

func (s *Service) ListUserAttachmentDownloads(ctx context.Context, userID, topicID int64, limit, offset int32) (domain.AttachmentDownloadList, error) {
	if userID <= 0 || topicID < 0 || limit <= 0 || limit > maxDownloadHistoryLimit || offset < 0 {
		return domain.AttachmentDownloadList{}, domain.ErrInvalidDownload
	}
	return s.repo.ListUserAttachmentDownloads(ctx, userID, topicID, limit, offset)
}

func (s *Service) ListUserAttachmentSales(ctx context.Context, userID int64, limit, offset int32) (domain.AttachmentSaleList, error) {
	if userID <= 0 || limit <= 0 || limit > maxDownloadHistoryLimit || offset < 0 {
		return domain.AttachmentSaleList{}, domain.ErrInvalidDownload
	}
	return s.repo.ListUserAttachmentSales(ctx, userID, limit, offset)
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
	attachment, err := s.repo.GetAttachment(ctx, attachmentID)
	if err != nil {
		return domain.Attachment{}, err
	}
	if attachment.OwnerID != ownerID {
		return domain.Attachment{}, domain.ErrAttachmentOwnerMismatch
	}
	if attachment.Status != domain.AttachmentStatusActive {
		return domain.Attachment{}, domain.ErrAttachmentArchived
	}
	if err := s.ensureTopicOwnedAndPublished(ctx, attachment.TopicID, ownerID); err != nil {
		return domain.Attachment{}, err
	}
	if priceCredits > 0 {
		if err := s.ensureMembershipEntitlement(ctx, ownerID); err != nil {
			return domain.Attachment{}, err
		}
	}
	return s.repo.UpdateAttachmentPrice(ctx, attachmentID, ownerID, priceCredits, s.now())
}

func (s *Service) ensureMembershipEntitlement(ctx context.Context, userID int64) error {
	if s.membershipEntitlements == nil {
		return domain.ErrMembershipServiceUnavailable
	}
	active, err := s.membershipEntitlements.HasActiveMembership(ctx, userID)
	if err != nil {
		return domain.ErrMembershipServiceUnavailable
	}
	if !active {
		return domain.ErrMembershipEntitlementRequired
	}
	return nil
}

func (s *Service) ensureTopicOwnedAndPublished(ctx context.Context, topicID, ownerID int64) error {
	topic, err := s.ensureTopicPublished(ctx, topicID)
	if err != nil {
		return err
	}
	if topic.AuthorID != ownerID {
		return domain.ErrAttachmentTopicOwnerMismatch
	}
	return nil
}

func (s *Service) ensureTopicPublished(ctx context.Context, topicID int64) (Topic, error) {
	if s.topics == nil {
		return Topic{}, domain.ErrContentServiceUnavailable
	}
	topic, err := s.topics.GetTopic(ctx, topicID)
	if err != nil {
		if errors.Is(err, domain.ErrAttachmentTopicUnavailable) {
			return Topic{}, err
		}
		return Topic{}, domain.ErrContentServiceUnavailable
	}
	if topic.ID != topicID || topic.Status != topicStatusPublished {
		return Topic{}, domain.ErrAttachmentTopicUnavailable
	}
	return topic, nil
}

func (s *Service) AuthorizeDownload(ctx context.Context, attachmentID, userID int64) (DownloadAuthorization, error) {
	if attachmentID <= 0 || userID <= 0 {
		return DownloadAuthorization{}, domain.ErrInvalidDownload
	}
	attachment, err := s.repo.GetAttachment(ctx, attachmentID)
	if err != nil {
		return DownloadAuthorization{}, err
	}
	if _, err := s.ensureTopicPublished(ctx, attachment.TopicID); err != nil {
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

	sourceEventID := attachmentDownloadEventID(attachment.ID, userID)
	existing, found, err := s.repo.GetDownload(ctx, attachment.ID, userID)
	if err != nil {
		return DownloadAuthorization{}, err
	}
	if found {
		if existing.SourceEventID != sourceEventID {
			return DownloadAuthorization{}, domain.ErrDownloadRecordMismatch
		}
		if existing.Status == domain.DownloadStatusAuthorized {
			return DownloadAuthorization{Attachment: attachment, AlreadyAuthorized: true}, nil
		}
		if existing.Status != domain.DownloadStatusPending {
			return DownloadAuthorization{}, domain.ErrDownloadRecordMismatch
		}
	}

	charge := attachment.PriceCredits
	if attachment.OwnerID == userID {
		charge = 0
	}
	if found {
		charge = existing.ChargedCredits
	}
	if charge < 0 || (attachment.OwnerID == userID && charge != 0) {
		return DownloadAuthorization{}, domain.ErrDownloadRecordMismatch
	}
	if charge > 0 {
		if err := s.ensureActivePaidAttachmentSaleMembership(ctx, attachment.OwnerID); err != nil {
			return DownloadAuthorization{}, err
		}
	}

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
		return s.charger.TransferCredits(ctx, CreditTransferCommand{
			PayerUserID:       userID,
			PayeeUserID:       attachment.OwnerID,
			Amount:            charge,
			DebitReason:       "attachment_download",
			DebitDescription:  fmt.Sprintf("下载付费附件《%s》", attachment.OriginalName),
			CreditReason:      "attachment_sale",
			CreditDescription: fmt.Sprintf("售卖付费附件《%s》", attachment.OriginalName),
			SourceEventID:     sourceEventID,
			SourceType:        "attachment",
			SourceID:          attachment.ID,
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

func (s *Service) ensureActivePaidAttachmentSaleMembership(ctx context.Context, ownerID int64) error {
	if s.membershipEntitlements == nil {
		return domain.ErrMembershipServiceUnavailable
	}
	active, err := s.membershipEntitlements.HasActiveMembership(ctx, ownerID)
	if err != nil {
		return domain.ErrMembershipServiceUnavailable
	}
	if !active {
		return domain.ErrPaidAttachmentSalesMembershipInactive
	}
	return nil
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
