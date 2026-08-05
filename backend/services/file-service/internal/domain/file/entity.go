package file

import (
	"context"
	"errors"
	"time"
)

type AttachmentStatus string

const (
	AttachmentStatusActive   AttachmentStatus = "ACTIVE"
	AttachmentStatusArchived AttachmentStatus = "ARCHIVED"

	DownloadStatusPending    = "PENDING"
	DownloadStatusAuthorized = "AUTHORIZED"
)

var (
	ErrAttachmentNotFound                    = errors.New("attachment not found")
	ErrAttachmentArchived                    = errors.New("attachment is archived")
	ErrAttachmentOwnerMismatch               = errors.New("attachment does not belong to user")
	ErrAttachmentObjectKeyTaken              = errors.New("attachment object key already exists")
	ErrInvalidAttachment                     = errors.New("invalid attachment")
	ErrInvalidDownload                       = errors.New("invalid attachment download")
	ErrDownloadRecordMismatch                = errors.New("attachment download record does not match authorization")
	ErrInsufficientCredits                   = errors.New("insufficient credits")
	ErrCreditServiceUnavailable              = errors.New("credit service unavailable")
	ErrMembershipEntitlementRequired         = errors.New("active membership entitlement required for paid attachments")
	ErrPaidAttachmentSalesMembershipInactive = errors.New("paid attachment sales unavailable because the author membership entitlement is inactive")
	ErrMembershipServiceUnavailable          = errors.New("membership service unavailable")
	ErrAttachmentTopicUnavailable            = errors.New("attachment topic is unavailable")
	ErrAttachmentTopicOwnerMismatch          = errors.New("attachment topic does not belong to user")
	ErrContentServiceUnavailable             = errors.New("content service unavailable")
	ErrInvalidAccountErasure                 = errors.New("invalid file account erasure")
	ErrAccountErasureUnavailable             = errors.New("file account erasure unavailable")
	ErrAccountErased                         = errors.New("file account erased")
	ErrFileNotFound                          = errors.New("file not found")
	ErrFileOwnerMismatch                     = errors.New("file does not belong to user")
	ErrFileDeleted                           = errors.New("file is deleted")
	ErrManagedMediaDeletionForbidden         = errors.New("managed media files cannot be deleted directly")
	ErrInvalidFile                           = errors.New("invalid file")
	ErrFileCapacityExceeded                  = errors.New("file capacity exceeded")
	ErrInvalidFileCapacity                   = errors.New("invalid file capacity")
	ErrFileStorageUnavailable                = errors.New("file storage unavailable")
	ErrFileObjectKeyTaken                    = errors.New("file object key already exists")
)

type Attachment struct {
	ID           int64
	TopicID      int64
	OwnerID      int64
	ObjectKey    string
	OriginalName string
	ContentType  string
	SizeBytes    int64
	PriceCredits int64
	Status       AttachmentStatus
	CreatedAt    time.Time
	UpdatedAt    time.Time
	ArchivedAt   *time.Time
}

type FileStatus string

const (
	FileStatusActive   FileStatus = "ACTIVE"
	FileStatusDeleting FileStatus = "DELETING"
	FileStatusDeleted  FileStatus = "DELETED"
	FileStatusErased   FileStatus = "ERASED"
)

type File struct {
	ID           int64
	OwnerID      int64
	BizType      string
	ObjectKey    string
	OriginalName string
	ContentType  string
	SizeBytes    int64
	Status       FileStatus
	CreatedAt    time.Time
	UpdatedAt    time.Time
	DeletedAt    *time.Time
}

type FileUsage struct {
	UsedBytes             int64
	CapacityBytes         int64
	RemainingBytes        int64
	FileCount             int64
	PolicyCapacityBytes   int64
	MaxFileSizeBytes      int64
	OverrideCapacityBytes *int64
}

type Download struct {
	AttachmentID   int64
	UserID         int64
	Status         string
	SourceEventID  string
	ChargedCredits int64
	CreatedAt      time.Time
	AuthorizedAt   *time.Time
}

type AttachmentDownload struct {
	Attachment     Attachment
	Status         string
	ChargedCredits int64
	CreatedAt      time.Time
	AuthorizedAt   *time.Time
}

type AttachmentDownloadList struct {
	Items []AttachmentDownload
	Total int64
}

type AttachmentSale struct {
	Attachment    Attachment
	EarnedCredits int64
	SoldAt        time.Time
}

type AttachmentSaleList struct {
	Items              []AttachmentSale
	Total              int64
	TotalEarnedCredits int64
}

type ErasureObject struct {
	AttachmentID int64
	FileID       int64
	ObjectKey    string
}

type AccountErasureResult struct {
	ArchivedAttachments int64
	DeletedDownloads    int64
	DeletedObjects      int64
}

type AccountErasureRepository interface {
	BeginAccountErasure(ctx context.Context, userID, deletionJobID int64, policyVersion int32) (AccountErasureResult, []ErasureObject, error)
	CompleteAccountErasureObject(ctx context.Context, userID, attachmentID int64, deletedAt time.Time) error
	CompleteAccountErasureFileObject(ctx context.Context, userID, fileID int64, deletedAt time.Time) error
	CompleteAccountErasure(ctx context.Context, userID int64, completedAt time.Time) (AccountErasureResult, error)
}

type Repository interface {
	EnsureSchema(ctx context.Context) error
	CreateFile(ctx context.Context, file File, capacityBytes int64) (File, error)
	GetFileUsage(ctx context.Context, userID int64) (int64, error)
	GetFileCount(ctx context.Context, userID int64) (int64, error)
	GetFileCapacityOverride(ctx context.Context, userID int64) (*int64, error)
	SetFileCapacityOverride(ctx context.Context, userID int64, overrideBytes *int64, updatedAt time.Time) error
	ListUserFiles(ctx context.Context, userID int64, limit, offset int32) ([]File, int64, error)
	GetFile(ctx context.Context, userID, fileID int64) (File, error)
	BeginFileDeletion(ctx context.Context, userID, fileID int64, updatedAt time.Time) (File, error)
	CompleteFileDeletion(ctx context.Context, userID, fileID int64, deletedAt time.Time) (File, error)
	CreateAttachment(ctx context.Context, attachment Attachment, capacityBytes int64) (Attachment, error)
	ListTopicAttachments(ctx context.Context, topicID int64) ([]Attachment, error)
	ListUserAttachmentDownloads(ctx context.Context, userID, topicID int64, limit, offset int32) (AttachmentDownloadList, error)
	ListUserAttachmentSales(ctx context.Context, userID int64, limit, offset int32) (AttachmentSaleList, error)
	GetAttachment(ctx context.Context, attachmentID int64) (Attachment, error)
	GetDownload(ctx context.Context, attachmentID, userID int64) (Download, bool, error)
	ArchiveAttachment(ctx context.Context, attachmentID, ownerID int64, archivedAt time.Time) (Attachment, error)
	UpdateAttachmentPrice(ctx context.Context, attachmentID, ownerID, priceCredits int64, updatedAt time.Time) (Attachment, error)
	EnsureDownload(ctx context.Context, attachmentID, userID int64, sourceEventID string, chargedCredits int64, createdAt time.Time) (Download, error)
	CompleteDownloadAuthorization(ctx context.Context, attachmentID, userID int64, authorizedAt time.Time, settle func(context.Context) error) (download Download, alreadyAuthorized bool, err error)
}
