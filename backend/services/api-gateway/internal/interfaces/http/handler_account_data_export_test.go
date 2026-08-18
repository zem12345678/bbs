package http

import (
	"archive/zip"
	"bytes"
	"context"
	"encoding/json"
	"io"
	stdhttp "net/http"
	"net/http/httptest"
	"os"
	"sort"
	"strconv"
	"strings"
	"testing"
	"time"

	"api-gateway/api/proto/filepb"
	"api-gateway/api/proto/userpb"
	"api-gateway/internal/clients"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
)

func TestBuildAccountDataExportArtifactIncludesCompleteArchiveAndFiles(t *testing.T) {
	users := &accountDataUserStub{user: &userpb.UserInfo{
		Id: 42, Username: "alice", Email: "alice@example.com", Nickname: "Alice", Bio: "bio",
		Status: 1, CreatedAt: 1700000000000, UpdatedAt: 1700000000100, EmailVerified: true,
	}}
	users.following = []*userpb.UserInfo{{Id: 70, Username: "following", UpdatedAt: time.Now().UnixMilli()}}
	for i := int64(1); i <= 101; i++ {
		users.followers = append(users.followers, &userpb.UserInfo{Id: 1000 + i, Username: "follower" + strconv.FormatInt(i, 10)})
	}
	sessions := &accountDataSessionStub{}
	files := &accountDataFileStub{}
	store := newFakeUserFileStore()
	for i := int64(1); i <= 101; i++ {
		sessions.events = append(sessions.events, &userpb.LoginEventInfo{
			Id: strconv.FormatInt(2000+i, 10), UserId: 42, SessionId: "session-" + strconv.FormatInt(i, 10),
			IpAddress: "127.0.0.1", UserAgent: "test", Success: true, CreatedAt: 1700000000000 + i,
		})
		file := &filepb.File{
			Id: 3000 + i, OwnerId: 42, BizType: "drive", ObjectKey: "files/42/object-" + strconv.FormatInt(i, 10),
			OriginalName: "report/" + strconv.FormatInt(i, 10) + ".txt", ContentType: "text/plain",
			SizeBytes: int64(len("file-" + strconv.FormatInt(i, 10))), Status: "ACTIVE", CreatedAt: 1700000000000 + i,
		}
		files.files = append(files.files, file)
		if i != 101 {
			store.objects[file.GetObjectKey()] = fakeUserFileObject{data: []byte("file-" + strconv.FormatInt(i, 10)), contentType: "text/plain"}
		}
	}
	files.attachments = []*filepb.Attachment{{
		Id: 4001, TopicId: 5001, OwnerId: 42, ObjectKey: "attachments/42/archived", OriginalName: "archived.bin",
		ContentType: "application/octet-stream", SizeBytes: 10, Status: "ARCHIVED", CreatedAt: 1700000000200, ArchivedAt: 1700000000300,
	}}
	store.objects["attachments/42/archived"] = fakeUserFileObject{data: []byte("attachment"), contentType: "application/octet-stream"}
	h := newAccountDataTestHandler(users, sessions, files, store)

	artifact, err := h.buildAccountDataExportArtifact(context.Background(), 42)
	require.NoError(t, err)
	temp := artifact.reader.(*os.File)
	tempName := temp.Name()
	payload, err := io.ReadAll(artifact.reader)
	require.NoError(t, err)
	require.EqualValues(t, len(payload), artifact.size)
	artifact.cleanup()
	_, statErr := os.Stat(tempName)
	require.ErrorIs(t, statErr, os.ErrNotExist)

	entries := accountDataZipEntries(t, payload)
	for _, name := range []string{
		"user.json", "profile.json", "ips.json", "notes.json", "followings.json", "followers.json",
		"drive.json", "attachments.json", "mutings.json", "blockings.json", "favorites.json", "antennas.json", "lists.csv",
	} {
		require.Contains(t, entries, name)
	}
	require.Equal(t, "file-1", string(entries["files/3001-report_1.txt"]))
	require.NotContains(t, entries, "files/3101-report_101.txt")
	require.Equal(t, "attachment", string(entries["attachments/4001-archived.bin"]))
	require.Equal(t, []int64{0, 2100}, sessions.afterIDs)
	require.Equal(t, []int64{0, 3100}, files.afterIDs)
	require.Equal(t, []int64{0}, files.attachmentAfterIDs)
	require.Equal(t, []int64{0, 1100}, users.followerAfterIDs)

	var ips struct {
		MetaVersion int                           `json:"metaVersion"`
		Host        string                        `json:"host"`
		Items       []accountDataLoginEventRecord `json:"ips"`
	}
	require.NoError(t, json.Unmarshal(entries["ips.json"], &ips))
	require.Equal(t, 1, ips.MetaVersion)
	require.Equal(t, "bbs.example.com", ips.Host)
	require.Len(t, ips.Items, 101)

	var drive struct {
		Items []accountDataDriveRecord `json:"drive"`
	}
	require.NoError(t, json.Unmarshal(entries["drive.json"], &drive))
	require.Len(t, drive.Items, 101)
	require.Equal(t, "3001-report_1.txt", drive.Items[0].FileName)
	require.Equal(t, "https://bbs.example.com/api/v1/files/3001/download", drive.Items[0].File.URL)

	var following struct {
		Items []string `json:"followings"`
	}
	require.NoError(t, json.Unmarshal(entries["followings.json"], &following))
	require.Equal(t, []string{"following@bbs.example.com"}, following.Items)
	var followers struct {
		Items []string `json:"followers"`
	}
	require.NoError(t, json.Unmarshal(entries["followers.json"], &followers))
	require.Len(t, followers.Items, 101)
	require.Equal(t, "follower1@bbs.example.com", followers.Items[0])
}

func TestExportAccountDataRegistersZipAndCompletionNotification(t *testing.T) {
	gin.SetMode(gin.TestMode)
	users := &accountDataUserStub{user: &userpb.UserInfo{Id: 42, Username: "alice", CreatedAt: 1700000000000, UpdatedAt: 1700000000000}}
	files := &accountDataFileStub{createResponse: &filepb.FileResponse{File: &filepb.File{Id: 9090, OwnerId: 42}}}
	store := newFakeUserFileStore()
	notifications := &clipExportNotificationStub{}
	permit := &clipExportPermitStub{}
	h := newAccountDataTestHandler(users, &accountDataSessionStub{}, files, store)
	h.clients.NotificationInternal = notifications
	h.SetAccountDataExportGate(&clipExportGateStub{permit: permit})

	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest(stdhttp.MethodPost, "/i/export-data", strings.NewReader(`{}`))
	c.Set("user_id", int64(42))
	h.exportAccountData(c)

	require.Equal(t, stdhttp.StatusNoContent, c.Writer.Status(), recorder.Body.String())
	require.NotNil(t, files.createRequest)
	require.Equal(t, "exports", files.createRequest.GetBizType())
	require.Equal(t, "application/zip", files.createRequest.GetContentType())
	require.True(t, strings.HasPrefix(files.createRequest.GetOriginalName(), "data-request-"))
	require.True(t, strings.HasSuffix(files.createRequest.GetOriginalName(), ".zip"))
	require.True(t, permit.committed)
	require.Equal(t, "data", notifications.req.GetExportedEntity())
	require.EqualValues(t, 9090, notifications.req.GetFileId())
	require.Len(t, store.objects, 1)
	for _, object := range store.objects {
		require.Equal(t, "application/zip", object.contentType)
		require.Contains(t, accountDataZipEntries(t, object.data), "user.json")
	}
}

func TestAccountDataExportRoutesRequireInteractiveAuthAndApplyThreeDayGate(t *testing.T) {
	gin.SetMode(gin.TestMode)
	newHandler := func() *Handler {
		h := newAccountDataTestHandler(
			&accountDataUserStub{user: &userpb.UserInfo{Id: 42, Username: "alice"}},
			&accountDataSessionStub{}, &accountDataFileStub{}, newFakeUserFileStore(),
		)
		h.SetAccountDataExportGate(&clipExportGateStub{err: errExportRateLimited})
		return h
	}
	for _, path := range []string{"/i/export-data", "/api/i/export-data", "/api/v1/i/export-data"} {
		t.Run(path, func(t *testing.T) {
			router := gin.New()
			NewInitControllers(newHandler())(router)
			request := httptest.NewRequest(stdhttp.MethodPost, path, strings.NewReader(`{}`))
			request.Header.Set("Authorization", "Bearer "+signedAuthToken(t, jwt.MapClaims{"sub": "42"}))
			request.Header.Set("Content-Type", "application/json")
			recorder := httptest.NewRecorder()
			router.ServeHTTP(recorder, request)
			require.Equal(t, stdhttp.StatusTooManyRequests, recorder.Code, recorder.Body.String())
		})
	}

	router := gin.New()
	NewInitControllers(newHandler())(router)
	request := httptest.NewRequest(stdhttp.MethodPost, "/api/v1/i/export-data", strings.NewReader(`{}`))
	request.Header.Set("Authorization", "Bearer "+signedAuthToken(t, jwt.MapClaims{
		"sub": "42", "jti": "data-export-api-token", "exp": time.Now().Add(time.Hour).Unix(),
		credentialVersionClaim: "0", "token_type": apiTokenType, "scopes": []string{"read"},
	}))
	request.Header.Set("Content-Type", "application/json")
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, request)
	require.Equal(t, stdhttp.StatusForbidden, recorder.Code, recorder.Body.String())
}

func newAccountDataTestHandler(users *accountDataUserStub, sessions *accountDataSessionStub, files *accountDataFileStub, store *fakeUserFileStore) *Handler {
	h := NewHandlerWithAttachmentStore(&clients.Clients{
		User: users, UserSessions: sessions, Content: &noteExportContentStub{}, File: files,
		Reaction: &favoriteExportReactionStub{}, UserSafety: &safetyExportClientStub{},
		UserLists: &userListExportStub{}, UserAntennas: &antennaExportAntennaStub{},
	}, "Authorization", "Bearer", testJWTSecret, store)
	h.SetPublicBaseURL("https://bbs.example.com")
	return h
}

func accountDataZipEntries(t *testing.T, payload []byte) map[string][]byte {
	t.Helper()
	archive, err := zip.NewReader(bytes.NewReader(payload), int64(len(payload)))
	require.NoError(t, err)
	result := make(map[string][]byte, len(archive.File))
	for _, entry := range archive.File {
		reader, openErr := entry.Open()
		require.NoError(t, openErr)
		data, readErr := io.ReadAll(reader)
		require.NoError(t, readErr)
		require.NoError(t, reader.Close())
		result[entry.Name] = data
	}
	return result
}

type accountDataUserStub struct {
	clients.UserClient
	user             *userpb.UserInfo
	following        []*userpb.UserInfo
	followers        []*userpb.UserInfo
	followerAfterIDs []int64
}

func (s *accountDataUserStub) GetUser(_ context.Context, _ *userpb.UserIDRequest, _ ...grpc.CallOption) (*userpb.UserResponse, error) {
	return &userpb.UserResponse{User: s.user}, nil
}

func (s *accountDataUserStub) ListFollowing(_ context.Context, request *userpb.ListFollowsRequest, _ ...grpc.CallOption) (*userpb.UserListResponse, error) {
	return accountDataUserPage(s.following, request), nil
}

func (s *accountDataUserStub) ListFollowers(_ context.Context, request *userpb.ListFollowsRequest, _ ...grpc.CallOption) (*userpb.UserListResponse, error) {
	s.followerAfterIDs = append(s.followerAfterIDs, request.GetAfterUserId())
	return accountDataUserPage(s.followers, request), nil
}

func accountDataUserPage(items []*userpb.UserInfo, request *userpb.ListFollowsRequest) *userpb.UserListResponse {
	items = append([]*userpb.UserInfo{}, items...)
	sort.Slice(items, func(i, j int) bool { return items[i].GetId() < items[j].GetId() })
	start := 0
	for start < len(items) && items[start].GetId() <= request.GetAfterUserId() {
		start++
	}
	end := min(len(items), start+int(request.GetPageSize()))
	return &userpb.UserListResponse{Items: items[start:end], Total: int64(len(items))}
}

type accountDataSessionStub struct {
	clients.UserSessionClient
	events   []*userpb.LoginEventInfo
	afterIDs []int64
}

func (s *accountDataSessionStub) ListLoginEvents(_ context.Context, request *userpb.ListLoginEventsRequest, _ ...grpc.CallOption) (*userpb.LoginEventListResponse, error) {
	s.afterIDs = append(s.afterIDs, request.GetAfterId())
	items := make([]*userpb.LoginEventInfo, 0)
	for _, item := range s.events {
		id, _ := strconv.ParseInt(item.GetId(), 10, 64)
		if id > request.GetAfterId() {
			items = append(items, item)
		}
	}
	sort.Slice(items, func(i, j int) bool { return items[i].GetId() < items[j].GetId() })
	if len(items) > int(request.GetLimit()) {
		items = items[:request.GetLimit()]
	}
	return &userpb.LoginEventListResponse{Items: items, Total: int64(len(s.events))}, nil
}

type accountDataFileStub struct {
	filepb.FileServiceClient
	files              []*filepb.File
	attachments        []*filepb.Attachment
	afterIDs           []int64
	attachmentAfterIDs []int64
	createRequest      *filepb.CreateFileRequest
	createResponse     *filepb.FileResponse
}

func (s *accountDataFileStub) ListOwnedTopicAttachments(context.Context, *filepb.ListOwnedTopicAttachmentsRequest, ...grpc.CallOption) (*filepb.AttachmentListResponse, error) {
	return &filepb.AttachmentListResponse{}, nil
}

func (s *accountDataFileStub) ListFiles(_ context.Context, request *filepb.ListFilesRequest, _ ...grpc.CallOption) (*filepb.FileListResponse, error) {
	s.afterIDs = append(s.afterIDs, request.GetAfterId())
	items := make([]*filepb.File, 0)
	for _, item := range s.files {
		if item.GetId() > request.GetAfterId() {
			items = append(items, item)
		}
	}
	sort.Slice(items, func(i, j int) bool { return items[i].GetId() < items[j].GetId() })
	if len(items) > int(request.GetLimit()) {
		items = items[:request.GetLimit()]
	}
	return &filepb.FileListResponse{Items: items, Total: int64(len(s.files))}, nil
}

func (s *accountDataFileStub) ListOwnedAttachments(_ context.Context, request *filepb.ListOwnedAttachmentsRequest, _ ...grpc.CallOption) (*filepb.AttachmentListResponse, error) {
	s.attachmentAfterIDs = append(s.attachmentAfterIDs, request.GetAfterId())
	items := make([]*filepb.Attachment, 0)
	for _, item := range s.attachments {
		if item.GetId() > request.GetAfterId() {
			items = append(items, item)
		}
	}
	sort.Slice(items, func(i, j int) bool { return items[i].GetId() < items[j].GetId() })
	if len(items) > int(request.GetLimit()) {
		items = items[:request.GetLimit()]
	}
	return &filepb.AttachmentListResponse{Items: items}, nil
}

func (s *accountDataFileStub) CreateFile(_ context.Context, request *filepb.CreateFileRequest, _ ...grpc.CallOption) (*filepb.FileResponse, error) {
	s.createRequest = request
	return s.createResponse, nil
}

var _ clients.UserClient = (*accountDataUserStub)(nil)
var _ clients.UserSessionClient = (*accountDataSessionStub)(nil)
var _ clients.FileClient = (*accountDataFileStub)(nil)
