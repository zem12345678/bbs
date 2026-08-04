package http

import (
	"context"
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

func TestUploadFileRateLimitStopsBeforeReadingMultipartBody(t *testing.T) {
	tests := []struct {
		name       string
		limiter    *fileUploadRateLimitStub
		wantStatus int
	}{
		{name: "window exhausted", limiter: &fileUploadRateLimitStub{limited: true}, wantStatus: stdhttp.StatusTooManyRequests},
		{name: "limiter unavailable", limiter: &fileUploadRateLimitStub{err: errors.New("redis unavailable")}, wantStatus: stdhttp.StatusServiceUnavailable},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gin.SetMode(gin.TestMode)
			fileClient := &fakeUserFileClient{}
			store := newFakeUserFileStore()
			h := NewHandlerWithAttachmentStore(&clients.Clients{File: fileClient}, "Authorization", "Bearer", testJWTSecret, store)
			h.SetFileUploadLimit(tt.limiter)
			router := gin.New()
			NewInitControllers(h)(router)
			body := &fileUploadUnreadBody{}
			req := httptest.NewRequest(stdhttp.MethodPost, "/api/v1/files", nil)
			req.Body = body
			req.ContentLength = -1
			req.Header.Set("Content-Type", "multipart/form-data; boundary=unused")
			req.Header.Set("Authorization", "Bearer "+signedAuthToken(t, jwt.MapClaims{"sub": "42"}))
			recorder := httptest.NewRecorder()

			router.ServeHTTP(recorder, req)

			require.Equal(t, tt.wantStatus, recorder.Code, recorder.Body.String())
			require.Equal(t, []string{"rate:files:upload:user:42"}, tt.limiter.keys)
			require.Zero(t, body.reads)
			require.Nil(t, fileClient.createReq)
			require.Empty(t, store.objects)
		})
	}
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
