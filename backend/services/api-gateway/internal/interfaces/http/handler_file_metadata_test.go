package http

import (
	"bytes"
	"context"
	"encoding/json"
	"mime/multipart"
	stdhttp "net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"api-gateway/api/proto/filepb"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
)

func TestFileFolderRoutesForwardAuthenticatedOwnerAndReturnSnakeCase(t *testing.T) {
	gin.SetMode(gin.TestMode)
	client := &fakeFileMetadataClient{
		listFoldersResp: &filepb.FolderListResponse{Items: []*filepb.Folder{{
			Id: 9007199254740993, OwnerId: 42, Name: "Projects", ParentId: 9007199254740995,
			CreatedAt: 11, UpdatedAt: 12, FoldersCount: 2, FilesCount: 3,
		}}, Total: 1},
		createFolderResp: &filepb.FolderResponse{Folder: &filepb.Folder{Id: 51, OwnerId: 42, Name: "Projects"}},
		updateFolderResp: &filepb.FolderResponse{Folder: &filepb.Folder{Id: 51, OwnerId: 42, Name: "Renamed"}},
	}
	router := newUserFileRouter(client, newFakeUserFileStore())
	token := signedAuthToken(t, jwt.MapClaims{"sub": "42"})

	list := authenticatedRequest(t, router, stdhttp.MethodGet, "/api/v1/file-folders?parent_id=9007199254740995&limit=7&offset=11&search_query=%20docs%20", "", token)
	require.Equal(t, stdhttp.StatusOK, list.Code, list.Body.String())
	require.Equal(t, int64(42), client.listFoldersReq.GetOwnerId())
	require.Equal(t, int64(9007199254740995), client.listFoldersReq.GetParentId())
	require.Equal(t, int32(7), client.listFoldersReq.GetLimit())
	require.Equal(t, int32(11), client.listFoldersReq.GetOffset())
	require.Equal(t, "docs", client.listFoldersReq.GetSearchQuery())
	var listEnvelope struct {
		Data struct {
			Items []map[string]any `json:"items"`
		} `json:"data"`
	}
	require.NoError(t, json.Unmarshal(list.Body.Bytes(), &listEnvelope))
	require.Len(t, listEnvelope.Data.Items, 1)
	require.Equal(t, "9007199254740993", listEnvelope.Data.Items[0]["id"])
	require.Equal(t, "9007199254740995", listEnvelope.Data.Items[0]["parent_id"])
	require.Equal(t, float64(2), listEnvelope.Data.Items[0]["folders_count"])
	require.NotContains(t, listEnvelope.Data.Items[0], "parentId")

	created := authenticatedRequest(t, router, stdhttp.MethodPost, "/api/v1/file-folders", `{"owner_id":"7","name":"  Projects  ","parent_id":"9007199254740995"}`, token)
	require.Equal(t, stdhttp.StatusOK, created.Code, created.Body.String())
	require.Equal(t, int64(42), client.createFolderReq.GetOwnerId())
	require.Equal(t, "Projects", client.createFolderReq.GetName())
	require.Equal(t, int64(9007199254740995), client.createFolderReq.GetParentId())

	updated := authenticatedRequest(t, router, stdhttp.MethodPut, "/api/v1/file-folders/51", `{"owner_id":"7","name":"  Renamed  ","parent_id":null}`, token)
	require.Equal(t, stdhttp.StatusOK, updated.Code, updated.Body.String())
	require.Equal(t, int64(42), client.updateFolderReq.GetOwnerId())
	require.Equal(t, int64(51), client.updateFolderReq.GetFolderId())
	require.NotNil(t, client.updateFolderReq.Name)
	require.Equal(t, "Renamed", client.updateFolderReq.GetName())
	require.NotNil(t, client.updateFolderReq.ParentId)
	require.Zero(t, client.updateFolderReq.GetParentId())

	deleted := authenticatedRequest(t, router, stdhttp.MethodDelete, "/api/v1/file-folders/51", "", token)
	require.Equal(t, stdhttp.StatusNoContent, deleted.Code, deleted.Body.String())
	require.Empty(t, deleted.Body.String())
	require.Equal(t, int64(42), client.deleteFolderReq.GetOwnerId())
	require.Equal(t, int64(51), client.deleteFolderReq.GetFolderId())
}

func TestListFilesAndUploadPreserveFolderSelectionSemantics(t *testing.T) {
	gin.SetMode(gin.TestMode)
	client := &fakeFileMetadataClient{}
	store := newFakeUserFileStore()
	router := newUserFileRouter(client, store)
	token := signedAuthToken(t, jwt.MapClaims{"sub": "42"})

	all := authenticatedRequest(t, router, stdhttp.MethodGet, "/api/v1/files", "", token)
	require.Equal(t, stdhttp.StatusOK, all.Code, all.Body.String())
	require.Nil(t, client.listFilesReq.FolderId)

	root := authenticatedRequest(t, router, stdhttp.MethodGet, "/api/v1/files?folder_id=0", "", token)
	require.Equal(t, stdhttp.StatusOK, root.Code, root.Body.String())
	require.NotNil(t, client.listFilesReq.FolderId)
	require.Zero(t, client.listFilesReq.GetFolderId())

	nested := authenticatedRequest(t, router, stdhttp.MethodGet, "/api/v1/files?folder_id=9007199254740993", "", token)
	require.Equal(t, stdhttp.StatusOK, nested.Code, nested.Body.String())
	require.Equal(t, int64(9007199254740993), client.listFilesReq.GetFolderId())
	require.Equal(t, int64(42), client.listFilesReq.GetOwnerId())

	uploadRequest := fileUploadRequestWithFolder(t, "/api/v1/files", "report.txt", []byte("report"), "documents", "9007199254740993")
	uploadRequest.Header.Set("Authorization", "Bearer "+token)
	upload := httptest.NewRecorder()
	router.ServeHTTP(upload, uploadRequest)
	require.Equal(t, stdhttp.StatusOK, upload.Code, upload.Body.String())
	require.Equal(t, int64(42), client.createFileReq.GetOwnerId())
	require.Equal(t, int64(9007199254740993), client.createFileReq.GetFolderId())
}

func TestUpdateFileDistinguishesOmittedAndNullFolder(t *testing.T) {
	gin.SetMode(gin.TestMode)
	client := &fakeFileMetadataClient{updateFileResp: &filepb.FileResponse{File: &filepb.File{
		Id: 71, OwnerId: 42, OriginalName: "report.txt", FolderId: 0, IsSensitive: false, Comment: "",
	}}}
	router := newUserFileRouter(client, newFakeUserFileStore())
	token := signedAuthToken(t, jwt.MapClaims{"sub": "42"})

	metadataOnly := authenticatedRequest(t, router, stdhttp.MethodPatch, "/api/v1/files/71", `{"owner_id":"7","is_sensitive":false,"comment":""}`, token)
	require.Equal(t, stdhttp.StatusOK, metadataOnly.Code, metadataOnly.Body.String())
	require.Equal(t, int64(42), client.updateFileReq.GetOwnerId())
	require.Equal(t, int64(71), client.updateFileReq.GetFileId())
	require.Nil(t, client.updateFileReq.Name)
	require.Nil(t, client.updateFileReq.FolderId)
	require.NotNil(t, client.updateFileReq.IsSensitive)
	require.False(t, client.updateFileReq.GetIsSensitive())
	require.NotNil(t, client.updateFileReq.Comment)
	require.Empty(t, client.updateFileReq.GetComment())

	movedToRoot := authenticatedRequest(t, router, stdhttp.MethodPatch, "/api/v1/files/71", `{"folder_id":null}`, token)
	require.Equal(t, stdhttp.StatusOK, movedToRoot.Code, movedToRoot.Body.String())
	require.NotNil(t, client.updateFileReq.FolderId)
	require.Zero(t, client.updateFileReq.GetFolderId())
	var envelope struct {
		Data map[string]any `json:"data"`
	}
	require.NoError(t, json.Unmarshal(movedToRoot.Body.Bytes(), &envelope))
	require.Contains(t, envelope.Data, "folder_id")
	require.Nil(t, envelope.Data["folder_id"])
	require.Contains(t, envelope.Data, "is_sensitive")
	require.NotContains(t, envelope.Data, "folderId")
}

func TestFileMetadataRoutesRequireAuthentication(t *testing.T) {
	gin.SetMode(gin.TestMode)
	client := &fakeFileMetadataClient{}
	router := newUserFileRouter(client, newFakeUserFileStore())
	tests := []struct {
		method string
		path   string
		body   string
	}{
		{stdhttp.MethodGet, "/api/v1/file-folders", ""},
		{stdhttp.MethodPost, "/api/v1/file-folders", `{"name":"private"}`},
		{stdhttp.MethodPut, "/api/v1/file-folders/1", `{"name":"private"}`},
		{stdhttp.MethodDelete, "/api/v1/file-folders/1", ""},
		{stdhttp.MethodPatch, "/api/v1/files/1", `{"name":"private"}`},
	}
	for _, tt := range tests {
		t.Run(tt.method+" "+tt.path, func(t *testing.T) {
			recorder := authenticatedRequest(t, router, tt.method, tt.path, tt.body, "")
			require.Equal(t, stdhttp.StatusUnauthorized, recorder.Code, recorder.Body.String())
		})
	}
	require.Zero(t, client.calls)
}

func TestFileMetadataValidationRejectsInvalidRequestsBeforeRPC(t *testing.T) {
	gin.SetMode(gin.TestMode)
	tests := []struct {
		name   string
		method string
		path   string
		body   string
	}{
		{"invalid file folder filter", stdhttp.MethodGet, "/api/v1/files?folder_id=-1", ""},
		{"invalid folder parent filter", stdhttp.MethodGet, "/api/v1/file-folders?parent_id=invalid", ""},
		{"empty create name", stdhttp.MethodPost, "/api/v1/file-folders", `{"name":"  "}`},
		{"negative create parent", stdhttp.MethodPost, "/api/v1/file-folders", `{"name":"docs","parent_id":-1}`},
		{"empty create parent", stdhttp.MethodPost, "/api/v1/file-folders", `{"name":"docs","parent_id":""}`},
		{"empty folder update", stdhttp.MethodPut, "/api/v1/file-folders/1", `{}`},
		{"blank folder update name", stdhttp.MethodPut, "/api/v1/file-folders/1", `{"name":"  "}`},
		{"folder name is a path", stdhttp.MethodPut, "/api/v1/file-folders/1", `{"name":"nested/path"}`},
		{"empty file update", stdhttp.MethodPatch, "/api/v1/files/1", `{}`},
		{"blank file update name", stdhttp.MethodPatch, "/api/v1/files/1", `{"name":"  "}`},
		{"file name is a path", stdhttp.MethodPatch, "/api/v1/files/1", `{"name":"nested\\path"}`},
		{"file comment too long", stdhttp.MethodPatch, "/api/v1/files/1", `{"comment":"` + strings.Repeat("x", 513) + `"}`},
		{"negative file folder", stdhttp.MethodPatch, "/api/v1/files/1", `{"folder_id":-1}`},
		{"invalid folder path", stdhttp.MethodDelete, "/api/v1/file-folders/0", ""},
		{"invalid file path", stdhttp.MethodPatch, "/api/v1/files/not-an-id", `{"name":"valid"}`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client := &fakeFileMetadataClient{}
			router := newUserFileRouter(client, newFakeUserFileStore())
			token := signedAuthToken(t, jwt.MapClaims{"sub": "42"})
			recorder := authenticatedRequest(t, router, tt.method, tt.path, tt.body, token)
			require.Equal(t, stdhttp.StatusBadRequest, recorder.Code, recorder.Body.String())
			require.Zero(t, client.calls)
		})
	}
}

func authenticatedRequest(t *testing.T, router *gin.Engine, method string, target string, body string, token string) *httptest.ResponseRecorder {
	t.Helper()
	request := httptest.NewRequest(method, target, strings.NewReader(body))
	if body != "" {
		request.Header.Set("Content-Type", "application/json")
	}
	if token != "" {
		request.Header.Set("Authorization", "Bearer "+token)
	}
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, request)
	return recorder
}

func fileUploadRequestWithFolder(t *testing.T, target string, filename string, content []byte, bizType string, folderID string) *stdhttp.Request {
	t.Helper()
	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	require.NoError(t, writer.WriteField("owner_id", "7"))
	require.NoError(t, writer.WriteField("biz_type", bizType))
	require.NoError(t, writer.WriteField("folder_id", folderID))
	part, err := writer.CreateFormFile("file", filename)
	require.NoError(t, err)
	_, err = part.Write(content)
	require.NoError(t, err)
	require.NoError(t, writer.Close())
	request := httptest.NewRequest(stdhttp.MethodPost, target, &body)
	request.Header.Set("Content-Type", writer.FormDataContentType())
	return request
}

type fakeFileMetadataClient struct {
	filepb.FileServiceClient
	calls            int
	listFoldersReq   *filepb.ListFoldersRequest
	createFolderReq  *filepb.CreateFolderRequest
	updateFolderReq  *filepb.UpdateFolderRequest
	deleteFolderReq  *filepb.DeleteFolderRequest
	listFilesReq     *filepb.ListFilesRequest
	createFileReq    *filepb.CreateFileRequest
	updateFileReq    *filepb.UpdateFileRequest
	listFoldersResp  *filepb.FolderListResponse
	createFolderResp *filepb.FolderResponse
	updateFolderResp *filepb.FolderResponse
	updateFileResp   *filepb.FileResponse
}

func (f *fakeFileMetadataClient) ListFolders(_ context.Context, request *filepb.ListFoldersRequest, _ ...grpc.CallOption) (*filepb.FolderListResponse, error) {
	f.calls++
	f.listFoldersReq = request
	if f.listFoldersResp != nil {
		return f.listFoldersResp, nil
	}
	return &filepb.FolderListResponse{}, nil
}

func (f *fakeFileMetadataClient) CreateFolder(_ context.Context, request *filepb.CreateFolderRequest, _ ...grpc.CallOption) (*filepb.FolderResponse, error) {
	f.calls++
	f.createFolderReq = request
	if f.createFolderResp != nil {
		return f.createFolderResp, nil
	}
	return &filepb.FolderResponse{Folder: &filepb.Folder{Id: 1, OwnerId: request.GetOwnerId(), Name: request.GetName(), ParentId: request.GetParentId()}}, nil
}

func (f *fakeFileMetadataClient) UpdateFolder(_ context.Context, request *filepb.UpdateFolderRequest, _ ...grpc.CallOption) (*filepb.FolderResponse, error) {
	f.calls++
	f.updateFolderReq = request
	if f.updateFolderResp != nil {
		return f.updateFolderResp, nil
	}
	return &filepb.FolderResponse{Folder: &filepb.Folder{Id: request.GetFolderId(), OwnerId: request.GetOwnerId(), Name: request.GetName(), ParentId: request.GetParentId()}}, nil
}

func (f *fakeFileMetadataClient) DeleteFolder(_ context.Context, request *filepb.DeleteFolderRequest, _ ...grpc.CallOption) (*filepb.FolderResponse, error) {
	f.calls++
	f.deleteFolderReq = request
	return &filepb.FolderResponse{}, nil
}

func (f *fakeFileMetadataClient) ListFiles(_ context.Context, request *filepb.ListFilesRequest, _ ...grpc.CallOption) (*filepb.FileListResponse, error) {
	f.calls++
	f.listFilesReq = request
	return &filepb.FileListResponse{}, nil
}

func (f *fakeFileMetadataClient) CreateFile(_ context.Context, request *filepb.CreateFileRequest, _ ...grpc.CallOption) (*filepb.FileResponse, error) {
	f.calls++
	f.createFileReq = request
	return &filepb.FileResponse{File: &filepb.File{
		Id: request.GetFolderId() + 1, OwnerId: request.GetOwnerId(), BizType: request.GetBizType(),
		ObjectKey: request.GetObjectKey(), OriginalName: request.GetOriginalName(), ContentType: request.GetContentType(),
		SizeBytes: request.GetSizeBytes(), Status: "ACTIVE", FolderId: request.GetFolderId(),
	}}, nil
}

func (f *fakeFileMetadataClient) UpdateFile(_ context.Context, request *filepb.UpdateFileRequest, _ ...grpc.CallOption) (*filepb.FileResponse, error) {
	f.calls++
	f.updateFileReq = request
	if f.updateFileResp != nil {
		return f.updateFileResp, nil
	}
	return &filepb.FileResponse{File: &filepb.File{
		Id: request.GetFileId(), OwnerId: request.GetOwnerId(), OriginalName: request.GetName(),
		FolderId: request.GetFolderId(), IsSensitive: request.GetIsSensitive(), Comment: request.GetComment(),
	}}, nil
}
