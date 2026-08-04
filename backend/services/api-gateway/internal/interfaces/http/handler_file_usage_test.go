package http

import (
	"context"
	"encoding/json"
	stdhttp "net/http"
	"net/http/httptest"
	"testing"

	"api-gateway/api/proto/filepb"
	"api-gateway/internal/clients"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
)

func TestGetFileUsageForwardsAuthenticatedOwner(t *testing.T) {
	gin.SetMode(gin.TestMode)
	client := &fakeFileUsageClient{response: &filepb.FileUsageResponse{
		UsedBytes: 120, CapacityBytes: 500, RemainingBytes: 380,
	}}
	h := NewHandler(&clients.Clients{File: client}, "Authorization", "Bearer", testJWTSecret)
	router := gin.New()
	NewInitControllers(h)(router)
	req := httptest.NewRequest(stdhttp.MethodGet, "/api/v1/files/usage", nil)
	req.Header.Set("Authorization", "Bearer "+signedAuthToken(t, jwt.MapClaims{"sub": "42"}))
	recorder := httptest.NewRecorder()

	router.ServeHTTP(recorder, req)

	require.Equal(t, stdhttp.StatusOK, recorder.Code, recorder.Body.String())
	require.NotNil(t, client.request)
	require.Equal(t, int64(42), client.request.GetOwnerId())
	var envelope struct {
		Data struct {
			UsedBytes      int64 `json:"used_bytes"`
			CapacityBytes  int64 `json:"capacity_bytes"`
			RemainingBytes int64 `json:"remaining_bytes"`
		} `json:"data"`
	}
	require.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &envelope))
	require.Equal(t, int64(120), envelope.Data.UsedBytes)
	require.Equal(t, int64(500), envelope.Data.CapacityBytes)
	require.Equal(t, int64(380), envelope.Data.RemainingBytes)
}

type fakeFileUsageClient struct {
	filepb.FileServiceClient
	request  *filepb.GetFileUsageRequest
	response *filepb.FileUsageResponse
}

func (f *fakeFileUsageClient) GetFileUsage(_ context.Context, req *filepb.GetFileUsageRequest, _ ...grpc.CallOption) (*filepb.FileUsageResponse, error) {
	f.request = req
	return f.response, nil
}
