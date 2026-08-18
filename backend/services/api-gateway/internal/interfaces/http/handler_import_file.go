package http

import (
	"context"
	"errors"
	"io"
	stdhttp "net/http"
	"strconv"
	"strings"

	"api-gateway/api/proto/filepb"
	"api-gateway/internal/storage"

	"github.com/gin-gonic/gin"
)

type importFileRequest struct {
	FileID string `json:"fileId"`
}

func bindImportFileID(c *gin.Context) (int64, bool) {
	var request importFileRequest
	if !bindJSON(c, &request) {
		return 0, false
	}
	return parseImportFileID(c, request.FileID)
}

func parseImportFileID(c *gin.Context, value string) (int64, bool) {
	fileID, err := strconv.ParseInt(strings.TrimSpace(value), 10, 64)
	if err != nil || fileID <= 0 {
		writeError(c, stdhttp.StatusBadRequest, "fileId must be a positive integer", "bad_request")
		return 0, false
	}
	return fileID, true
}

func (h *Handler) readOwnedImportFile(c *gin.Context, ctx context.Context, ownerID, fileID, maxBytes int64) ([]byte, bool) {
	fileResponse, err := h.clients.File.GetFile(ctx, &filepb.GetFileRequest{OwnerId: ownerID, FileId: fileID})
	if err != nil {
		writeRPCError(c, err)
		return nil, false
	}
	file := fileResponse.GetFile()
	if file == nil || file.GetOwnerId() != ownerID || file.GetObjectKey() == "" {
		writeError(c, stdhttp.StatusNotFound, "import file not found", "not_found")
		return nil, false
	}
	if file.GetSizeBytes() <= 0 {
		writeError(c, stdhttp.StatusBadRequest, "import file is empty", "empty_file")
		return nil, false
	}
	if file.GetSizeBytes() > maxBytes {
		writeError(c, stdhttp.StatusRequestEntityTooLarge, "import file is too large", "file_too_large")
		return nil, false
	}

	object, _, err := h.attachments.Open(ctx, file.GetObjectKey())
	if err != nil {
		if errors.Is(err, storage.ErrObjectNotFound) {
			writeError(c, stdhttp.StatusNotFound, "import file object not found", "not_found")
		} else {
			writeError(c, stdhttp.StatusBadGateway, "import file object unavailable", "storage_unavailable")
		}
		return nil, false
	}
	payload, readErr := io.ReadAll(io.LimitReader(object, maxBytes+1))
	_ = object.Close()
	if readErr != nil {
		writeError(c, stdhttp.StatusBadGateway, "read import file failed", "storage_unavailable")
		return nil, false
	}
	if len(payload) == 0 {
		writeError(c, stdhttp.StatusBadRequest, "import file is empty", "empty_file")
		return nil, false
	}
	if int64(len(payload)) > maxBytes {
		writeError(c, stdhttp.StatusRequestEntityTooLarge, "import file is too large", "file_too_large")
		return nil, false
	}
	return payload, true
}
