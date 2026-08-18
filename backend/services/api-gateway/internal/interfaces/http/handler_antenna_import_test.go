package http

import (
	"bytes"
	"context"
	"errors"
	stdhttp "net/http"
	"net/http/httptest"
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

func TestImportAntennasReadsOwnedFileAndCreatesValidRecords(t *testing.T) {
	gin.SetMode(gin.TestMode)
	payload := []byte(`[
		{"name":" Go watch ","src":"all","keywords":[["Go","Redis"]],"excludeKeywords":[["spam"]],"users":[],"caseSensitive":true,"localOnly":true,"excludeBots":true,"withReplies":true,"withFile":true},
		{"name":"list members","src":"list","keywords":[["release"]],"excludeKeywords":[],"users":["unused"],"userListAccts":["alice@bbs.example.com","bob@bbs.example.com"],"caseSensitive":false,"localOnly":false,"excludeBots":false,"withReplies":false,"withFile":false},
		{"name":" ","src":"all","keywords":[["ignored"]],"excludeKeywords":[],"users":[],"caseSensitive":false,"withReplies":false,"withFile":false}
	]`)
	store := newFakeUserFileStore()
	store.objects["files/42/antennas.json"] = fakeUserFileObject{data: payload, contentType: "application/json"}
	files := &fakeUserFileClient{getResp: &filepb.FileResponse{File: &filepb.File{
		Id: 99, OwnerId: 42, ObjectKey: "files/42/antennas.json", SizeBytes: int64(len(payload)), Status: "ACTIVE",
	}}}
	antennas := &antennaImportUserStub{}
	limiter := &antennaImportLimiterStub{}
	h := NewHandlerWithAttachmentStore(&clients.Clients{File: files, UserAntennas: antennas}, "Authorization", "Bearer", testJWTSecret, store)
	h.SetAntennaImportLimit(limiter)

	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest(stdhttp.MethodPost, "/i/import-antennas", strings.NewReader(`{"fileId":"99"}`))
	c.Request.Header.Set("Content-Type", "application/json")
	c.Set("user_id", int64(42))

	h.importAntennas(c)
	c.Writer.WriteHeaderNow()

	require.Equal(t, stdhttp.StatusNoContent, recorder.Code, recorder.Body.String())
	require.Equal(t, int64(42), files.getReq.GetOwnerId())
	require.Equal(t, int64(99), files.getReq.GetFileId())
	require.Equal(t, []string{"files/42/antennas.json"}, store.openKeys)
	require.Equal(t, []string{"rate:imports:antennas:user:42"}, limiter.keys)
	require.Len(t, antennas.createRequests, 2)
	require.Equal(t, "Go watch", antennas.createRequests[0].GetName())
	require.Equal(t, "all", antennas.createRequests[0].GetSource())
	require.Equal(t, []string{"Go", "Redis"}, antennas.createRequests[0].GetKeywords()[0].GetTerms())
	require.Equal(t, []string{"spam"}, antennas.createRequests[0].GetExcludeKeywords()[0].GetTerms())
	require.True(t, antennas.createRequests[0].GetCaseSensitive())
	require.True(t, antennas.createRequests[0].GetLocalOnly())
	require.True(t, antennas.createRequests[0].GetExcludeBots())
	require.True(t, antennas.createRequests[0].GetWithReplies())
	require.True(t, antennas.createRequests[0].GetWithFile())
	require.Equal(t, "users", antennas.createRequests[1].GetSource())
	require.Equal(t, []string{"alice@bbs.example.com", "bob@bbs.example.com"}, antennas.createRequests[1].GetUsers())
}

func TestImportAntennasSkipsInvalidRecordsAndAcceptsEmptyArray(t *testing.T) {
	for _, testCase := range []struct {
		name    string
		payload string
	}{
		{name: "empty array", payload: `[]`},
		{name: "invalid records", payload: `[{"name":"missing keywords","src":"all","keywords":[],"excludeKeywords":[],"users":[]}]`},
	} {
		t.Run(testCase.name, func(t *testing.T) {
			store := newFakeUserFileStore()
			store.objects["files/42/import.json"] = fakeUserFileObject{data: []byte(testCase.payload)}
			files := &fakeUserFileClient{getResp: &filepb.FileResponse{File: &filepb.File{
				Id: 8, OwnerId: 42, ObjectKey: "files/42/import.json", SizeBytes: int64(len(testCase.payload)),
			}}}
			antennas := &antennaImportUserStub{}
			h := NewHandlerWithAttachmentStore(&clients.Clients{File: files, UserAntennas: antennas}, "Authorization", "Bearer", testJWTSecret, store)
			recorder := performAntennaImport(h, `{"fileId":"8"}`, 42)

			require.Equal(t, stdhttp.StatusNoContent, recorder.Code, recorder.Body.String())
			require.Empty(t, antennas.createRequests)
			require.Zero(t, antennas.listCalls)
		})
	}
}

func TestImportAntennasRejectsUnownedMalformedAndOversizedFiles(t *testing.T) {
	validPayload := []byte(`[{"name":"watch","src":"all","keywords":[["go"]],"excludeKeywords":[],"users":[]}]`)
	oversizedPayload := bytes.Repeat([]byte(" "), int(antennaImportMaxBytes)+1)
	for _, testCase := range []struct {
		name       string
		file       *filepb.File
		payload    []byte
		wantStatus int
		wantOpen   bool
	}{
		{name: "other owner", file: &filepb.File{Id: 7, OwnerId: 99, ObjectKey: "other.json", SizeBytes: int64(len(validPayload))}, payload: validPayload, wantStatus: stdhttp.StatusNotFound},
		{name: "malformed JSON", file: &filepb.File{Id: 7, OwnerId: 42, ObjectKey: "bad.json", SizeBytes: 2}, payload: []byte(`{]`), wantStatus: stdhttp.StatusBadRequest, wantOpen: true},
		{name: "trailing JSON", file: &filepb.File{Id: 7, OwnerId: 42, ObjectKey: "trailing.json", SizeBytes: 5}, payload: []byte(`[] {}`), wantStatus: stdhttp.StatusBadRequest, wantOpen: true},
		{name: "metadata oversized", file: &filepb.File{Id: 7, OwnerId: 42, ObjectKey: "large.json", SizeBytes: antennaImportMaxBytes + 1}, payload: validPayload, wantStatus: stdhttp.StatusRequestEntityTooLarge},
		{name: "object oversized", file: &filepb.File{Id: 7, OwnerId: 42, ObjectKey: "actual-large.json", SizeBytes: 1}, payload: oversizedPayload, wantStatus: stdhttp.StatusRequestEntityTooLarge, wantOpen: true},
	} {
		t.Run(testCase.name, func(t *testing.T) {
			store := newFakeUserFileStore()
			store.objects[testCase.file.GetObjectKey()] = fakeUserFileObject{data: testCase.payload}
			files := &fakeUserFileClient{getResp: &filepb.FileResponse{File: testCase.file}}
			antennas := &antennaImportUserStub{}
			h := NewHandlerWithAttachmentStore(&clients.Clients{File: files, UserAntennas: antennas}, "Authorization", "Bearer", testJWTSecret, store)

			recorder := performAntennaImport(h, `{"fileId":"7"}`, 42)

			require.Equal(t, testCase.wantStatus, recorder.Code, recorder.Body.String())
			require.Equal(t, testCase.wantOpen, len(store.openKeys) > 0)
			require.Empty(t, antennas.createRequests)
		})
	}
}

func TestImportAntennasRejectsMissingAndEmptyFileMetadata(t *testing.T) {
	for _, testCase := range []struct {
		name       string
		response   *filepb.FileResponse
		wantStatus int
	}{
		{name: "missing file", response: &filepb.FileResponse{}, wantStatus: stdhttp.StatusNotFound},
		{name: "empty file", response: &filepb.FileResponse{File: &filepb.File{Id: 7, OwnerId: 42, ObjectKey: "empty.json"}}, wantStatus: stdhttp.StatusBadRequest},
	} {
		t.Run(testCase.name, func(t *testing.T) {
			store := newFakeUserFileStore()
			files := &fakeUserFileClient{getResp: testCase.response}
			antennas := &antennaImportUserStub{}
			h := NewHandlerWithAttachmentStore(&clients.Clients{File: files, UserAntennas: antennas}, "Authorization", "Bearer", testJWTSecret, store)

			recorder := performAntennaImport(h, `{"fileId":"7"}`, 42)

			require.Equal(t, testCase.wantStatus, recorder.Code, recorder.Body.String())
			require.Empty(t, store.openKeys)
			require.Empty(t, antennas.createRequests)
		})
	}
}

func TestImportAntennasChecksCapacityBeforeCreating(t *testing.T) {
	payload := []byte(`[{"name":"watch","src":"all","keywords":[["go"]],"excludeKeywords":[],"users":[]}]`)
	store := newFakeUserFileStore()
	store.objects["import.json"] = fakeUserFileObject{data: payload}
	files := &fakeUserFileClient{getResp: &filepb.FileResponse{File: &filepb.File{Id: 7, OwnerId: 42, ObjectKey: "import.json", SizeBytes: int64(len(payload))}}}
	existing := make([]*userpb.AntennaInfo, antennaImportMaxItems)
	antennas := &antennaImportUserStub{existing: existing}
	h := NewHandlerWithAttachmentStore(&clients.Clients{File: files, UserAntennas: antennas}, "Authorization", "Bearer", testJWTSecret, store)

	recorder := performAntennaImport(h, `{"fileId":"7"}`, 42)

	require.Equal(t, stdhttp.StatusPreconditionFailed, recorder.Code, recorder.Body.String())
	require.Empty(t, antennas.createRequests)
}

func TestAntennaImportRoutesRequireInteractiveSessionAndApplyRateLimit(t *testing.T) {
	gin.SetMode(gin.TestMode)
	newHandler := func() (*Handler, *antennaImportLimiterStub) {
		limiter := &antennaImportLimiterStub{limited: true}
		h := NewHandlerWithAttachmentStore(&clients.Clients{
			File: &fakeUserFileClient{}, UserAntennas: &antennaImportUserStub{},
		}, "Authorization", "Bearer", testJWTSecret, newFakeUserFileStore())
		h.SetAntennaImportLimit(limiter)
		return h, limiter
	}
	for _, path := range []string{"/i/import-antennas", "/api/i/import-antennas", "/api/v1/i/import-antennas"} {
		t.Run(path, func(t *testing.T) {
			h, limiter := newHandler()
			router := gin.New()
			NewInitControllers(h)(router)
			request := httptest.NewRequest(stdhttp.MethodPost, path, strings.NewReader(`{"fileId":"7"}`))
			request.Header.Set("Authorization", "Bearer "+signedAuthToken(t, jwt.MapClaims{"sub": "42"}))
			request.Header.Set("Content-Type", "application/json")
			recorder := httptest.NewRecorder()

			router.ServeHTTP(recorder, request)

			require.Equal(t, stdhttp.StatusTooManyRequests, recorder.Code, recorder.Body.String())
			require.Equal(t, []string{"rate:imports:antennas:user:42"}, limiter.keys)
		})
	}

	h, limiter := newHandler()
	router := gin.New()
	NewInitControllers(h)(router)
	request := httptest.NewRequest(stdhttp.MethodPost, "/api/v1/i/import-antennas", strings.NewReader(`{"fileId":"7"}`))
	request.Header.Set("Authorization", "Bearer "+signedAuthToken(t, jwt.MapClaims{
		"sub": "42", "jti": "antenna-import-api-token", "exp": time.Now().Add(time.Hour).Unix(),
		credentialVersionClaim: "0", "token_type": apiTokenType, "scopes": []string{"write"},
	}))
	request.Header.Set("Content-Type", "application/json")
	recorder := httptest.NewRecorder()

	router.ServeHTTP(recorder, request)

	require.Equal(t, stdhttp.StatusForbidden, recorder.Code, recorder.Body.String())
	require.Empty(t, limiter.keys)
}

func TestAntennaImportRateLimiterFailureIsUnavailable(t *testing.T) {
	store := newFakeUserFileStore()
	limiter := &antennaImportLimiterStub{err: errors.New("redis unavailable")}
	h := NewHandlerWithAttachmentStore(&clients.Clients{File: &fakeUserFileClient{}, UserAntennas: &antennaImportUserStub{}}, "Authorization", "Bearer", testJWTSecret, store)
	h.SetAntennaImportLimit(limiter)

	recorder := performAntennaImport(h, `{"fileId":"7"}`, 42)

	require.Equal(t, stdhttp.StatusServiceUnavailable, recorder.Code, recorder.Body.String())
	require.Equal(t, []string{"rate:imports:antennas:user:42"}, limiter.keys)
}

func TestImportedAntennaAccountsResolveOnlyForTheLocalHost(t *testing.T) {
	users := &antennaImportUsernameStub{}
	h := NewHandler(&clients.Clients{User: users}, "Authorization", "Bearer", testJWTSecret)
	h.SetPublicBaseURL("https://bbs.example.com:8443")

	ids, err := h.resolveAntennaUserIDs(context.Background(), []string{
		"alice@bbs.example.com:8443", "remote@example.net", "17",
	})

	require.NoError(t, err)
	require.Equal(t, []int64{71, 17}, ids)
	require.Equal(t, []string{"alice"}, users.usernames)
}

func performAntennaImport(h *Handler, body string, ownerID int64) *httptest.ResponseRecorder {
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest(stdhttp.MethodPost, "/i/import-antennas", strings.NewReader(body))
	c.Request.Header.Set("Content-Type", "application/json")
	c.Set("user_id", ownerID)
	h.importAntennas(c)
	c.Writer.WriteHeaderNow()
	return recorder
}

type antennaImportUserStub struct {
	clients.UserAntennaClient
	existing       []*userpb.AntennaInfo
	createRequests []*userpb.CreateAntennaRequest
	listCalls      int
}

func (s *antennaImportUserStub) ListAntennas(_ context.Context, _ *userpb.ListAntennasRequest, _ ...grpc.CallOption) (*userpb.AntennaListResponse, error) {
	s.listCalls++
	return &userpb.AntennaListResponse{Items: s.existing, Total: int64(len(s.existing))}, nil
}

func (s *antennaImportUserStub) CreateAntenna(_ context.Context, request *userpb.CreateAntennaRequest, _ ...grpc.CallOption) (*userpb.AntennaInfoResponse, error) {
	s.createRequests = append(s.createRequests, request)
	return &userpb.AntennaInfoResponse{Antenna: &userpb.AntennaInfo{Id: int64(len(s.createRequests)), OwnerId: request.GetOwnerId()}}, nil
}

type antennaImportLimiterStub struct {
	limited bool
	err     error
	keys    []string
}

func (s *antennaImportLimiterStub) Limit(_ context.Context, key string) (bool, error) {
	s.keys = append(s.keys, key)
	return s.limited, s.err
}

type antennaImportUsernameStub struct {
	clients.UserClient
	usernames []string
}

func (s *antennaImportUsernameStub) GetUserByUsername(_ context.Context, request *userpb.UsernameRequest, _ ...grpc.CallOption) (*userpb.UserResponse, error) {
	s.usernames = append(s.usernames, request.GetUsername())
	return &userpb.UserResponse{User: &userpb.UserInfo{Id: 71, Username: request.GetUsername()}}, nil
}
