package http

import (
	"bytes"
	"encoding/json"
	"mime/multipart"
	stdhttp "net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
	"github.com/stretchr/testify/require"
)

func TestUploadUserAvatarSavesImageAndReturnsURL(t *testing.T) {
	gin.SetMode(gin.TestMode)
	t.Chdir(t.TempDir())

	recorder := performImageUpload(t, "/api/v1/users/me/avatar", "avatar.png")

	require.Equal(t, stdhttp.StatusOK, recorder.Code, recorder.Body.String())
	var envelope struct {
		Data map[string]string `json:"data"`
	}
	require.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &envelope))
	require.Equal(t, envelope.Data["url"], envelope.Data["avatar_url"])
	require.True(t, strings.HasPrefix(envelope.Data["url"], "http://example.test/uploads/avatars/"))
	require.True(t, strings.HasSuffix(envelope.Data["url"], ".png"))

	uploadedPath := strings.TrimPrefix(envelope.Data["path"], "/")
	_, err := os.Stat(filepath.FromSlash(uploadedPath))
	require.NoError(t, err)
}

func TestUploadImageSavesContentImageAndReturnsURL(t *testing.T) {
	gin.SetMode(gin.TestMode)
	t.Chdir(t.TempDir())

	recorder := performImageUpload(t, "/api/v1/uploads/images", "inline.webp")

	require.Equal(t, stdhttp.StatusOK, recorder.Code, recorder.Body.String())
	var envelope struct {
		Data map[string]string `json:"data"`
	}
	require.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &envelope))
	require.Equal(t, envelope.Data["url"], envelope.Data["image_url"])
	require.True(t, strings.HasPrefix(envelope.Data["url"], "http://example.test/uploads/images/"))
	require.True(t, strings.HasSuffix(envelope.Data["url"], ".webp"))

	uploadedPath := strings.TrimPrefix(envelope.Data["path"], "/")
	_, err := os.Stat(filepath.FromSlash(uploadedPath))
	require.NoError(t, err)
}

func performImageUpload(t *testing.T, path string, filename string) *httptest.ResponseRecorder {
	t.Helper()
	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	part, err := writer.CreateFormFile("file", filename)
	require.NoError(t, err)
	_, err = part.Write([]byte("image bytes"))
	require.NoError(t, err)
	require.NoError(t, writer.Close())

	router := gin.New()
	h := NewHandler(nil, "Authorization", "Bearer", testJWTSecret)
	NewInitControllers(h)(router)

	req := httptest.NewRequest(stdhttp.MethodPost, path, &body)
	req.Host = "example.test"
	req.Header.Set("Authorization", "Bearer "+signedAuthToken(t, jwt.MapClaims{"sub": "42", "username": "alice"}))
	req.Header.Set("Content-Type", writer.FormDataContentType())
	recorder := httptest.NewRecorder()

	router.ServeHTTP(recorder, req)
	return recorder
}
