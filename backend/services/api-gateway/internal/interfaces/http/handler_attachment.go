package http

import (
	"context"
	"io"
	"mime"
	stdhttp "net/http"
	"path"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"api-gateway/api/proto/contentpb"
	"api-gateway/api/proto/filepb"
	"api-gateway/pkg/http/response"

	"github.com/gin-gonic/gin"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

const (
	maxAttachmentSize        = int64(50 << 20)
	maxAttachmentRequestSize = maxAttachmentSize + int64(1<<20)
	attachmentTransferTime   = 2 * time.Minute
)

func (h *Handler) listTopicAttachments(c *gin.Context) {
	topicID, ok := pathInt64(c, "id")
	if !ok {
		return
	}
	if !h.hasFileClient(c) {
		return
	}
	ctx, cancel := rpcContext(c)
	defer cancel()
	topicResp, err := h.clients.Content.GetTopic(ctx, &contentpb.GetTopicRequest{Key: &contentpb.GetTopicRequest_Id{Id: topicID}})
	if err != nil {
		writeRPCError(c, err)
		return
	}
	if topicResp.GetTopic() == nil || topicResp.GetTopic().GetStatus() != contentStatusPublished {
		writeError(c, stdhttp.StatusNotFound, "topic not found", "not_found")
		return
	}
	attachments, err := h.clients.File.ListTopicAttachments(ctx, &filepb.ListTopicAttachmentsRequest{TopicId: topicID})
	if err != nil {
		writeRPCError(c, err)
		return
	}
	response.Success(c, gin.H{"items": attachmentPayloads(attachments.GetItems())})
}

func (h *Handler) listUserAttachmentDownloads(c *gin.Context) {
	if !h.hasFileClient(c) {
		return
	}
	ctx, cancel := rpcContext(c)
	defer cancel()
	downloads, err := h.clients.File.ListUserAttachmentDownloads(ctx, &filepb.ListUserAttachmentDownloadsRequest{
		UserId: currentUserID(c),
		Limit:  queryInt32(c, "limit", 20),
		Offset: queryInt32(c, "offset", 0),
	})
	if err != nil {
		writeRPCError(c, err)
		return
	}
	response.Success(c, gin.H{"items": attachmentDownloadPayloads(downloads.GetItems())})
}

func (h *Handler) uploadTopicAttachment(c *gin.Context) {
	topicID, ok := pathInt64(c, "id")
	if !ok {
		return
	}
	if !h.hasFileClient(c) || !h.hasAttachmentStore(c) {
		return
	}
	ownerCtx, ownerCancel := rpcContext(c)
	defer ownerCancel()
	topic, ok := h.requireTopicOwner(c, ownerCtx, topicID)
	if !ok {
		return
	}
	if topic.GetStatus() != contentStatusPublished {
		writeError(c, stdhttp.StatusPreconditionFailed, "topic must be published before uploading attachments", "failed_precondition")
		return
	}
	if !h.ensureCurrentUserCanPost(c, ownerCtx) {
		return
	}

	c.Request.Body = stdhttp.MaxBytesReader(c.Writer, c.Request.Body, maxAttachmentRequestSize)
	priceCredits, ok := attachmentPriceCredits(c)
	if !ok {
		return
	}
	if priceCredits > 0 && !h.ensureCurrentUserHasMembershipPaidAttachmentEntitlement(c, ownerCtx) {
		return
	}
	fileHeader, err := c.FormFile("file")
	if err != nil {
		writeError(c, stdhttp.StatusBadRequest, "missing attachment file", "bad_request")
		return
	}
	if fileHeader.Size <= 0 || fileHeader.Size > maxAttachmentSize {
		writeError(c, stdhttp.StatusBadRequest, "attachment file size must be between 1 byte and 50 MiB", "bad_request")
		return
	}
	filename, ok := safeAttachmentFilename(fileHeader.Filename)
	if !ok {
		writeError(c, stdhttp.StatusBadRequest, "invalid attachment filename", "bad_request")
		return
	}
	reader, err := fileHeader.Open()
	if err != nil {
		writeError(c, stdhttp.StatusBadRequest, "open attachment file failed", "bad_request")
		return
	}
	defer reader.Close()
	objectName, err := uploadedAvatarName(strings.ToLower(filepath.Ext(filename)))
	if err != nil {
		writeError(c, stdhttp.StatusInternalServerError, "create attachment name failed", "internal_error")
		return
	}
	objectKey := "topics/" + strconv.FormatInt(topicID, 10) + "/" + objectName
	contentType := attachmentContentType(fileHeader.Header.Get("Content-Type"))
	transferCtx, transferCancel := context.WithTimeout(c.Request.Context(), attachmentTransferTime)
	err = h.attachments.Upload(transferCtx, objectKey, reader, fileHeader.Size, contentType)
	transferCancel()
	if err != nil {
		writeError(c, stdhttp.StatusBadGateway, "store attachment failed", "storage_unavailable")
		return
	}

	ctx, cancel := rpcContext(c)
	defer cancel()
	created, err := h.clients.File.CreateAttachment(ctx, &filepb.CreateAttachmentRequest{
		TopicId:      topicID,
		OwnerId:      currentUserID(c),
		ObjectKey:    objectKey,
		OriginalName: filename,
		ContentType:  contentType,
		SizeBytes:    fileHeader.Size,
		PriceCredits: priceCredits,
	})
	if err != nil {
		if canDeleteUploadedAttachmentAfterCreateError(err) {
			cleanupCtx, cleanupCancel := context.WithTimeout(context.Background(), requestTimeout)
			_ = h.attachments.Delete(cleanupCtx, objectKey)
			cleanupCancel()
		}
		writeRPCError(c, err)
		return
	}
	response.Success(c, attachmentPayload(created.GetAttachment()))
}

func (h *Handler) downloadTopicAttachment(c *gin.Context) {
	attachmentID, ok := pathInt64(c, "id")
	if !ok {
		return
	}
	if !h.hasFileClient(c) || !h.hasAttachmentStore(c) {
		return
	}
	lookupCtx, lookupCancel := rpcContext(c)
	attachmentResp, err := h.clients.File.GetAttachment(lookupCtx, &filepb.GetAttachmentRequest{AttachmentId: attachmentID})
	lookupCancel()
	if err != nil {
		writeRPCError(c, err)
		return
	}
	attachment := attachmentResp.GetAttachment()
	if attachment == nil {
		writeError(c, stdhttp.StatusNotFound, "attachment not found", "not_found")
		return
	}
	topicCtx, topicCancel := rpcContext(c)
	topicResp, err := h.clients.Content.GetTopic(topicCtx, &contentpb.GetTopicRequest{Key: &contentpb.GetTopicRequest_Id{Id: attachment.GetTopicId()}})
	topicCancel()
	if err != nil {
		writeRPCError(c, err)
		return
	}
	if topicResp.GetTopic() == nil || topicResp.GetTopic().GetStatus() != contentStatusPublished {
		writeError(c, stdhttp.StatusNotFound, "topic not found", "not_found")
		return
	}
	transferCtx, transferCancel := context.WithTimeout(c.Request.Context(), attachmentTransferTime)
	defer transferCancel()
	object, objectInfo, err := h.attachments.Open(transferCtx, attachment.GetObjectKey())
	if err != nil {
		writeError(c, stdhttp.StatusBadGateway, "attachment object unavailable", "storage_unavailable")
		return
	}
	defer object.Close()
	if objectInfo.Size != attachment.GetSizeBytes() {
		writeError(c, stdhttp.StatusBadGateway, "attachment object is invalid", "storage_unavailable")
		return
	}

	authorizationCtx, authorizationCancel := rpcContext(c)
	defer authorizationCancel()
	authorization, err := h.clients.File.AuthorizeAttachmentDownload(authorizationCtx, &filepb.AuthorizeAttachmentDownloadRequest{
		AttachmentId: attachmentID,
		UserId:       currentUserID(c),
	})
	if err != nil {
		writeRPCError(c, err)
		return
	}
	if authorization.GetAttachment() == nil {
		writeError(c, stdhttp.StatusNotFound, "attachment not found", "not_found")
		return
	}
	contentType := attachment.GetContentType()
	if contentType == "" {
		contentType = objectInfo.ContentType
	}
	if contentType == "" {
		contentType = "application/octet-stream"
	}
	c.Header("Content-Type", contentType)
	c.Header("Content-Disposition", mime.FormatMediaType("attachment", map[string]string{"filename": attachment.GetOriginalName()}))
	c.Header("Content-Length", strconv.FormatInt(objectInfo.Size, 10))
	c.Status(stdhttp.StatusOK)
	_, _ = io.Copy(c.Writer, object)
}

func (h *Handler) archiveTopicAttachment(c *gin.Context) {
	attachmentID, ok := pathInt64(c, "id")
	if !ok {
		return
	}
	if !h.hasFileClient(c) {
		return
	}
	ctx, cancel := rpcContext(c)
	defer cancel()
	archived, err := h.clients.File.ArchiveAttachment(ctx, &filepb.ArchiveAttachmentRequest{
		AttachmentId: attachmentID,
		OwnerId:      currentUserID(c),
	})
	if err != nil {
		writeRPCError(c, err)
		return
	}
	response.Success(c, attachmentPayload(archived.GetAttachment()))
}

func (h *Handler) updateTopicAttachmentPrice(c *gin.Context) {
	attachmentID, ok := pathInt64(c, "id")
	if !ok {
		return
	}
	if !h.hasFileClient(c) {
		return
	}
	var request updateAttachmentPriceRequest
	if !bindJSON(c, &request) {
		return
	}
	if request.PriceCredits == nil || request.PriceCredits.Int64() < 0 {
		writeError(c, stdhttp.StatusBadRequest, "price_credits must be a non-negative integer", "bad_request")
		return
	}
	ctx, cancel := rpcContext(c)
	defer cancel()
	attachmentResp, err := h.clients.File.GetAttachment(ctx, &filepb.GetAttachmentRequest{AttachmentId: attachmentID})
	if err != nil {
		writeRPCError(c, err)
		return
	}
	attachment := attachmentResp.GetAttachment()
	if attachment == nil {
		writeError(c, stdhttp.StatusNotFound, "attachment not found", "not_found")
		return
	}
	topic, ok := h.requireTopicOwner(c, ctx, attachment.GetTopicId())
	if !ok {
		return
	}
	if topic.GetStatus() != contentStatusPublished {
		writeError(c, stdhttp.StatusPreconditionFailed, "topic must be published before updating attachment price", "failed_precondition")
		return
	}
	if !h.ensureCurrentUserCanPost(c, ctx) {
		return
	}
	if request.PriceCredits.Int64() > 0 && !h.ensureCurrentUserHasMembershipPaidAttachmentEntitlement(c, ctx) {
		return
	}
	updated, err := h.clients.File.UpdateAttachmentPrice(ctx, &filepb.UpdateAttachmentPriceRequest{
		AttachmentId: attachmentID,
		OwnerId:      currentUserID(c),
		PriceCredits: request.PriceCredits.Int64(),
	})
	if err != nil {
		writeRPCError(c, err)
		return
	}
	response.Success(c, attachmentPayload(updated.GetAttachment()))
}

func (h *Handler) hasFileClient(c *gin.Context) bool {
	if h == nil || h.clients == nil || h.clients.File == nil {
		writeError(c, stdhttp.StatusServiceUnavailable, "file service unavailable", "service_unavailable")
		return false
	}
	return true
}

func (h *Handler) hasAttachmentStore(c *gin.Context) bool {
	if h == nil || h.attachments == nil {
		writeError(c, stdhttp.StatusServiceUnavailable, "attachment storage unavailable", "storage_unavailable")
		return false
	}
	return true
}

func attachmentPriceCredits(c *gin.Context) (int64, bool) {
	value := strings.TrimSpace(c.PostForm("price_credits"))
	if value == "" {
		return 0, true
	}
	price, err := strconv.ParseInt(value, 10, 64)
	if err != nil || price < 0 {
		writeError(c, stdhttp.StatusBadRequest, "price_credits must be a non-negative integer", "bad_request")
		return 0, false
	}
	return price, true
}

func safeAttachmentFilename(value string) (string, bool) {
	name := path.Base(strings.ReplaceAll(strings.TrimSpace(value), "\\", "/"))
	if name == "" || name == "." || name == ".." || name == "/" || len(name) > 255 || strings.ContainsAny(name, "\x00\r\n") {
		return "", false
	}
	return name, true
}

func attachmentContentType(value string) string {
	contentType, _, err := mime.ParseMediaType(value)
	if err != nil || strings.TrimSpace(contentType) == "" {
		return "application/octet-stream"
	}
	return contentType
}

func canDeleteUploadedAttachmentAfterCreateError(err error) bool {
	switch status.Code(err) {
	case codes.InvalidArgument, codes.NotFound, codes.AlreadyExists, codes.PermissionDenied, codes.FailedPrecondition:
		return true
	default:
		return false
	}
}

func attachmentPayload(attachment *filepb.Attachment) gin.H {
	if attachment == nil {
		return nil
	}
	return gin.H{
		"id":            attachment.GetId(),
		"topic_id":      attachment.GetTopicId(),
		"original_name": attachment.GetOriginalName(),
		"content_type":  attachment.GetContentType(),
		"size_bytes":    attachment.GetSizeBytes(),
		"price_credits": attachment.GetPriceCredits(),
		"status":        attachment.GetStatus(),
		"created_at":    attachment.GetCreatedAt(),
	}
}

func attachmentPayloads(attachments []*filepb.Attachment) []gin.H {
	items := make([]gin.H, 0, len(attachments))
	for _, attachment := range attachments {
		items = append(items, attachmentPayload(attachment))
	}
	return items
}

func attachmentDownloadPayload(download *filepb.AttachmentDownload) gin.H {
	if download == nil {
		return nil
	}
	return gin.H{
		"attachment":      attachmentPayload(download.GetAttachment()),
		"status":          download.GetStatus(),
		"charged_credits": download.GetChargedCredits(),
		"created_at":      download.GetCreatedAt(),
		"authorized_at":   download.GetAuthorizedAt(),
	}
}

func attachmentDownloadPayloads(downloads []*filepb.AttachmentDownload) []gin.H {
	items := make([]gin.H, 0, len(downloads))
	for _, download := range downloads {
		if payload := attachmentDownloadPayload(download); payload != nil {
			items = append(items, payload)
		}
	}
	return items
}
