package http

import (
	"bytes"
	"context"
	"errors"
	"io"
	"mime"
	stdhttp "net/http"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"api-gateway/api/proto/filepb"
	"api-gateway/internal/storage"
	"api-gateway/pkg/http/response"

	"github.com/gin-gonic/gin"
)

const (
	maxFileSize        = int64(50 << 20)
	maxFileRequestSize = maxFileSize + int64(1<<20)
	fileTransferTime   = 2 * time.Minute
)

func (h *Handler) uploadFile(c *gin.Context) {
	ownerID := currentUserID(c)
	if !h.allowFileUploadRateLimit(c, ownerID) {
		return
	}
	if !h.hasFileClient(c) || !h.hasAttachmentStore(c) {
		return
	}
	c.Request.Body = stdhttp.MaxBytesReader(c.Writer, c.Request.Body, maxFileRequestSize)
	fileHeader, err := c.FormFile("file")
	if err != nil {
		writeError(c, stdhttp.StatusBadRequest, "missing file", "bad_request")
		return
	}
	if fileHeader.Size <= 0 || fileHeader.Size > maxFileSize {
		writeError(c, stdhttp.StatusBadRequest, "file size must be between 1 byte and 50 MiB", "bad_request")
		return
	}
	filename, ok := safeAttachmentFilename(fileHeader.Filename)
	if !ok {
		writeError(c, stdhttp.StatusBadRequest, "invalid file name", "bad_request")
		return
	}
	reader, err := fileHeader.Open()
	if err != nil {
		writeError(c, stdhttp.StatusBadRequest, "open file failed", "bad_request")
		return
	}
	defer reader.Close()
	head := make([]byte, 512)
	n, readErr := io.ReadFull(reader, head)
	if readErr != nil && !errors.Is(readErr, io.EOF) && !errors.Is(readErr, io.ErrUnexpectedEOF) {
		writeError(c, stdhttp.StatusBadRequest, "read file failed", "bad_request")
		return
	}
	contentType := genericFileContentType(fileHeader.Header.Get("Content-Type"), head[:n])
	objectName, err := uploadedAvatarName(strings.ToLower(filepath.Ext(filename)))
	if err != nil {
		writeError(c, stdhttp.StatusInternalServerError, "create file name failed", "internal_error")
		return
	}
	objectKey := "files/" + strconv.FormatInt(ownerID, 10) + "/" + objectName
	transferCtx, transferCancel := context.WithTimeout(c.Request.Context(), fileTransferTime)
	err = h.attachments.Upload(transferCtx, objectKey, io.MultiReader(bytes.NewReader(head[:n]), reader), fileHeader.Size, contentType)
	transferCancel()
	if err != nil {
		writeError(c, stdhttp.StatusBadGateway, "store file failed", "storage_unavailable")
		return
	}
	ctx, cancel := rpcContext(c)
	defer cancel()
	created, err := h.clients.File.CreateFile(ctx, &filepb.CreateFileRequest{
		OwnerId:      ownerID,
		BizType:      normalizedFileBizType(c.PostForm("biz_type")),
		ObjectKey:    objectKey,
		OriginalName: filename,
		ContentType:  contentType,
		SizeBytes:    fileHeader.Size,
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
	if created.GetFile() == nil {
		cleanupCtx, cleanupCancel := context.WithTimeout(context.Background(), requestTimeout)
		_ = h.attachments.Delete(cleanupCtx, objectKey)
		cleanupCancel()
		writeError(c, stdhttp.StatusBadGateway, "file metadata unavailable", "service_unavailable")
		return
	}
	response.Success(c, h.filePayload(c, created.GetFile()))
}

func (h *Handler) listFiles(c *gin.Context) {
	if !h.hasFileClient(c) {
		return
	}
	ctx, cancel := rpcContext(c)
	defer cancel()
	result, err := h.clients.File.ListFiles(ctx, &filepb.ListFilesRequest{
		OwnerId: currentUserID(c),
		Limit:   queryInt32(c, "limit", 20),
		Offset:  queryInt32(c, "offset", 0),
	})
	if err != nil {
		writeRPCError(c, err)
		return
	}
	items := make([]gin.H, 0, len(result.GetItems()))
	for _, item := range result.GetItems() {
		items = append(items, h.filePayload(c, item))
	}
	response.Success(c, gin.H{"items": items, "total": result.GetTotal()})
}

func (h *Handler) getFileUsage(c *gin.Context) {
	if !h.hasFileClient(c) {
		return
	}
	ctx, cancel := rpcContext(c)
	defer cancel()
	result, err := h.clients.File.GetFileUsage(ctx, &filepb.GetFileUsageRequest{OwnerId: currentUserID(c)})
	if err != nil {
		writeRPCError(c, err)
		return
	}
	response.Success(c, gin.H{
		"used_bytes":      result.GetUsedBytes(),
		"capacity_bytes":  result.GetCapacityBytes(),
		"remaining_bytes": result.GetRemainingBytes(),
	})
}

func (h *Handler) getFile(c *gin.Context) {
	fileID, ok := pathInt64(c, "id")
	if !ok || !h.hasFileClient(c) {
		return
	}
	ctx, cancel := rpcContext(c)
	defer cancel()
	result, err := h.clients.File.GetFile(ctx, &filepb.GetFileRequest{OwnerId: currentUserID(c), FileId: fileID})
	if err != nil {
		writeRPCError(c, err)
		return
	}
	response.Success(c, h.filePayload(c, result.GetFile()))
}

func (h *Handler) downloadFile(c *gin.Context) {
	fileID, ok := pathInt64(c, "id")
	if !ok || !h.hasFileClient(c) || !h.hasAttachmentStore(c) {
		return
	}
	lookupCtx, lookupCancel := rpcContext(c)
	result, err := h.clients.File.GetFile(lookupCtx, &filepb.GetFileRequest{OwnerId: currentUserID(c), FileId: fileID})
	lookupCancel()
	if err != nil {
		writeRPCError(c, err)
		return
	}
	item := result.GetFile()
	if item == nil || item.GetOwnerId() != currentUserID(c) {
		writeError(c, stdhttp.StatusNotFound, "file not found", "not_found")
		return
	}
	transferCtx, transferCancel := context.WithTimeout(c.Request.Context(), fileTransferTime)
	defer transferCancel()
	object, info, err := h.attachments.Open(transferCtx, item.GetObjectKey())
	if err != nil {
		if errors.Is(err, storage.ErrObjectNotFound) {
			writeError(c, stdhttp.StatusNotFound, "file object not found", "not_found")
		} else {
			writeError(c, stdhttp.StatusBadGateway, "file object unavailable", "storage_unavailable")
		}
		return
	}
	defer object.Close()
	contentType := item.GetContentType()
	if contentType == "" {
		contentType = info.ContentType
	}
	if contentType == "" {
		contentType = "application/octet-stream"
	}
	c.Header("Content-Type", contentType)
	c.Header("Content-Disposition", mime.FormatMediaType("attachment", map[string]string{"filename": item.GetOriginalName()}))
	c.Header("Content-Length", strconv.FormatInt(info.Size, 10))
	c.Status(stdhttp.StatusOK)
	_, _ = io.Copy(c.Writer, object)
}

func (h *Handler) deleteFile(c *gin.Context) {
	fileID, ok := pathInt64(c, "id")
	if !ok || !h.hasFileClient(c) {
		return
	}
	ctx, cancel := rpcContext(c)
	defer cancel()
	result, err := h.clients.File.DeleteFile(ctx, &filepb.DeleteFileRequest{OwnerId: currentUserID(c), FileId: fileID})
	if err != nil {
		writeRPCError(c, err)
		return
	}
	response.Success(c, h.filePayload(c, result.GetFile()))
}

func (h *Handler) filePayload(c *gin.Context, item *filepb.File) gin.H {
	if item == nil {
		return gin.H{}
	}
	id := strconv.FormatInt(item.GetId(), 10)
	return gin.H{
		"id":            id,
		"owner_id":      strconv.FormatInt(item.GetOwnerId(), 10),
		"biz_type":      item.GetBizType(),
		"original_name": item.GetOriginalName(),
		"content_type":  item.GetContentType(),
		"size_bytes":    item.GetSizeBytes(),
		"status":        item.GetStatus(),
		"created_at":    item.GetCreatedAt(),
		"updated_at":    item.GetUpdatedAt(),
		"deleted_at":    item.GetDeletedAt(),
		"url":           h.publicURL(c, "/api/v1/files/"+id+"/download"),
		"download_url":  h.publicURL(c, "/api/v1/files/"+id+"/download"),
	}
}

func genericFileContentType(header string, head []byte) string {
	contentType, _, err := mime.ParseMediaType(header)
	if err == nil && strings.TrimSpace(contentType) != "" && contentType != "application/octet-stream" {
		return contentType
	}
	if len(head) > 0 {
		return stdhttp.DetectContentType(head)
	}
	if err == nil && strings.TrimSpace(contentType) != "" {
		return contentType
	}
	return "application/octet-stream"
}

func normalizedFileBizType(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return "files"
	}
	if len(value) > 64 || strings.ContainsAny(value, "/\\") || strings.ContainsRune(value, '\x00') {
		return "files"
	}
	return value
}

func (h *Handler) registerUploadedImage(c *gin.Context, folder, originalName, contentType string, size int64, objectKey string) (int64, bool) {
	ownerID := currentUserID(c)
	if ownerID <= 0 || strings.HasPrefix(c.Request.URL.Path, "/api/v1/admin/") || h == nil || h.clients == nil || h.clients.File == nil {
		return 0, true
	}
	ctx, cancel := rpcContext(c)
	defer cancel()
	created, err := h.clients.File.CreateFile(ctx, &filepb.CreateFileRequest{
		OwnerId:      ownerID,
		BizType:      normalizedFileBizType(folder),
		ObjectKey:    objectKey,
		OriginalName: originalName,
		ContentType:  contentType,
		SizeBytes:    size,
	})
	if err != nil {
		if canDeleteUploadedAttachmentAfterCreateError(err) {
			cleanupCtx, cleanupCancel := context.WithTimeout(context.Background(), requestTimeout)
			_ = h.attachments.Delete(cleanupCtx, objectKey)
			cleanupCancel()
		}
		writeRPCError(c, err)
		return 0, false
	}
	if created.GetFile() == nil {
		cleanupCtx, cleanupCancel := context.WithTimeout(context.Background(), requestTimeout)
		_ = h.attachments.Delete(cleanupCtx, objectKey)
		cleanupCancel()
		writeError(c, stdhttp.StatusBadGateway, "file metadata unavailable", "service_unavailable")
		return 0, false
	}
	return created.GetFile().GetId(), true
}
