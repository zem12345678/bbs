package http

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	stdhttp "net/http"
	"net/http/httptest"
	"testing"

	"api-gateway/internal/clients"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
	"github.com/stretchr/testify/require"
)

func TestUserUploadRateLimitStopsEveryUploadBeforeReadingMultipartBody(t *testing.T) {
	endpoints := []struct {
		name string
		path string
	}{
		{name: "file", path: "/api/v1/files"},
		{name: "topic attachment", path: "/api/v1/topics/1001/attachments"},
		{name: "content image", path: "/api/v1/uploads/images"},
		{name: "user avatar", path: "/api/v1/users/me/avatar"},
	}
	failures := []struct {
		name        string
		limiter     *fileUploadRateLimitStub
		wantStatus  int
		wantMessage string
		wantCode    string
	}{
		{
			name: "window exhausted", limiter: &fileUploadRateLimitStub{limited: true},
			wantStatus: stdhttp.StatusTooManyRequests, wantMessage: "file upload rate limit exceeded", wantCode: "rate_limited",
		},
		{
			name: "limiter unavailable", limiter: &fileUploadRateLimitStub{err: errors.New("redis unavailable")},
			wantStatus: stdhttp.StatusServiceUnavailable, wantMessage: "file upload rate limiter unavailable", wantCode: "unavailable",
		},
	}
	for _, endpoint := range endpoints {
		for _, failure := range failures {
			t.Run(endpoint.name+"/"+failure.name, func(t *testing.T) {
				limiter := &fileUploadRateLimitStub{limited: failure.limiter.limited, err: failure.limiter.err}
				gin.SetMode(gin.TestMode)
				fileClient := &fakeUserFileClient{}
				store := newFakeUserFileStore()
				h := NewHandlerWithAttachmentStore(&clients.Clients{File: fileClient}, "Authorization", "Bearer", testJWTSecret, store)
				h.SetFileUploadLimit(limiter)
				router := gin.New()
				NewInitControllers(h)(router)
				body := &fileUploadUnreadBody{}
				req := httptest.NewRequest(stdhttp.MethodPost, endpoint.path, nil)
				req.Body = body
				req.ContentLength = -1
				req.Header.Set("Content-Type", "multipart/form-data; boundary=unused")
				req.Header.Set("Authorization", "Bearer "+signedAuthToken(t, jwt.MapClaims{"sub": "42"}))
				recorder := httptest.NewRecorder()

				router.ServeHTTP(recorder, req)

				require.Equal(t, failure.wantStatus, recorder.Code, recorder.Body.String())
				require.Equal(t, []string{"rate:files:upload:user:42"}, limiter.keys)
				require.Zero(t, body.reads)
				require.Nil(t, fileClient.createReq)
				require.Empty(t, store.objects)

				var envelope struct {
					Message string         `json:"message"`
					Meta    map[string]any `json:"meta"`
				}
				require.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &envelope))
				require.Equal(t, failure.wantMessage, envelope.Message)
				require.Equal(t, failure.wantCode, envelope.Meta["legacy_code"])
			})
		}
	}
}

func TestAdminAvatarDoesNotUseUserUploadRateLimit(t *testing.T) {
	gin.SetMode(gin.TestMode)
	limiter := &fileUploadRateLimitStub{limited: true}
	store := newFakeUserFileStore()
	h := NewHandlerWithAttachmentStore(&clients.Clients{
		Admin: &fakeFileAdminClient{},
	}, "Authorization", "Bearer", testJWTSecret, store)
	h.SetFileUploadLimit(limiter)
	h.SetPublicBaseURL("http://example.test")
	router := gin.New()
	NewInitControllers(h)(router)

	adminRecorder := performImageUpload(t, router, "/api/v1/admin/uploads/avatar", "admin.png")
	require.Equal(t, stdhttp.StatusOK, adminRecorder.Code, adminRecorder.Body.String())
	require.Empty(t, limiter.keys)
}

type fileUploadRateLimitStub struct {
	limited bool
	err     error
	keys    []string
}

func (l *fileUploadRateLimitStub) Limit(_ context.Context, key string) (bool, error) {
	l.keys = append(l.keys, key)
	return l.limited, l.err
}

type fileUploadUnreadBody struct {
	reads int
}

func (b *fileUploadUnreadBody) Read([]byte) (int, error) {
	b.reads++
	return 0, io.EOF
}

func (*fileUploadUnreadBody) Close() error {
	return nil
}
