package http

import (
	"bytes"
	"encoding/json"
	"fmt"
	stdhttp "net/http"
	"strconv"
	"strings"

	"api-gateway/api/proto/filepb"
	"api-gateway/pkg/http/response"

	"github.com/gin-gonic/gin"
)

const (
	maxFileFolderNameLength = 200
	maxFileMetadataName     = 255
	maxFileMetadataComment  = 512
)

type optionalRootID struct {
	Present bool
	Value   int64
}

func (v *optionalRootID) UnmarshalJSON(data []byte) error {
	v.Present = true
	trimmed := bytes.TrimSpace(data)
	if bytes.Equal(trimmed, []byte("null")) {
		v.Value = 0
		return nil
	}
	text := string(trimmed)
	if strings.HasPrefix(text, `"`) {
		var raw string
		if err := json.Unmarshal(trimmed, &raw); err != nil {
			return err
		}
		text = strings.TrimSpace(raw)
	}
	value, err := strconv.ParseInt(text, 10, 64)
	if err != nil || value < 0 {
		return fmt.Errorf("folder id must be a non-negative integer")
	}
	v.Value = value
	return nil
}

type createFileFolderRequest struct {
	Name     string         `json:"name"`
	ParentID optionalRootID `json:"parent_id"`
}

type updateFileFolderRequest struct {
	Name     *string        `json:"name"`
	ParentID optionalRootID `json:"parent_id"`
}

type updateFileRequest struct {
	Name        *string        `json:"name"`
	FolderID    optionalRootID `json:"folder_id"`
	IsSensitive *bool          `json:"is_sensitive"`
	Comment     *string        `json:"comment"`
}

func (h *Handler) listFileFolders(c *gin.Context) {
	if !h.hasFileClient(c) {
		return
	}
	parentID, ok := queryRootID(c, "parent_id", 0)
	if !ok {
		return
	}
	searchQuery := strings.TrimSpace(c.Query("search_query"))
	if len(searchQuery) > maxFileFolderNameLength || strings.ContainsRune(searchQuery, '\x00') {
		writeError(c, stdhttp.StatusBadRequest, "invalid search_query", "bad_request")
		return
	}
	ctx, cancel := rpcContext(c)
	defer cancel()
	result, err := h.clients.File.ListFolders(ctx, &filepb.ListFoldersRequest{
		OwnerId:     currentUserID(c),
		ParentId:    parentID,
		Limit:       queryInt32(c, "limit", 20),
		Offset:      queryInt32(c, "offset", 0),
		SearchQuery: searchQuery,
	})
	if err != nil {
		writeRPCError(c, err)
		return
	}
	if result == nil {
		writeError(c, stdhttp.StatusBadGateway, "folder metadata unavailable", "service_unavailable")
		return
	}
	items := make([]gin.H, 0, len(result.GetItems()))
	for _, item := range result.GetItems() {
		items = append(items, folderPayload(item))
	}
	response.Success(c, gin.H{"items": items, "total": result.GetTotal()})
}

func (h *Handler) createFileFolder(c *gin.Context) {
	if !h.hasFileClient(c) {
		return
	}
	var request createFileFolderRequest
	if !bindJSON(c, &request) {
		return
	}
	request.Name = strings.TrimSpace(request.Name)
	if !validFileEntryName(request.Name, maxFileFolderNameLength) {
		writeError(c, stdhttp.StatusBadRequest, "invalid folder name", "bad_request")
		return
	}
	ctx, cancel := rpcContext(c)
	defer cancel()
	result, err := h.clients.File.CreateFolder(ctx, &filepb.CreateFolderRequest{
		OwnerId:  currentUserID(c),
		Name:     request.Name,
		ParentId: request.ParentID.Value,
	})
	if err != nil {
		writeRPCError(c, err)
		return
	}
	if result.GetFolder() == nil {
		writeError(c, stdhttp.StatusBadGateway, "folder metadata unavailable", "service_unavailable")
		return
	}
	response.Success(c, folderPayload(result.GetFolder()))
}

func (h *Handler) updateFileFolder(c *gin.Context) {
	folderID, ok := pathInt64(c, "id")
	if !ok || !h.hasFileClient(c) {
		return
	}
	var request updateFileFolderRequest
	if !bindJSON(c, &request) {
		return
	}
	if request.Name == nil && !request.ParentID.Present {
		writeError(c, stdhttp.StatusBadRequest, "at least one folder field is required", "bad_request")
		return
	}
	if request.Name != nil {
		name := strings.TrimSpace(*request.Name)
		if !validFileEntryName(name, maxFileFolderNameLength) {
			writeError(c, stdhttp.StatusBadRequest, "invalid folder name", "bad_request")
			return
		}
		request.Name = &name
	}
	rpcRequest := &filepb.UpdateFolderRequest{
		OwnerId:  currentUserID(c),
		FolderId: folderID,
		Name:     request.Name,
	}
	if request.ParentID.Present {
		rpcRequest.ParentId = &request.ParentID.Value
	}
	ctx, cancel := rpcContext(c)
	defer cancel()
	result, err := h.clients.File.UpdateFolder(ctx, rpcRequest)
	if err != nil {
		writeRPCError(c, err)
		return
	}
	if result.GetFolder() == nil {
		writeError(c, stdhttp.StatusBadGateway, "folder metadata unavailable", "service_unavailable")
		return
	}
	response.Success(c, folderPayload(result.GetFolder()))
}

func (h *Handler) deleteFileFolder(c *gin.Context) {
	folderID, ok := pathInt64(c, "id")
	if !ok || !h.hasFileClient(c) {
		return
	}
	ctx, cancel := rpcContext(c)
	defer cancel()
	_, err := h.clients.File.DeleteFolder(ctx, &filepb.DeleteFolderRequest{
		OwnerId:  currentUserID(c),
		FolderId: folderID,
	})
	if err != nil {
		writeRPCError(c, err)
		return
	}
	c.Status(stdhttp.StatusNoContent)
}

func (h *Handler) updateFile(c *gin.Context) {
	fileID, ok := pathInt64(c, "id")
	if !ok || !h.hasFileClient(c) {
		return
	}
	var request updateFileRequest
	if !bindJSON(c, &request) {
		return
	}
	if request.Name == nil && !request.FolderID.Present && request.IsSensitive == nil && request.Comment == nil {
		writeError(c, stdhttp.StatusBadRequest, "at least one file field is required", "bad_request")
		return
	}
	if request.Name != nil {
		name := strings.TrimSpace(*request.Name)
		if !validFileEntryName(name, maxFileMetadataName) {
			writeError(c, stdhttp.StatusBadRequest, "invalid file name", "bad_request")
			return
		}
		request.Name = &name
	}
	if request.Comment != nil && (len(*request.Comment) > maxFileMetadataComment || strings.ContainsRune(*request.Comment, '\x00')) {
		writeError(c, stdhttp.StatusBadRequest, "invalid file comment", "bad_request")
		return
	}
	rpcRequest := &filepb.UpdateFileRequest{
		OwnerId:     currentUserID(c),
		FileId:      fileID,
		Name:        request.Name,
		IsSensitive: request.IsSensitive,
		Comment:     request.Comment,
	}
	if request.FolderID.Present {
		rpcRequest.FolderId = &request.FolderID.Value
	}
	ctx, cancel := rpcContext(c)
	defer cancel()
	result, err := h.clients.File.UpdateFile(ctx, rpcRequest)
	if err != nil {
		writeRPCError(c, err)
		return
	}
	if result.GetFile() == nil {
		writeError(c, stdhttp.StatusBadGateway, "file metadata unavailable", "service_unavailable")
		return
	}
	response.Success(c, h.filePayload(c, result.GetFile()))
}

func folderPayload(item *filepb.Folder) gin.H {
	if item == nil {
		return gin.H{}
	}
	return gin.H{
		"id":            strconv.FormatInt(item.GetId(), 10),
		"owner_id":      strconv.FormatInt(item.GetOwnerId(), 10),
		"name":          item.GetName(),
		"parent_id":     nullableEntityID(item.GetParentId()),
		"created_at":    item.GetCreatedAt(),
		"updated_at":    item.GetUpdatedAt(),
		"folders_count": item.GetFoldersCount(),
		"files_count":   item.GetFilesCount(),
	}
}

func queryOptionalRootID(c *gin.Context, name string) (*int64, bool) {
	raw, exists := c.GetQuery(name)
	if !exists {
		return nil, true
	}
	value, ok := parseRootID(c, name, raw)
	if !ok {
		return nil, false
	}
	return &value, true
}

func queryRootID(c *gin.Context, name string, fallback int64) (int64, bool) {
	raw, exists := c.GetQuery(name)
	if !exists {
		return fallback, true
	}
	return parseRootID(c, name, raw)
}

func multipartRootID(c *gin.Context, name string) (int64, bool) {
	raw := strings.TrimSpace(c.PostForm(name))
	if raw == "" {
		return 0, true
	}
	return parseRootID(c, name, raw)
}

func parseRootID(c *gin.Context, name string, raw string) (int64, bool) {
	value, err := strconv.ParseInt(strings.TrimSpace(raw), 10, 64)
	if err != nil || value < 0 {
		writeError(c, stdhttp.StatusBadRequest, name+" must be a non-negative integer", "bad_request")
		return 0, false
	}
	return value, true
}

func nullableEntityID(id int64) any {
	if id <= 0 {
		return nil
	}
	return strconv.FormatInt(id, 10)
}

func validFileEntryName(name string, maxLength int) bool {
	return name != "" && len(name) <= maxLength && !strings.ContainsAny(name, "/\\") && !strings.ContainsRune(name, '\x00')
}
