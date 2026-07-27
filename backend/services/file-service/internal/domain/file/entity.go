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

type Repository interface {
	EnsureSchema(ctx context.Context) error
	CreateAttachment(ctx context.Context, attachment Attachment) (Attachment, error)
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
