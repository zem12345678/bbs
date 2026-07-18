package http

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"mime/multipart"
	stdhttp "net/http"
	"net/http/httptest"
	"testing"

	"api-gateway/api/proto/adminpb"
	"api-gateway/api/proto/contentpb"
	"api-gateway/api/proto/filepb"
	"api-gateway/api/proto/userpb"
	"api-gateway/internal/clients"
	"api-gateway/internal/storage"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
)

func TestUploadTopicAttachmentStoresObjectAndHidesObjectKey(t *testing.T) {
	gin.SetMode(gin.TestMode)
	contentClient := &fakeTopicContentClient{getTopicResp: &contentpb.TopicResponse{Topic: &contentpb.TopicInfo{Id: 1001, AuthorId: 42, Status: contentStatusPublished}}}
	userClient := &fakeUserClient{userResponse: &userpb.UserResponse{User: &userpb.UserInfo{Id: 42, Status: userStatusActive}}}
	fileClient := &fakeAttachmentFileClient{}
	store := &fakeAttachmentStore{}
	h := NewHandlerWithAttachmentStore(&clients.Clients{Content: contentClient, User: userClient, File: fileClient}, "Authorization", "Bearer", testJWTSecret, store)
	router := gin.New()
	NewInitControllers(h)(router)

	req := attachmentUploadRequest(t, "/api/v1/topics/1001/attachments", "guide.pdf", "attachment bytes", "9")
	req.Header.Set("Authorization", "Bearer "+signedAuthToken(t, jwt.MapClaims{"sub": "42", "username": "alice"}))
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, req)

	require.Equal(t, stdhttp.StatusOK, recorder.Code, recorder.Body.String())
	require.NotNil(t, fileClient.createReq)
	require.EqualValues(t, 1001, fileClient.createReq.GetTopicId())
	require.EqualValues(t, 42, fileClient.createReq.GetOwnerId())
	require.Equal(t, "guide.pdf", fileClient.createReq.GetOriginalName())
	require.EqualValues(t, 9, fileClient.createReq.GetPriceCredits())
	require.Equal(t, []byte("attachment bytes"), store.uploaded)
	require.Equal(t, fileClient.createReq.GetObjectKey(), store.uploadKey)
	require.NotEmpty(t, store.uploadKey)
	require.NotContains(t, recorder.Body.String(), "object_key")

	var envelope struct {
		Data map[string]any `json:"data"`
	}
	require.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &envelope))
	require.Equal(t, "guide.pdf", envelope.Data["original_name"])
	require.Equal(t, float64(9), envelope.Data["price_credits"])
}

func TestUploadTopicAttachmentRejectsUnverifiedAuthorWhenEmailGateEnabled(t *testing.T) {
	gin.SetMode(gin.TestMode)
	contentClient := &fakeTopicContentClient{getTopicResp: &contentpb.TopicResponse{Topic: &contentpb.TopicInfo{Id: 1001, AuthorId: 42, Status: contentStatusPublished}}}
	userClient := &fakeUserClient{userResponse: &userpb.UserResponse{User: &userpb.UserInfo{Id: 42, Status: userStatusActive}}}
	fileClient := &fakeAttachmentFileClient{}
	store := &fakeAttachmentStore{}
	h := NewHandlerWithAttachmentStore(&clients.Clients{
		Content: contentClient,
		User:    userClient,
		File:    fileClient,
		Admin:   fakeAuthSettingsAdminClient{items: []*adminpb.SettingInfo{authSetting("auth.email_verification.required", "true")}},
	}, "Authorization", "Bearer", testJWTSecret, store)
	router := gin.New()
	NewInitControllers(h)(router)

	req := attachmentUploadRequest(t, "/api/v1/topics/1001/attachments", "guide.pdf", "attachment bytes", "9")
	req.Header.Set("Authorization", "Bearer "+signedAuthToken(t, jwt.MapClaims{"sub": "42", "username": "alice"}))
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, req)

	require.Equal(t, stdhttp.StatusForbidden, recorder.Code, recorder.Body.String())
	require.Contains(t, recorder.Body.String(), "email_not_verified")
	require.Nil(t, fileClient.createReq)
	require.Empty(t, store.uploaded)
}

func TestUploadTopicAttachmentRejectsNonOwnerBeforeStorage(t *testing.T) {
	gin.SetMode(gin.TestMode)
	contentClient := &fakeTopicContentClient{getTopicResp: &contentpb.TopicResponse{Topic: &contentpb.TopicInfo{Id: 1001, AuthorId: 7, Status: contentStatusPublished}}}
	fileClient := &fakeAttachmentFileClient{}
	store := &fakeAttachmentStore{}
	h := NewHandlerWithAttachmentStore(&clients.Clients{Content: contentClient, File: fileClient}, "Authorization", "Bearer", testJWTSecret, store)
	router := gin.New()
	NewInitControllers(h)(router)

	req := attachmentUploadRequest(t, "/api/v1/topics/1001/attachments", "guide.pdf", "attachment bytes", "0")
	req.Header.Set("Authorization", "Bearer "+signedAuthToken(t, jwt.MapClaims{"sub": "42", "username": "alice"}))
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, req)

	require.Equal(t, stdhttp.StatusForbidden, recorder.Code, recorder.Body.String())
	require.Nil(t, fileClient.createReq)
	require.Empty(t, store.uploaded)
}

func TestUploadTopicAttachmentRejectsUnpublishedTopicBeforeStorage(t *testing.T) {
	gin.SetMode(gin.TestMode)
	contentClient := &fakeTopicContentClient{getTopicResp: &contentpb.TopicResponse{Topic: &contentpb.TopicInfo{Id: 1001, AuthorId: 42, Status: 4}}}
	fileClient := &fakeAttachmentFileClient{}
	store := &fakeAttachmentStore{}
	h := NewHandlerWithAttachmentStore(&clients.Clients{Content: contentClient, File: fileClient}, "Authorization", "Bearer", testJWTSecret, store)
	router := gin.New()
	NewInitControllers(h)(router)

	req := attachmentUploadRequest(t, "/api/v1/topics/1001/attachments", "guide.pdf", "attachment bytes", "0")
	req.Header.Set("Authorization", "Bearer "+signedAuthToken(t, jwt.MapClaims{"sub": "42", "username": "alice"}))
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, req)

	require.Equal(t, stdhttp.StatusPreconditionFailed, recorder.Code, recorder.Body.String())
	require.Contains(t, recorder.Body.String(), "topic must be published before uploading attachments")
	require.Nil(t, fileClient.createReq)
	require.Empty(t, store.uploaded)
}

func TestDownloadTopicAttachmentPreflightsObjectBeforeAuthorization(t *testing.T) {
	gin.SetMode(gin.TestMode)
	attachment := &filepb.Attachment{Id: 88, TopicId: 1001, ObjectKey: "topics/1/guide.pdf", OriginalName: "guide.pdf", ContentType: "application/pdf", SizeBytes: 4, PriceCredits: 9, Status: "ACTIVE"}
	contentClient := &fakeTopicContentClient{getTopicResp: &contentpb.TopicResponse{Topic: &contentpb.TopicInfo{Id: 1001, Status: contentStatusPublished}}}
	fileClient := &fakeAttachmentFileClient{getResp: &filepb.AttachmentResponse{Attachment: attachment}, authorizeResp: &filepb.DownloadAuthorizationResponse{Attachment: attachment, ChargedCredits: 9}}
	store := &fakeAttachmentStore{openData: []byte("data"), openInfo: storage.ObjectInfo{Size: 4, ContentType: "application/pdf"}}
	h := NewHandlerWithAttachmentStore(&clients.Clients{Content: contentClient, File: fileClient}, "Authorization", "Bearer", testJWTSecret, store)
	router := gin.New()
	NewInitControllers(h)(router)

	req := httptest.NewRequest(stdhttp.MethodGet, "/api/v1/attachments/88/download", nil)
	req.Header.Set("Authorization", "Bearer "+signedAuthToken(t, jwt.MapClaims{"sub": "42", "username": "alice"}))
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, req)

	require.Equal(t, stdhttp.StatusOK, recorder.Code, recorder.Body.String())
	require.Equal(t, "data", recorder.Body.String())
	require.Equal(t, "application/pdf", recorder.Header().Get("Content-Type"))
	require.Contains(t, recorder.Header().Get("Content-Disposition"), "attachment")
	require.Equal(t, attachment.GetObjectKey(), store.openKey)
	require.NotNil(t, fileClient.authorizeReq)
	require.EqualValues(t, 88, fileClient.authorizeReq.GetAttachmentId())
	require.EqualValues(t, 42, fileClient.authorizeReq.GetUserId())
}

func TestDownloadTopicAttachmentDoesNotAuthorizeMissingObject(t *testing.T) {
	gin.SetMode(gin.TestMode)
	attachment := &filepb.Attachment{Id: 89, TopicId: 1001, ObjectKey: "topics/1/missing.pdf", OriginalName: "missing.pdf", ContentType: "application/pdf", SizeBytes: 4, Status: "ACTIVE"}
	contentClient := &fakeTopicContentClient{getTopicResp: &contentpb.TopicResponse{Topic: &contentpb.TopicInfo{Id: 1001, Status: contentStatusPublished}}}
	fileClient := &fakeAttachmentFileClient{getResp: &filepb.AttachmentResponse{Attachment: attachment}}
	store := &fakeAttachmentStore{openErr: errors.New("object not found")}
	h := NewHandlerWithAttachmentStore(&clients.Clients{Content: contentClient, File: fileClient}, "Authorization", "Bearer", testJWTSecret, store)
	router := gin.New()
	NewInitControllers(h)(router)

	req := httptest.NewRequest(stdhttp.MethodGet, "/api/v1/attachments/89/download", nil)
	req.Header.Set("Authorization", "Bearer "+signedAuthToken(t, jwt.MapClaims{"sub": "42", "username": "alice"}))
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, req)

	require.Equal(t, stdhttp.StatusBadGateway, recorder.Code, recorder.Body.String())
	require.Nil(t, fileClient.authorizeReq)
}

func TestDownloadTopicAttachmentRejectsArchivedTopicBeforeOpeningOrCharging(t *testing.T) {
	gin.SetMode(gin.TestMode)
	attachment := &filepb.Attachment{Id: 90, TopicId: 1001, ObjectKey: "topics/1/archived.pdf", OriginalName: "archived.pdf", ContentType: "application/pdf", SizeBytes: 4, PriceCredits: 9, Status: "ACTIVE"}
	contentClient := &fakeTopicContentClient{getTopicResp: &contentpb.TopicResponse{Topic: &contentpb.TopicInfo{Id: 1001, Status: 4}}}
	fileClient := &fakeAttachmentFileClient{getResp: &filepb.AttachmentResponse{Attachment: attachment}, authorizeResp: &filepb.DownloadAuthorizationResponse{Attachment: attachment, ChargedCredits: 9}}
	store := &fakeAttachmentStore{openData: []byte("data"), openInfo: storage.ObjectInfo{Size: 4, ContentType: "application/pdf"}}
	h := NewHandlerWithAttachmentStore(&clients.Clients{Content: contentClient, File: fileClient}, "Authorization", "Bearer", testJWTSecret, store)
	router := gin.New()
	NewInitControllers(h)(router)

	req := httptest.NewRequest(stdhttp.MethodGet, "/api/v1/attachments/90/download", nil)
	req.Header.Set("Authorization", "Bearer "+signedAuthToken(t, jwt.MapClaims{"sub": "42", "username": "alice"}))
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, req)

	require.Equal(t, stdhttp.StatusNotFound, recorder.Code, recorder.Body.String())
	require.Empty(t, store.openKey)
	require.Nil(t, fileClient.authorizeReq)
}

func TestListUserAttachmentDownloadsBindsCurrentUserAndHidesObjectKey(t *testing.T) {
	gin.SetMode(gin.TestMode)
	attachment := &filepb.Attachment{
		Id:           88,
		TopicId:      1001,
		ObjectKey:    "topics/1001/guide.pdf",
		OriginalName: "guide.pdf",
		ContentType:  "application/pdf",
		SizeBytes:    4,
		PriceCredits: 9,
		Status:       "ARCHIVED",
	}
	fileClient := &fakeAttachmentFileClient{listDownloadsResp: &filepb.AttachmentDownloadListResponse{Items: []*filepb.AttachmentDownload{{
		Attachment:     attachment,
		Status:         "AUTHORIZED",
		ChargedCredits: 9,
		CreatedAt:      100,
		AuthorizedAt:   200,
	}}}}
	h := NewHandler(&clients.Clients{File: fileClient}, "Authorization", "Bearer", testJWTSecret)
	router := gin.New()
	NewInitControllers(h)(router)

	req := httptest.NewRequest(stdhttp.MethodGet, "/api/v1/attachments/downloads?user_id=99&limit=5&offset=2", nil)
	req.Header.Set("Authorization", "Bearer "+signedAuthToken(t, jwt.MapClaims{"sub": "42", "username": "alice"}))
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, req)

	require.Equal(t, stdhttp.StatusOK, recorder.Code, recorder.Body.String())
	require.NotNil(t, fileClient.listDownloadsReq)
	require.EqualValues(t, 42, fileClient.listDownloadsReq.GetUserId())
	require.EqualValues(t, 5, fileClient.listDownloadsReq.GetLimit())
	require.EqualValues(t, 2, fileClient.listDownloadsReq.GetOffset())
	require.NotContains(t, recorder.Body.String(), "object_key")

	var envelope struct {
		Data struct {
			Items []struct {
				Status         string `json:"status"`
				ChargedCredits int64  `json:"charged_credits"`
				Attachment     struct {
					ID     int64  `json:"id"`
					Status string `json:"status"`
				} `json:"attachment"`
			} `json:"items"`
		} `json:"data"`
	}
	require.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &envelope))
	require.Len(t, envelope.Data.Items, 1)
	require.Equal(t, "AUTHORIZED", envelope.Data.Items[0].Status)
	require.EqualValues(t, 9, envelope.Data.Items[0].ChargedCredits)
	require.EqualValues(t, 88, envelope.Data.Items[0].Attachment.ID)
	require.Equal(t, "ARCHIVED", envelope.Data.Items[0].Attachment.Status)
}

func TestUpdateTopicAttachmentPriceBindsCurrentUserAndHidesObjectKey(t *testing.T) {
	gin.SetMode(gin.TestMode)
	attachment := &filepb.Attachment{
		Id:           88,
		TopicId:      1001,
		OwnerId:      42,
		ObjectKey:    "topics/1001/guide.pdf",
		OriginalName: "guide.pdf",
		ContentType:  "application/pdf",
		SizeBytes:    4,
		PriceCredits: 13,
		Status:       "ACTIVE",
	}
	contentClient := &fakeTopicContentClient{getTopicResp: &contentpb.TopicResponse{Topic: &contentpb.TopicInfo{Id: 1001, AuthorId: 42, Status: contentStatusPublished}}}
	userClient := &fakeUserClient{userResponse: &userpb.UserResponse{User: &userpb.UserInfo{Id: 42, Status: userStatusActive, EmailVerified: true}}}
	fileClient := &fakeAttachmentFileClient{
		getResp:    &filepb.AttachmentResponse{Attachment: attachment},
		updateResp: &filepb.AttachmentResponse{Attachment: attachment},
	}
	h := NewHandler(&clients.Clients{Content: contentClient, User: userClient, File: fileClient}, "Authorization", "Bearer", testJWTSecret)
	router := gin.New()
	NewInitControllers(h)(router)

	req := httptest.NewRequest(stdhttp.MethodPatch, "/api/v1/attachments/88", bytes.NewBufferString(`{"price_credits":13}`))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+signedAuthToken(t, jwt.MapClaims{"sub": "42", "username": "alice"}))
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, req)

	require.Equal(t, stdhttp.StatusOK, recorder.Code, recorder.Body.String())
	require.NotNil(t, fileClient.updateReq)
	require.EqualValues(t, 88, fileClient.updateReq.GetAttachmentId())
	require.EqualValues(t, 42, fileClient.updateReq.GetOwnerId())
	require.EqualValues(t, 13, fileClient.updateReq.GetPriceCredits())
	require.NotContains(t, recorder.Body.String(), "object_key")
}

func TestUpdateTopicAttachmentPriceRequiresPublishedEligibleAuthor(t *testing.T) {
	gin.SetMode(gin.TestMode)
	for _, tt := range []struct {
		name        string
		topicStatus int32
		user        *userpb.UserInfo
		wantStatus  int
		wantCode    string
	}{
		{
			name:        "unverified author",
			topicStatus: contentStatusPublished,
			user:        &userpb.UserInfo{Id: 42, Status: userStatusActive},
			wantStatus:  stdhttp.StatusForbidden,
			wantCode:    "email_not_verified",
		},
		{
			name:        "muted author",
			topicStatus: contentStatusPublished,
			user:        &userpb.UserInfo{Id: 42, Status: userStatusMuted, EmailVerified: true},
			wantStatus:  stdhttp.StatusForbidden,
			wantCode:    "user_muted",
		},
		{
			name:        "archived topic",
			topicStatus: 4,
			user:        &userpb.UserInfo{Id: 42, Status: userStatusActive, EmailVerified: true},
			wantStatus:  stdhttp.StatusPreconditionFailed,
			wantCode:    "topic must be published before updating attachment price",
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			attachment := &filepb.Attachment{Id: 88, TopicId: 1001, OwnerId: 42, Status: "ACTIVE"}
			contentClient := &fakeTopicContentClient{getTopicResp: &contentpb.TopicResponse{Topic: &contentpb.TopicInfo{Id: 1001, AuthorId: 42, Status: tt.topicStatus}}}
			userClient := &fakeUserClient{userResponse: &userpb.UserResponse{User: tt.user}}
			fileClient := &fakeAttachmentFileClient{getResp: &filepb.AttachmentResponse{Attachment: attachment}}
			h := NewHandler(&clients.Clients{
				Admin:   fakeAuthSettingsAdminClient{items: []*adminpb.SettingInfo{authSetting("auth.email_verification.required", "true")}},
				Content: contentClient,
				File:    fileClient,
				User:    userClient,
			}, "Authorization", "Bearer", testJWTSecret)
			router := gin.New()
			NewInitControllers(h)(router)

			req := httptest.NewRequest(stdhttp.MethodPatch, "/api/v1/attachments/88", bytes.NewBufferString(`{"price_credits":13}`))
			req.Header.Set("Content-Type", "application/json")
			req.Header.Set("Authorization", "Bearer "+signedAuthToken(t, jwt.MapClaims{"sub": "42", "username": "alice"}))
			recorder := httptest.NewRecorder()
			router.ServeHTTP(recorder, req)

			require.Equal(t, tt.wantStatus, recorder.Code, recorder.Body.String())
			require.Contains(t, recorder.Body.String(), tt.wantCode)
			require.NotNil(t, fileClient.getReq)
			require.Nil(t, fileClient.updateReq)
		})
	}
}

func TestUpdateTopicAttachmentPriceRejectsInvalidPrice(t *testing.T) {
	gin.SetMode(gin.TestMode)
	fileClient := &fakeAttachmentFileClient{}
	h := NewHandler(&clients.Clients{File: fileClient}, "Authorization", "Bearer", testJWTSecret)
	router := gin.New()
	NewInitControllers(h)(router)

	for _, body := range []string{`{}`, `{"price_credits":-1}`, `{"price_credits":"invalid"}`} {
		req := httptest.NewRequest(stdhttp.MethodPatch, "/api/v1/attachments/88", bytes.NewBufferString(body))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "Bearer "+signedAuthToken(t, jwt.MapClaims{"sub": "42", "username": "alice"}))
		recorder := httptest.NewRecorder()
		router.ServeHTTP(recorder, req)
		require.Equal(t, stdhttp.StatusBadRequest, recorder.Code, recorder.Body.String())
	}
	require.Nil(t, fileClient.updateReq)
}

func TestUpdateTopicAttachmentPriceRejectsNonOwner(t *testing.T) {
	gin.SetMode(gin.TestMode)
	attachment := &filepb.Attachment{Id: 88, TopicId: 1001, OwnerId: 7, Status: "ACTIVE"}
	contentClient := &fakeTopicContentClient{getTopicResp: &contentpb.TopicResponse{Topic: &contentpb.TopicInfo{Id: 1001, AuthorId: 7, Status: contentStatusPublished}}}
	fileClient := &fakeAttachmentFileClient{getResp: &filepb.AttachmentResponse{Attachment: attachment}}
	h := NewHandler(&clients.Clients{Content: contentClient, File: fileClient}, "Authorization", "Bearer", testJWTSecret)
	router := gin.New()
	NewInitControllers(h)(router)

	req := httptest.NewRequest(stdhttp.MethodPatch, "/api/v1/attachments/88", bytes.NewBufferString(`{"price_credits":13}`))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+signedAuthToken(t, jwt.MapClaims{"sub": "42", "username": "alice"}))
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, req)

	require.Equal(t, stdhttp.StatusForbidden, recorder.Code, recorder.Body.String())
	require.Nil(t, fileClient.updateReq)
}

func attachmentUploadRequest(t *testing.T, target, filename, content, price string) *stdhttp.Request {
	t.Helper()
	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	require.NoError(t, writer.WriteField("price_credits", price))
	part, err := writer.CreateFormFile("file", filename)
	require.NoError(t, err)
	_, err = part.Write([]byte(content))
	require.NoError(t, err)
	require.NoError(t, writer.Close())
	req := httptest.NewRequest(stdhttp.MethodPost, target, &body)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	return req
}

type fakeAttachmentFileClient struct {
	filepb.FileServiceClient
	createReq         *filepb.CreateAttachmentRequest
	getReq            *filepb.GetAttachmentRequest
	authorizeReq      *filepb.AuthorizeAttachmentDownloadRequest
	archiveReq        *filepb.ArchiveAttachmentRequest
	updateReq         *filepb.UpdateAttachmentPriceRequest
	listDownloadsReq  *filepb.ListUserAttachmentDownloadsRequest
	createResp        *filepb.AttachmentResponse
	getResp           *filepb.AttachmentResponse
	authorizeResp     *filepb.DownloadAuthorizationResponse
	listDownloadsResp *filepb.AttachmentDownloadListResponse
	createErr         error
	getErr            error
	authorizeErr      error
	updateErr         error
	updateResp        *filepb.AttachmentResponse
}

func (f *fakeAttachmentFileClient) CreateAttachment(_ context.Context, req *filepb.CreateAttachmentRequest, _ ...grpc.CallOption) (*filepb.AttachmentResponse, error) {
	f.createReq = req
	if f.createErr != nil {
		return nil, f.createErr
	}
	if f.createResp != nil {
		return f.createResp, nil
	}
	return &filepb.AttachmentResponse{Attachment: &filepb.Attachment{
		Id:           8,
		TopicId:      req.GetTopicId(),
		ObjectKey:    req.GetObjectKey(),
		OriginalName: req.GetOriginalName(),
		ContentType:  req.GetContentType(),
		SizeBytes:    req.GetSizeBytes(),
		PriceCredits: req.GetPriceCredits(),
		Status:       "ACTIVE",
	}}, nil
}

func (f *fakeAttachmentFileClient) GetAttachment(_ context.Context, req *filepb.GetAttachmentRequest, _ ...grpc.CallOption) (*filepb.AttachmentResponse, error) {
	f.getReq = req
	if f.getErr != nil {
		return nil, f.getErr
	}
	return f.getResp, nil
}

func (f *fakeAttachmentFileClient) ListUserAttachmentDownloads(_ context.Context, req *filepb.ListUserAttachmentDownloadsRequest, _ ...grpc.CallOption) (*filepb.AttachmentDownloadListResponse, error) {
	f.listDownloadsReq = req
	if f.listDownloadsResp != nil {
		return f.listDownloadsResp, nil
	}
	return &filepb.AttachmentDownloadListResponse{}, nil
}

func (f *fakeAttachmentFileClient) AuthorizeAttachmentDownload(_ context.Context, req *filepb.AuthorizeAttachmentDownloadRequest, _ ...grpc.CallOption) (*filepb.DownloadAuthorizationResponse, error) {
	f.authorizeReq = req
	if f.authorizeErr != nil {
		return nil, f.authorizeErr
	}
	return f.authorizeResp, nil
}

func (f *fakeAttachmentFileClient) ArchiveAttachment(_ context.Context, req *filepb.ArchiveAttachmentRequest, _ ...grpc.CallOption) (*filepb.AttachmentResponse, error) {
	f.archiveReq = req
	return &filepb.AttachmentResponse{}, nil
}

func (f *fakeAttachmentFileClient) UpdateAttachmentPrice(_ context.Context, req *filepb.UpdateAttachmentPriceRequest, _ ...grpc.CallOption) (*filepb.AttachmentResponse, error) {
	f.updateReq = req
	if f.updateErr != nil {
		return nil, f.updateErr
	}
	if f.updateResp != nil {
		return f.updateResp, nil
	}
	return &filepb.AttachmentResponse{}, nil
}

type fakeAttachmentStore struct {
	uploadKey string
	uploaded  []byte
	openKey   string
	openData  []byte
	openInfo  storage.ObjectInfo
	openErr   error
}

func (s *fakeAttachmentStore) Upload(_ context.Context, key string, reader io.Reader, _ int64, _ string) error {
	s.uploadKey = key
	data, err := io.ReadAll(reader)
	if err != nil {
		return err
	}
	s.uploaded = data
	return nil
}

func (s *fakeAttachmentStore) Open(_ context.Context, key string) (io.ReadCloser, storage.ObjectInfo, error) {
	s.openKey = key
	if s.openErr != nil {
		return nil, storage.ObjectInfo{}, s.openErr
	}
	return io.NopCloser(bytes.NewReader(s.openData)), s.openInfo, nil
}

func (s *fakeAttachmentStore) Delete(context.Context, string) error { return nil }

var _ storage.ObjectStore = (*fakeAttachmentStore)(nil)
