package http

import (
	"bytes"
	"context"
	stdhttp "net/http"
	"net/http/httptest"
	"testing"
	"time"

	"api-gateway/api/proto/userpb"
	"api-gateway/internal/clients"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
)

func TestProfileUpdateCompatAliasesPreserveNullableFields(t *testing.T) {
	gin.SetMode(gin.TestMode)
	for _, path := range []string{"/i/update", "/api/i/update", "/api/v1/i/update"} {
		t.Run(path, func(t *testing.T) {
			client := &profileCompatUserClient{user: &userpb.UserInfo{
				Id: 42, Nickname: "Before", Bio: "old", Birthday: "1990-01-01",
				FollowingVisibility: "public", FollowersVisibility: "public", ProfileTheme: "default",
			}}
			h := NewHandler(&clients.Clients{User: client}, "Authorization", "Bearer", testJWTSecret)
			router := gin.New()
			NewInitControllers(h)(router)
			recorder := httptest.NewRecorder()
			request := httptest.NewRequest(stdhttp.MethodPost, path, bytes.NewBufferString(`{"name":"After","description":null,"birthday":null,"followingVisibility":"followers","followersVisibility":"private"}`))
			request.Header.Set("Content-Type", "application/json")
			request.Header.Set("Authorization", "Bearer "+signedAuthToken(t, jwt.MapClaims{
				"sub": "42", "jti": "profile-compat", "exp": time.Now().Add(time.Hour).Unix(),
				credentialVersionClaim: credentialVersionInitial, "token_type": apiTokenType, "scopes": []string{"write"},
			}))
			router.ServeHTTP(recorder, request)

			require.Equal(t, stdhttp.StatusOK, recorder.Code, recorder.Body.String())
			require.NotNil(t, client.req)
			require.Equal(t, "After", client.req.GetNickname())
			require.Equal(t, "", client.req.GetBio())
			require.NotNil(t, client.req.Birthday)
			require.Equal(t, "", client.req.GetBirthday())
			require.Equal(t, "followers", client.req.GetFollowingVisibility())
			require.Equal(t, "private", client.req.GetFollowersVisibility())
		})
	}
}

type profileCompatUserClient struct {
	userpb.UserServiceClient
	user *userpb.UserInfo
	req  *userpb.UpdateProfileRequest
}

func (client *profileCompatUserClient) GetUser(_ context.Context, _ *userpb.UserIDRequest, _ ...grpc.CallOption) (*userpb.UserResponse, error) {
	return &userpb.UserResponse{Success: true, User: client.user}, nil
}

func (client *profileCompatUserClient) UpdateProfile(_ context.Context, request *userpb.UpdateProfileRequest, _ ...grpc.CallOption) (*userpb.UserResponse, error) {
	client.req = request
	updated := *client.user
	updated.Nickname = request.GetNickname()
	updated.Bio = request.GetBio()
	if request.Birthday != nil {
		updated.Birthday = request.GetBirthday()
	}
	if request.FollowingVisibility != nil {
		updated.FollowingVisibility = request.GetFollowingVisibility()
	}
	if request.FollowersVisibility != nil {
		updated.FollowersVisibility = request.GetFollowersVisibility()
	}
	return &userpb.UserResponse{Success: true, User: &updated}, nil
}

var _ clients.UserClient = (*profileCompatUserClient)(nil)
