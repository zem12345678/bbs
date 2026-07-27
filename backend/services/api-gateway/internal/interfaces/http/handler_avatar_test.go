package http

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"mime/multipart"
	stdhttp "net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"api-gateway/internal/storage"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
	"github.com/stretchr/testify/require"
)

func TestUploadUserAvatarSavesImageAndReturnsURL(t *testing.T) {
	gin.SetMode(gin.TestMode)
	t.Chdir(t.TempDir())
	router, store := newImageUploadRouter()

	recorder := performImageUpload(t, router, "/api/v1/users/me/avatar", "avatar.png")

	require.Equal(t, stdhttp.StatusOK, recorder.Code, recorder.Body.String())
	var envelope struct {
		Data map[string]string `json:"data"`
	}
	require.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &envelope))
	require.Equal(t, envelope.Data["url"], envelope.Data["avatar_url"])
	require.True(t, strings.HasPrefix(envelope.Data["url"], "http://example.test/uploads/avatars/"))
	require.True(t, strings.HasSuffix(envelope.Data["url"], ".png"))

	uploadedPath := strings.TrimPrefix(envelope.Data["path"], "/")
	require.Equal(t, testPNGImage, store.objects[uploadedPath].data)
	_, err := os.Stat("uploads")
	require.ErrorIs(t, err, os.ErrNotExist)
}

func TestUploadImageSavesContentImageAndReturnsURL(t *testing.T) {
	gin.SetMode(gin.TestMode)
	t.Chdir(t.TempDir())
	router, store := newImageUploadRouter()

	recorder := performImageUpload(t, router, "/api/v1/uploads/images", "inline.png")

	require.Equal(t, stdhttp.StatusOK, recorder.Code, recorder.Body.String())
	var envelope struct {
		Data map[string]string `json:"data"`
	}
	require.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &envelope))
	require.Equal(t, envelope.Data["url"], envelope.Data["image_url"])
	require.True(t, strings.HasPrefix(envelope.Data["url"], "http://example.test/uploads/images/"))
	require.NotContains(t, envelope.Data["url"], "attacker.example")
	require.True(t, strings.HasSuffix(envelope.Data["url"], ".png"))

	uploadedPath := strings.TrimPrefix(envelope.Data["path"], "/")
	require.Equal(t, testPNGImage, store.objects[uploadedPath].data)
	_, err := os.Stat("uploads")
	require.ErrorIs(t, err, os.ErrNotExist)
}

func TestUploadedImageIsServedFromObjectStorage(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router, _ := newImageUploadRouter()
	uploaded := performImageUpload(t, router, "/api/v1/uploads/images", "inline.png")
	var envelope struct {
		Data map[string]string `json:"data"`
	}
	require.NoError(t, json.Unmarshal(uploaded.Body.Bytes(), &envelope))

	req := httptest.NewRequest(stdhttp.MethodGet, envelope.Data["path"], nil)
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, req)

	require.Equal(t, stdhttp.StatusOK, recorder.Code, recorder.Body.String())
	require.Equal(t, "image/png", recorder.Header().Get("Content-Type"))
	require.Equal(t, "public, max-age=31536000, immutable", recorder.Header().Get("Cache-Control"))
	require.Equal(t, "nosniff", recorder.Header().Get("X-Content-Type-Options"))
	require.Equal(t, string(testPNGImage), recorder.Body.String())
}

func TestUploadImageRejectsContentMismatchedWithExtension(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router, store := newImageUploadRouter()
	recorder := performImageUploadData(t, router, "/api/v1/uploads/images", "not-an-image.png", []byte("<html>not an image</html>"))

	require.Equal(t, stdhttp.StatusBadRequest, recorder.Code, recorder.Body.String())
	require.Empty(t, store.objects)
}

func TestUploadedImageRouteDoesNotExposeAttachmentObjects(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router, store := newImageUploadRouter()

	req := httptest.NewRequest(stdhttp.MethodGet, "/uploads/topics/secret.pdf", nil)
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, req)

	require.Equal(t, stdhttp.StatusNotFound, recorder.Code, recorder.Body.String())
	require.Empty(t, store.openKeys)
}

func performImageUpload(t *testing.T, router *gin.Engine, path string, filename string) *httptest.ResponseRecorder {
	return performImageUploadData(t, router, path, filename, testPNGImage)
}

func performImageUploadData(t *testing.T, router *gin.Engine, path string, filename string, data []byte) *httptest.ResponseRecorder {
	t.Helper()
	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	part, err := writer.CreateFormFile("file", filename)
	require.NoError(t, err)
	_, err = part.Write(data)
	require.NoError(t, err)
	require.NoError(t, writer.Close())

	req := httptest.NewRequest(stdhttp.MethodPost, path, &body)
	req.Host = "example.test"
	req.Header.Set("X-Forwarded-Host", "attacker.example")
	req.Header.Set("X-Forwarded-Proto", "https")
	req.Header.Set("Authorization", "Bearer "+signedAuthToken(t, jwt.MapClaims{"sub": "42", "username": "alice"}))
	req.Header.Set("Content-Type", writer.FormDataContentType())
	recorder := httptest.NewRecorder()

	router.ServeHTTP(recorder, req)
	return recorder
}

func newImageUploadRouter() (*gin.Engine, *fakeImageStore) {
	store := &fakeImageStore{objects: make(map[string]fakeImageObject)}
	router := gin.New()
	h := NewHandlerWithAttachmentStore(nil, "Authorization", "Bearer", testJWTSecret, store)
	h.SetPublicBaseURL("http://example.test")
	NewInitControllers(h)(router)
	return router, store
}

type fakeImageObject struct {
	data        []byte
	contentType string
}

type fakeImageStore struct {
	objects  map[string]fakeImageObject
	openKeys []string
}

func (s *fakeImageStore) Upload(_ context.Context, key string, reader io.Reader, _ int64, contentType string) error {
	data, err := io.ReadAll(reader)
	if err != nil {
		return err
	}
	s.objects[key] = fakeImageObject{data: data, contentType: contentType}
	return nil
}

func (s *fakeImageStore) Open(_ context.Context, key string) (io.ReadCloser, storage.ObjectInfo, error) {
	s.openKeys = append(s.openKeys, key)
	object, ok := s.objects[key]
	if !ok {
		return nil, storage.ObjectInfo{}, storage.ErrObjectNotFound
	}
	return io.NopCloser(bytes.NewReader(object.data)), storage.ObjectInfo{Size: int64(len(object.data)), ContentType: object.contentType}, nil
}

func (s *fakeImageStore) Delete(_ context.Context, key string) error {
	delete(s.objects, key)
	return nil
}

var _ storage.ObjectStore = (*fakeImageStore)(nil)

var testPNGImage = []byte{0x89, 'P', 'N', 'G', '\r', '\n', 0x1a, '\n'}
