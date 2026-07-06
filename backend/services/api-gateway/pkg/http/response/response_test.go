package response

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"api-gateway/pkg/exception"

	"github.com/gin-gonic/gin"
	"github.com/spf13/viper"
	"github.com/stretchr/testify/require"
)

func TestSuccessEnvelope(t *testing.T) {
	gin.SetMode(gin.TestMode)
	viper.Set("app.name", "gateway-test")

	router := gin.New()
	router.GET("/ok", func(c *gin.Context) {
		Success(c, gin.H{"id": "route-1"})
	})

	req := httptest.NewRequest(http.MethodGet, "/ok", nil)
	req.Header.Set("X-Request-ID", "req-1")
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	require.Equal(t, http.StatusOK, rec.Code)

	var got map[string]any
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &got))
	require.Equal(t, "gateway-test", got["service"])
	require.Equal(t, float64(http.StatusOK), got["http_code"])
	require.Equal(t, float64(0), got["code"])
	require.Equal(t, "成功", got["reason"])
	require.Equal(t, "success", got["message"])
	require.Equal(t, "req-1", got["request_id"])
	require.Equal(t, map[string]any{"id": "route-1"}, got["data"])
}

func TestFailedEnvelopeUsesWrappedApiException(t *testing.T) {
	gin.SetMode(gin.TestMode)
	viper.Set("app.name", "gateway-test")

	router := gin.New()
	router.GET("/bad-request", func(c *gin.Context) {
		Failed(c, fmt.Errorf("wrapped: %w", exception.NewBadRequest("参数错误")))
	})

	req := httptest.NewRequest(http.MethodGet, "/bad-request", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	require.Equal(t, http.StatusBadRequest, rec.Code)

	var got map[string]any
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &got))
	require.Equal(t, "gateway-test", got["service"])
	require.Equal(t, float64(http.StatusBadRequest), got["http_code"])
	require.Equal(t, float64(exception.CODE_BAD_REQUEST), got["code"])
	require.Equal(t, "请求不合法", got["reason"])
	require.Equal(t, "参数错误", got["message"])
}
