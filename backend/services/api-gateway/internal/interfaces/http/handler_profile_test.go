package http

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"api-gateway/api/proto/userpb"
	"api-gateway/internal/clients"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
)

func TestUpdateMeForwardsBackgroundURL(t *testing.T) {
	gin.SetMode(gin.TestMode)
	userClient := &captureUpdateProfileUserClient{}
	h := NewHandler(&clients.Clients{User: userClient}, "Authorization", "Bearer", testJWTSecret)

	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Set("user_id", int64(42))
	c.Request = httptest.NewRequest(
		http.MethodPut,
		"/api/v1/users/me",
		strings.NewReader(`{"nickname":"alice","avatar_url":"http://example.test/avatar.png","background_url":"http://example.test/bg.webp","bio":"hello"}`),
	)
	c.Request.Header.Set("Content-Type", "application/json")

	h.updateMe(c)

	require.Equal(t, http.StatusOK, recorder.Code, recorder.Body.String())
	require.NotNil(t, userClient.req)
	require.Equal(t, int64(42), userClient.req.GetId())
	require.Equal(t, "alice", userClient.req.GetNickname())
	require.Equal(t, "http://example.test/avatar.png", userClient.req.GetAvatarUrl())
	require.Equal(t, "http://example.test/bg.webp", userClient.req.GetBackgroundUrl())
	require.Equal(t, "hello", userClient.req.GetBio())
}

type captureUpdateProfileUserClient struct {
	userpb.UserServiceClient
	req *userpb.UpdateProfileRequest
}

func (c *captureUpdateProfileUserClient) UpdateProfile(_ context.Context, req *userpb.UpdateProfileRequest, _ ...grpc.CallOption) (*userpb.UserResponse, error) {
	c.req = req
	return &userpb.UserResponse{
		User: &userpb.UserInfo{
			Id:            req.GetId(),
			Nickname:      req.GetNickname(),
			AvatarUrl:     req.GetAvatarUrl(),
			BackgroundUrl: req.GetBackgroundUrl(),
			Bio:           req.GetBio(),
		},
	}, nil
}
