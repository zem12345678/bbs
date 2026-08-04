package http

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"mime/multipart"
	stdhttp "net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"api-gateway/api/proto/adminpb"
	"api-gateway/api/proto/filepb"
	"api-gateway/internal/clients"
	"api-gateway/internal/storage"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

const (
	largeFileID = int64(9007199254740993)
	largeUserID = int64(9007199254740995)
)

func TestUploadFileUsesAuthenticatedUserAndReturnsStringIDs(t *testing.T) {
	gin.SetMode(gin.TestMode)
	fileClient := &fakeUserFileClient{createResp: &filepb.FileResponse{File: &filepb.File{
		Id: largeFileID, OwnerId: largeUserID, BizType: "documents", OriginalName: "report.txt", Status: "ACTIVE",
	}}}
	store := newFakeUserFileStore()
	router := newUserFileRouter(fileClient, store)

	req := userFileUploadRequest(t, "/api/v1/files", "report.txt", []byte("quarterly report"), "documents")
	req.Header.Set("Authorization", "Bearer "+signedAuthToken(t, jwt.MapClaims{"sub": "9007199254740995", "username": "alice"}))
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, req)

	require.Equal(t, stdhttp.StatusOK, recorder.Code, recorder.Body.String())
	require.NotNil(t, fileClient.createReq)
	require.Equal(t, largeUserID, fileClient.createReq.GetOwnerId())
	require.Equal(t, "documents", fileClient.createReq.GetBizType())
	require.Equal(t, "report.txt", fileClient.createReq.GetOriginalName())
	require.EqualValues(t, len("quarterly report"), fileClient.createReq.GetSizeBytes())
	require.True(t, strings.HasPrefix(fileClient.createReq.GetObjectKey(), "files/9007199254740995/"))
	require.Equal(t, []byte("quarterly report"), store.objects[fileClient.createReq.GetObjectKey()].data)

	var envelope struct {
		Data struct {
			ID      string `json:"id"`
			OwnerID string `json:"owner_id"`
		} `json:"data"`
	}
	require.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &envelope))
	require.Equal(t, "9007199254740993", envelope.Data.ID)
	require.Equal(t, "9007199254740995", envelope.Data.OwnerID)
}

func TestUploadFileCleansObjectOnlyAfterDefinitiveMetadataRejection(t *testing.T) {
	tests := []struct {
		name           string
		createErr      error
		wantStatus     int
		wantObjectGone bool
	}{
		{
			name:           "invalid argument",
			createErr:      status.Error(codes.InvalidArgument, "invalid metadata"),
			wantStatus:     stdhttp.StatusBadRequest,
			wantObjectGone: true,
		},
		{
			name:           "capacity exhausted",
			createErr:      status.Error(codes.ResourceExhausted, "file capacity exhausted"),
			wantStatus:     stdhttp.StatusTooManyRequests,
			wantObjectGone: true,
		},
		{
			name:       "deadline exceeded",
			createErr:  status.Error(codes.DeadlineExceeded, "file service deadline exceeded"),
			wantStatus: stdhttp.StatusGatewayTimeout,
		},
		{
			name:       "service unavailable",
			createErr:  status.Error(codes.Unavailable, "file service unavailable"),
			wantStatus: stdhttp.StatusServiceUnavailable,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gin.SetMode(gin.TestMode)
			client := &fakeUserFileClient{createErr: tt.createErr}
			store := newFakeUserFileStore()
			router := newUserFileRouter(client, store)
			req := userFileUploadRequest(t, "/api/v1/files", "report.txt", []byte("data"), "documents")
			req.Header.Set("Authorization", "Bearer "+signedAuthToken(t, jwt.MapClaims{"sub": "42"}))
			recorder := httptest.NewRecorder()

			router.ServeHTTP(recorder, req)

			require.Equal(t, tt.wantStatus, recorder.Code, recorder.Body.String())
			require.NotNil(t, client.createReq)
			objectKey := client.createReq.GetObjectKey()
			if tt.wantObjectGone {
				require.Equal(t, []string{objectKey}, store.deletedKeys)
				require.NotContains(t, store.objects, objectKey)
			} else {
				require.Empty(t, store.deletedKeys)
				require.Contains(t, store.objects, objectKey)
			}
		})
	}
}

func TestUploadFileCleansObjectWhenMetadataResponseIsEmpty(t *testing.T) {
	gin.SetMode(gin.TestMode)
	client := &fakeUserFileClient{createResp: &filepb.FileResponse{}}
	store := newFakeUserFileStore()
	router := newUserFileRouter(client, store)
	req := userFileUploadRequest(t, "/api/v1/files", "report.txt", []byte("data"), "documents")
	req.Header.Set("Authorization", "Bearer "+signedAuthToken(t, jwt.MapClaims{"sub": "42"}))
	recorder := httptest.NewRecorder()

	router.ServeHTTP(recorder, req)

	require.Equal(t, stdhttp.StatusBadGateway, recorder.Code, recorder.Body.String())
	require.NotNil(t, client.createReq)
	require.Equal(t, []string{client.createReq.GetObjectKey()}, store.deletedKeys)
	require.NotContains(t, store.objects, client.createReq.GetObjectKey())
}

func TestFileRoutesForwardAuthenticatedOwnerAndPreserveIDs(t *testing.T) {
	gin.SetMode(gin.TestMode)
	item := &filepb.File{Id: largeFileID, OwnerId: 42, BizType: "files", OriginalName: "archive.zip", Status: "ACTIVE"}
	fileClient := &fakeUserFileClient{
		listResp:   &filepb.FileListResponse{Items: []*filepb.File{item}, Total: 1},
		getResp:    &filepb.FileResponse{File: item},
		deleteResp: &filepb.FileResponse{File: &filepb.File{Id: largeFileID, OwnerId: 42, Status: "DELETED"}},
	}
	router := newUserFileRouter(fileClient, newFakeUserFileStore())
	token := signedAuthToken(t, jwt.MapClaims{"sub": "42", "username": "alice"})

	listReq := httptest.NewRequest(stdhttp.MethodGet, "/api/v1/files?limit=7&offset=11", nil)
	listReq.Header.Set("Authorization", "Bearer "+token)
	listRecorder := httptest.NewRecorder()
	router.ServeHTTP(listRecorder, listReq)
	require.Equal(t, stdhttp.StatusOK, listRecorder.Code, listRecorder.Body.String())
	require.Equal(t, int64(42), fileClient.listReq.GetOwnerId())
	require.Equal(t, int32(7), fileClient.listReq.GetLimit())
	require.Equal(t, int32(11), fileClient.listReq.GetOffset())
	var listEnvelope struct {
		Data struct {
			Items []struct {
				ID string `json:"id"`
			} `json:"items"`
		} `json:"data"`
	}
	require.NoError(t, json.Unmarshal(listRecorder.Body.Bytes(), &listEnvelope))
	require.Len(t, listEnvelope.Data.Items, 1)
	require.Equal(t, "9007199254740993", listEnvelope.Data.Items[0].ID)

	getReq := httptest.NewRequest(stdhttp.MethodGet, "/api/v1/files/9007199254740993", nil)
	getReq.Header.Set("Authorization", "Bearer "+token)
	getRecorder := httptest.NewRecorder()
	router.ServeHTTP(getRecorder, getReq)
	require.Equal(t, stdhttp.StatusOK, getRecorder.Code, getRecorder.Body.String())
	require.Equal(t, int64(42), fileClient.getReq.GetOwnerId())
	require.Equal(t, largeFileID, fileClient.getReq.GetFileId())

	deleteReq := httptest.NewRequest(stdhttp.MethodDelete, "/api/v1/files/9007199254740993", nil)
	deleteReq.Header.Set("Authorization", "Bearer "+token)
	deleteRecorder := httptest.NewRecorder()
	router.ServeHTTP(deleteRecorder, deleteReq)
	require.Equal(t, stdhttp.StatusOK, deleteRecorder.Code, deleteRecorder.Body.String())
	require.Equal(t, int64(42), fileClient.deleteReq.GetOwnerId())
	require.Equal(t, largeFileID, fileClient.deleteReq.GetFileId())
}

func TestDownloadFileRejectsMetadataOwnedByAnotherUser(t *testing.T) {
	gin.SetMode(gin.TestMode)
	fileClient := &fakeUserFileClient{getResp: &filepb.FileResponse{File: &filepb.File{
		Id: 88, OwnerId: 7, ObjectKey: "files/7/private.txt", OriginalName: "private.txt", ContentType: "text/plain",
	}}}
	store := newFakeUserFileStore()
	store.objects["files/7/private.txt"] = fakeUserFileObject{data: []byte("private"), contentType: "text/plain"}
	router := newUserFileRouter(fileClient, store)
	req := httptest.NewRequest(stdhttp.MethodGet, "/api/v1/files/88/download", nil)
	req.Header.Set("Authorization", "Bearer "+signedAuthToken(t, jwt.MapClaims{"sub": "42"}))
	recorder := httptest.NewRecorder()

	router.ServeHTTP(recorder, req)

	require.Equal(t, stdhttp.StatusNotFound, recorder.Code, recorder.Body.String())
	require.Equal(t, int64(42), fileClient.getReq.GetOwnerId())
	require.Equal(t, int64(88), fileClient.getReq.GetFileId())
	require.Empty(t, store.openKeys)
	require.NotContains(t, recorder.Body.String(), "private")
}

func TestUploadImageRegistersUserFileMetadata(t *testing.T) {
	gin.SetMode(gin.TestMode)
	fileClient := &fakeUserFileClient{createResp: &filepb.FileResponse{File: &filepb.File{
		Id: largeFileID, OwnerId: 42, BizType: "images", Status: "ACTIVE",
	}}}
	store := newFakeUserFileStore()
	router := newUserFileRouter(fileClient, store)

	recorder := performImageUpload(t, router, "/api/v1/uploads/images", "inline.png")

	require.Equal(t, stdhttp.StatusOK, recorder.Code, recorder.Body.String())
	require.NotNil(t, fileClient.createReq)
	require.Equal(t, int64(42), fileClient.createReq.GetOwnerId())
	require.Equal(t, "images", fileClient.createReq.GetBizType())
	require.Equal(t, "inline.png", fileClient.createReq.GetOriginalName())
	require.Equal(t, "image/png", fileClient.createReq.GetContentType())
	require.EqualValues(t, len(testPNGImage), fileClient.createReq.GetSizeBytes())
	require.True(t, strings.HasPrefix(fileClient.createReq.GetObjectKey(), "uploads/images/"))
	var envelope struct {
		Data map[string]string `json:"data"`
	}
	require.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &envelope))
	require.Equal(t, "9007199254740993", envelope.Data["file_id"])
	require.Equal(t, "http://example.test/api/v1/files/9007199254740993/download", envelope.Data["file_url"])
}

func TestUploadImageCleansObjectOnlyAfterDefinitiveMetadataRejection(t *testing.T) {
	tests := []struct {
		name           string
		createErr      error
		wantStatus     int
		wantObjectGone bool
	}{
		{
			name:           "invalid argument",
			createErr:      status.Error(codes.InvalidArgument, "invalid metadata"),
			wantStatus:     stdhttp.StatusBadRequest,
			wantObjectGone: true,
		},
		{
			name:           "capacity exhausted",
			createErr:      status.Error(codes.ResourceExhausted, "file capacity exhausted"),
			wantStatus:     stdhttp.StatusTooManyRequests,
			wantObjectGone: true,
		},
		{
			name:       "deadline exceeded",
			createErr:  status.Error(codes.DeadlineExceeded, "file service deadline exceeded"),
			wantStatus: stdhttp.StatusGatewayTimeout,
		},
		{
			name:       "service unavailable",
			createErr:  status.Error(codes.Unavailable, "file service unavailable"),
			wantStatus: stdhttp.StatusServiceUnavailable,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gin.SetMode(gin.TestMode)
			client := &fakeUserFileClient{createErr: tt.createErr}
			store := newFakeUserFileStore()
			router := newUserFileRouter(client, store)

			recorder := performImageUpload(t, router, "/api/v1/uploads/images", "inline.png")

			require.Equal(t, tt.wantStatus, recorder.Code, recorder.Body.String())
			require.NotNil(t, client.createReq)
			objectKey := client.createReq.GetObjectKey()
			if tt.wantObjectGone {
				require.Equal(t, []string{objectKey}, store.deletedKeys)
				require.NotContains(t, store.objects, objectKey)
			} else {
				require.Empty(t, store.deletedKeys)
				require.Contains(t, store.objects, objectKey)
			}
		})
	}
}

func TestUploadAdminAvatarDoesNotRegisterUserFile(t *testing.T) {
	gin.SetMode(gin.TestMode)
	fileClient := &fakeUserFileClient{}
	store := newFakeUserFileStore()
	h := NewHandlerWithAttachmentStore(&clients.Clients{Admin: &fakeFileAdminClient{}, File: fileClient}, "Authorization", "Bearer", testJWTSecret, store)
	h.SetPublicBaseURL("http://example.test")
	router := gin.New()
	NewInitControllers(h)(router)

	recorder := performImageUpload(t, router, "/api/v1/admin/uploads/avatar", "admin.png")

	require.Equal(t, stdhttp.StatusOK, recorder.Code, recorder.Body.String())
	require.Nil(t, fileClient.createReq)
	require.Len(t, store.objects, 1)
}

func TestNormalizedFileBizType(t *testing.T) {
	tests := map[string]string{
		"":                      "files",
		"documents":             "documents",
		"x0":                    "x0",
		"nested/path":           "files",
		`nested\path`:           "files",
		"contains\x00nul":       "files",
		strings.Repeat("a", 65): "files",
	}
	for input, expected := range tests {
		require.Equal(t, expected, normalizedFileBizType(input), input)
	}
}

func newUserFileRouter(fileClient filepb.FileServiceClient, store storage.ObjectStore) *gin.Engine {
	h := NewHandlerWithAttachmentStore(&clients.Clients{File: fileClient}, "Authorization", "Bearer", testJWTSecret, store)
	h.SetPublicBaseURL("http://example.test")
	router := gin.New()
	NewInitControllers(h)(router)
	return router
}

func userFileUploadRequest(t *testing.T, target, filename string, content []byte, bizType string) *stdhttp.Request {
	t.Helper()
	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	require.NoError(t, writer.WriteField("owner_id", "7"))
	require.NoError(t, writer.WriteField("biz_type", bizType))
	part, err := writer.CreateFormFile("file", filename)
	require.NoError(t, err)
	_, err = part.Write(content)
	require.NoError(t, err)
	require.NoError(t, writer.Close())
	req := httptest.NewRequest(stdhttp.MethodPost, target, &body)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	return req
}

type fakeUserFileClient struct {
	filepb.FileServiceClient
	createReq  *filepb.CreateFileRequest
	listReq    *filepb.ListFilesRequest
	getReq     *filepb.GetFileRequest
	deleteReq  *filepb.DeleteFileRequest
	createResp *filepb.FileResponse
	listResp   *filepb.FileListResponse
	getResp    *filepb.FileResponse
	deleteResp *filepb.FileResponse
	createErr  error
	getErr     error
}

func (f *fakeUserFileClient) CreateFile(_ context.Context, req *filepb.CreateFileRequest, _ ...grpc.CallOption) (*filepb.FileResponse, error) {
	f.createReq = req
	if f.createErr != nil {
		return nil, f.createErr
	}
	if f.createResp != nil {
		return f.createResp, nil
	}
	return &filepb.FileResponse{File: &filepb.File{
		Id:           1,
		OwnerId:      req.GetOwnerId(),
		BizType:      req.GetBizType(),
		ObjectKey:    req.GetObjectKey(),
		OriginalName: req.GetOriginalName(),
		ContentType:  req.GetContentType(),
		SizeBytes:    req.GetSizeBytes(),
		Status:       "ACTIVE",
	}}, nil
}

func (f *fakeUserFileClient) ListFiles(_ context.Context, req *filepb.ListFilesRequest, _ ...grpc.CallOption) (*filepb.FileListResponse, error) {
	f.listReq = req
	if f.listResp != nil {
		return f.listResp, nil
	}
	return &filepb.FileListResponse{}, nil
}

func (f *fakeUserFileClient) GetFile(_ context.Context, req *filepb.GetFileRequest, _ ...grpc.CallOption) (*filepb.FileResponse, error) {
	f.getReq = req
	if f.getErr != nil {
		return nil, f.getErr
	}
	if f.getResp != nil {
		return f.getResp, nil
	}
	return &filepb.FileResponse{File: &filepb.File{Id: req.GetFileId(), OwnerId: req.GetOwnerId(), Status: "ACTIVE"}}, nil
}

func (f *fakeUserFileClient) DeleteFile(_ context.Context, req *filepb.DeleteFileRequest, _ ...grpc.CallOption) (*filepb.FileResponse, error) {
	f.deleteReq = req
	if f.deleteResp != nil {
		return f.deleteResp, nil
	}
	return &filepb.FileResponse{File: &filepb.File{Id: req.GetFileId(), OwnerId: req.GetOwnerId(), Status: "DELETED"}}, nil
}

type fakeUserFileObject struct {
	data        []byte
	contentType string
}

type fakeUserFileStore struct {
	objects     map[string]fakeUserFileObject
	openKeys    []string
	deletedKeys []string
}

func newFakeUserFileStore() *fakeUserFileStore {
	return &fakeUserFileStore{objects: make(map[string]fakeUserFileObject)}
}

func (s *fakeUserFileStore) Upload(_ context.Context, key string, reader io.Reader, _ int64, contentType string) error {
	data, err := io.ReadAll(reader)
	if err != nil {
		return err
	}
	s.objects[key] = fakeUserFileObject{data: data, contentType: contentType}
	return nil
}

func (s *fakeUserFileStore) Open(_ context.Context, key string) (io.ReadCloser, storage.ObjectInfo, error) {
	s.openKeys = append(s.openKeys, key)
	object, ok := s.objects[key]
	if !ok {
		return nil, storage.ObjectInfo{}, storage.ErrObjectNotFound
	}
	return io.NopCloser(bytes.NewReader(object.data)), storage.ObjectInfo{Size: int64(len(object.data)), ContentType: object.contentType}, nil
}

func (s *fakeUserFileStore) Delete(_ context.Context, key string) error {
	s.deletedKeys = append(s.deletedKeys, key)
	delete(s.objects, key)
	return nil
}

var _ storage.ObjectStore = (*fakeUserFileStore)(nil)

type fakeFileAdminClient struct {
	adminpb.AdminServiceClient
}

func (*fakeFileAdminClient) GetProfile(context.Context, *adminpb.ProfileRequest, ...grpc.CallOption) (*adminpb.ProfileResponse, error) {
	return &adminpb.ProfileResponse{User: &adminpb.AdminUserInfo{Id: 1, Username: "admin"}}, nil
}

func (*fakeFileAdminClient) RecordOperationLog(context.Context, *adminpb.RecordOperationLogRequest, ...grpc.CallOption) (*adminpb.SimpleResponse, error) {
	return &adminpb.SimpleResponse{Success: true}, nil
}
