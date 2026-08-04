package grpc

import (
	"context"
	"errors"
	"time"

	pb "file-service/api/proto/filepb"
	app "file-service/internal/application/file"
	domain "file-service/internal/domain/file"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type Handler struct {
	pb.UnimplementedFileServiceServer
	service *app.Service
}

func NewHandler(service *app.Service) *Handler {
	return &Handler{service: service}
}

func (h *Handler) EraseUserData(ctx context.Context, req *pb.EraseUserDataRequest) (*pb.EraseUserDataResponse, error) {
	result, err := h.service.EraseUserData(ctx, req.GetUserId(), req.GetDeletionJobId(), req.GetPolicyVersion())
	if err != nil {
		return nil, toStatus(err)
	}
	return &pb.EraseUserDataResponse{
		Completed:           true,
		ArchivedAttachments: result.ArchivedAttachments,
		DeletedDownloads:    result.DeletedDownloads,
		DeletedObjects:      result.DeletedObjects,
	}, nil
}

func (h *Handler) CreateAttachment(ctx context.Context, req *pb.CreateAttachmentRequest) (*pb.AttachmentResponse, error) {
	attachment, err := h.service.CreateAttachment(ctx, app.CreateAttachmentCommand{
		TopicID:      req.GetTopicId(),
		OwnerID:      req.GetOwnerId(),
		ObjectKey:    req.GetObjectKey(),
		OriginalName: req.GetOriginalName(),
		ContentType:  req.GetContentType(),
		SizeBytes:    req.GetSizeBytes(),
		PriceCredits: req.GetPriceCredits(),
	})
	if err != nil {
		return nil, toStatus(err)
	}
	return &pb.AttachmentResponse{Attachment: toPB(attachment)}, nil
}

func (h *Handler) GetAttachment(ctx context.Context, req *pb.GetAttachmentRequest) (*pb.AttachmentResponse, error) {
	attachment, err := h.service.GetAttachment(ctx, req.GetAttachmentId())
	if err != nil {
		return nil, toStatus(err)
	}
	return &pb.AttachmentResponse{Attachment: toPB(attachment)}, nil
}

func (h *Handler) ListTopicAttachments(ctx context.Context, req *pb.ListTopicAttachmentsRequest) (*pb.AttachmentListResponse, error) {
	attachments, err := h.service.ListTopicAttachments(ctx, req.GetTopicId())
	if err != nil {
		return nil, toStatus(err)
	}
	items := make([]*pb.Attachment, 0, len(attachments))
	for _, attachment := range attachments {
		items = append(items, toPB(attachment))
	}
	return &pb.AttachmentListResponse{Items: items}, nil
}

func (h *Handler) ListUserAttachmentDownloads(ctx context.Context, req *pb.ListUserAttachmentDownloadsRequest) (*pb.AttachmentDownloadListResponse, error) {
	downloads, err := h.service.ListUserAttachmentDownloads(ctx, req.GetUserId(), req.GetTopicId(), req.GetLimit(), req.GetOffset())
	if err != nil {
		return nil, toStatus(err)
	}
	items := make([]*pb.AttachmentDownload, 0, len(downloads.Items))
	for _, download := range downloads.Items {
		items = append(items, downloadToPB(download))
	}
	return &pb.AttachmentDownloadListResponse{Items: items, Total: downloads.Total}, nil
}

func (h *Handler) ListUserAttachmentSales(ctx context.Context, req *pb.ListUserAttachmentSalesRequest) (*pb.AttachmentSaleListResponse, error) {
	sales, err := h.service.ListUserAttachmentSales(ctx, req.GetUserId(), req.GetLimit(), req.GetOffset())
	if err != nil {
		return nil, toStatus(err)
	}
	items := make([]*pb.AttachmentSale, 0, len(sales.Items))
	for _, sale := range sales.Items {
		items = append(items, saleToPB(sale))
	}
	return &pb.AttachmentSaleListResponse{
		Items:              items,
		Total:              sales.Total,
		TotalEarnedCredits: sales.TotalEarnedCredits,
	}, nil
}

func (h *Handler) AuthorizeAttachmentDownload(ctx context.Context, req *pb.AuthorizeAttachmentDownloadRequest) (*pb.DownloadAuthorizationResponse, error) {
	authorization, err := h.service.AuthorizeDownload(ctx, req.GetAttachmentId(), req.GetUserId())
	if err != nil {
		return nil, toStatus(err)
	}
	return &pb.DownloadAuthorizationResponse{
		Attachment:        toPB(authorization.Attachment),
		AlreadyAuthorized: authorization.AlreadyAuthorized,
		ChargedCredits:    authorization.ChargedCredits,
	}, nil
}

func (h *Handler) ArchiveAttachment(ctx context.Context, req *pb.ArchiveAttachmentRequest) (*pb.AttachmentResponse, error) {
	attachment, err := h.service.ArchiveAttachment(ctx, req.GetAttachmentId(), req.GetOwnerId())
	if err != nil {
		return nil, toStatus(err)
	}
	return &pb.AttachmentResponse{Attachment: toPB(attachment)}, nil
}

func (h *Handler) UpdateAttachmentPrice(ctx context.Context, req *pb.UpdateAttachmentPriceRequest) (*pb.AttachmentResponse, error) {
	attachment, err := h.service.UpdateAttachmentPrice(ctx, req.GetAttachmentId(), req.GetOwnerId(), req.GetPriceCredits())
	if err != nil {
		return nil, toStatus(err)
	}
	return &pb.AttachmentResponse{Attachment: toPB(attachment)}, nil
}

func (h *Handler) CreateFile(ctx context.Context, req *pb.CreateFileRequest) (*pb.FileResponse, error) {
	file, err := h.service.CreateFile(ctx, app.CreateFileCommand{
		OwnerID:      req.GetOwnerId(),
		BizType:      req.GetBizType(),
		ObjectKey:    req.GetObjectKey(),
		OriginalName: req.GetOriginalName(),
		ContentType:  req.GetContentType(),
		SizeBytes:    req.GetSizeBytes(),
	})
	if err != nil {
		return nil, toStatus(err)
	}
	return &pb.FileResponse{File: fileToPB(file)}, nil
}

func (h *Handler) ListFiles(ctx context.Context, req *pb.ListFilesRequest) (*pb.FileListResponse, error) {
	items, total, err := h.service.ListFiles(ctx, req.GetOwnerId(), req.GetLimit(), req.GetOffset())
	if err != nil {
		return nil, toStatus(err)
	}
	files := make([]*pb.File, 0, len(items))
	for _, item := range items {
		files = append(files, fileToPB(item))
	}
	return &pb.FileListResponse{Items: files, Total: total}, nil
}

func (h *Handler) GetFile(ctx context.Context, req *pb.GetFileRequest) (*pb.FileResponse, error) {
	file, err := h.service.GetFile(ctx, req.GetOwnerId(), req.GetFileId())
	if err != nil {
		return nil, toStatus(err)
	}
	return &pb.FileResponse{File: fileToPB(file)}, nil
}

func (h *Handler) DeleteFile(ctx context.Context, req *pb.DeleteFileRequest) (*pb.FileResponse, error) {
	file, err := h.service.DeleteFile(ctx, req.GetOwnerId(), req.GetFileId())
	if err != nil {
		return nil, toStatus(err)
	}
	return &pb.FileResponse{File: fileToPB(file)}, nil
}

func toPB(attachment domain.Attachment) *pb.Attachment {
	return &pb.Attachment{
		Id:           attachment.ID,
		TopicId:      attachment.TopicID,
		OwnerId:      attachment.OwnerID,
		ObjectKey:    attachment.ObjectKey,
		OriginalName: attachment.OriginalName,
		ContentType:  attachment.ContentType,
		SizeBytes:    attachment.SizeBytes,
		PriceCredits: attachment.PriceCredits,
		Status:       string(attachment.Status),
		CreatedAt:    millis(attachment.CreatedAt),
		UpdatedAt:    millis(attachment.UpdatedAt),
		ArchivedAt:   millisPointer(attachment.ArchivedAt),
	}
}

func fileToPB(file domain.File) *pb.File {
	return &pb.File{
		Id:           file.ID,
		OwnerId:      file.OwnerID,
		BizType:      file.BizType,
		ObjectKey:    file.ObjectKey,
		OriginalName: file.OriginalName,
		ContentType:  file.ContentType,
		SizeBytes:    file.SizeBytes,
		Status:       string(file.Status),
		CreatedAt:    millis(file.CreatedAt),
		UpdatedAt:    millis(file.UpdatedAt),
		DeletedAt:    millisPointer(file.DeletedAt),
	}
}

func downloadToPB(download domain.AttachmentDownload) *pb.AttachmentDownload {
	return &pb.AttachmentDownload{
		Attachment:     toPB(download.Attachment),
		Status:         download.Status,
		ChargedCredits: download.ChargedCredits,
		CreatedAt:      millis(download.CreatedAt),
		AuthorizedAt:   millisPointer(download.AuthorizedAt),
	}
}

func saleToPB(sale domain.AttachmentSale) *pb.AttachmentSale {
	return &pb.AttachmentSale{
		Attachment:    toPB(sale.Attachment),
		EarnedCredits: sale.EarnedCredits,
		SoldAt:        millis(sale.SoldAt),
	}
}

func millis(value time.Time) int64 {
	if value.IsZero() {
		return 0
	}
	return value.UnixMilli()
}

func millisPointer(value *time.Time) int64 {
	if value == nil {
		return 0
	}
	return value.UnixMilli()
}

func toStatus(err error) error {
	switch {
	case errors.Is(err, domain.ErrAttachmentNotFound):
		return status.Error(codes.NotFound, err.Error())
	case errors.Is(err, domain.ErrFileNotFound):
		return status.Error(codes.NotFound, err.Error())
	case errors.Is(err, domain.ErrAttachmentOwnerMismatch):
		return status.Error(codes.PermissionDenied, err.Error())
	case errors.Is(err, domain.ErrFileOwnerMismatch):
		return status.Error(codes.PermissionDenied, err.Error())
	case errors.Is(err, domain.ErrMembershipEntitlementRequired):
		return status.Error(codes.PermissionDenied, err.Error())
	case errors.Is(err, domain.ErrAttachmentTopicOwnerMismatch):
		return status.Error(codes.PermissionDenied, err.Error())
	case errors.Is(err, domain.ErrAttachmentArchived),
		errors.Is(err, domain.ErrFileDeleted),
		errors.Is(err, domain.ErrAttachmentTopicUnavailable),
		errors.Is(err, domain.ErrInsufficientCredits),
		errors.Is(err, domain.ErrPaidAttachmentSalesMembershipInactive),
		errors.Is(err, domain.ErrDownloadRecordMismatch),
		errors.Is(err, domain.ErrAccountErased):
		return status.Error(codes.FailedPrecondition, err.Error())
	case errors.Is(err, domain.ErrAttachmentObjectKeyTaken), errors.Is(err, domain.ErrFileObjectKeyTaken):
		return status.Error(codes.AlreadyExists, err.Error())
	case errors.Is(err, domain.ErrInvalidAttachment),
		errors.Is(err, domain.ErrInvalidFile),
		errors.Is(err, domain.ErrInvalidDownload),
		errors.Is(err, domain.ErrInvalidAccountErasure):
		return status.Error(codes.InvalidArgument, err.Error())
	case errors.Is(err, domain.ErrCreditServiceUnavailable),
		errors.Is(err, domain.ErrMembershipServiceUnavailable),
		errors.Is(err, domain.ErrContentServiceUnavailable),
		errors.Is(err, domain.ErrAccountErasureUnavailable),
		errors.Is(err, domain.ErrFileStorageUnavailable):
		return status.Error(codes.Unavailable, err.Error())
	default:
		return status.Error(codes.Internal, "file service request failed")
	}
}
