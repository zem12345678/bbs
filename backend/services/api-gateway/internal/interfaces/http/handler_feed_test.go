package http

import (
	"context"
	"encoding/json"
	stdhttp "net/http"
	"net/http/httptest"
	"testing"

	"api-gateway/api/proto/adminpb"
	"api-gateway/api/proto/feedpb"
	"api-gateway/api/proto/userpb"
	"api-gateway/internal/clients"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
)

func TestFeedArticlesFollowingFiltersByFollowedAuthors(t *testing.T) {
	gin.SetMode(gin.TestMode)
	token := signedAuthToken(t, jwt.MapClaims{"sub": "1", "username": "alice"})
	h := NewHandler(&clients.Clients{
		Feed: &fakeFeedClient{latest: []*feedpb.FeedItem{
			{Id: 1, AuthorId: 10, Title: "other"},
			{Id: 2, AuthorId: 20, Title: "followed first"},
			{Id: 3, AuthorId: 30, Title: "other again"},
			{Id: 4, AuthorId: 20, Title: "followed second"},
		}},
		User: &fakeUserClient{following: []*userpb.UserInfo{{Id: 20}}},
	}, "Authorization", "Bearer", testJWTSecret)

	c, recorder := newFeedContext("/api/v1/feed?sort=follow&limit=1&offset=1", token)
	h.feedArticles(c)

	require.Equal(t, stdhttp.StatusOK, recorder.Code)
	var envelope struct {
		Data feedpb.FeedListResponse `json:"data"`
	}
	require.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &envelope))
	require.Len(t, envelope.Data.Items, 1)
	require.Equal(t, int64(4), envelope.Data.Items[0].GetId())
	require.Equal(t, int64(20), envelope.Data.Items[0].GetAuthorId())
}

func TestFeedArticlesFollowingExcludesMutedAndBlockedAuthors(t *testing.T) {
	gin.SetMode(gin.TestMode)
	token := signedAuthToken(t, jwt.MapClaims{"sub": "1", "username": "alice"})
	h := NewHandler(&clients.Clients{
		Feed: &fakeFeedClient{latest: []*feedpb.FeedItem{
			{Id: 1, AuthorId: 20, Title: "visible"},
			{Id: 2, AuthorId: 30, Title: "muted"},
			{Id: 3, AuthorId: 40, Title: "blocked"},
		}},
		User:       &fakeUserClient{following: []*userpb.UserInfo{{Id: 20}, {Id: 30}, {Id: 40}}},
		UserSafety: &fakeUserSafetyClient{muted: []*userpb.UserInfo{{Id: 30}}, blocked: []*userpb.UserInfo{{Id: 40}}},
	}, "Authorization", "Bearer", testJWTSecret)

	c, recorder := newFeedContext("/api/v1/feed?sort=follow&limit=10", token)
	h.feedArticles(c)

	require.Equal(t, stdhttp.StatusOK, recorder.Code)
	var envelope struct {
		Data feedpb.FeedListResponse `json:"data"`
	}
	require.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &envelope))
	require.Len(t, envelope.Data.Items, 1)
	require.Equal(t, int64(20), envelope.Data.Items[0].GetAuthorId())
}

func TestFeedArticlesFollowingRequiresAuth(t *testing.T) {
	gin.SetMode(gin.TestMode)
	h := NewHandler(nil, "Authorization", "Bearer", testJWTSecret)

	c, recorder := newFeedContext("/api/v1/feed?sort=follow", "")
	h.feedArticles(c)

	require.Equal(t, stdhttp.StatusUnauthorized, recorder.Code)
}

func TestFeedArticlesActiveUsesActiveFeed(t *testing.T) {
	gin.SetMode(gin.TestMode)
	h := NewHandler(&clients.Clients{
		Feed: &fakeFeedClient{
			latest: []*feedpb.FeedItem{{Id: 1, Title: "latest"}},
			active: []*feedpb.FeedItem{{Id: 2, Title: "active"}},
		},
	}, "Authorization", "Bearer", testJWTSecret)

	c, recorder := newFeedContext("/api/v1/feed?sort=active&limit=1", "")
	h.feedArticles(c)

	require.Equal(t, stdhttp.StatusOK, recorder.Code)
	var envelope struct {
		Data feedpb.FeedListResponse `json:"data"`
	}
	require.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &envelope))
	require.Len(t, envelope.Data.Items, 1)
	require.Equal(t, int64(2), envelope.Data.Items[0].GetId())
}

func TestEnsureCurrentUserCanPostAllowsUnverifiedWhenEmailGateDisabled(t *testing.T) {
	gin.SetMode(gin.TestMode)
	h := NewHandler(&clients.Clients{
		Admin: fakeAuthSettingsAdminClient{items: []*adminpb.SettingInfo{authSetting("auth.email_verification.required", "false")}},
		User:  &fakeUserClient{userResponse: &userpb.UserResponse{User: &userpb.UserInfo{Id: 1, Status: userStatusActive}}},
	}, "Authorization", "Bearer", testJWTSecret)

	c, _ := newFeedContext("/api/v1/topics", "")
	c.Set("user_id", int64(1))

	require.True(t, h.ensureCurrentUserCanPost(c, context.Background()))
}

func TestEnsureCurrentUserCanPostRejectsUnverifiedWhenEmailGateEnabled(t *testing.T) {
	gin.SetMode(gin.TestMode)
	h := NewHandler(&clients.Clients{
		Admin: fakeAuthSettingsAdminClient{items: []*adminpb.SettingInfo{authSetting("auth.email_verification.required", "true")}},
		User:  &fakeUserClient{userResponse: &userpb.UserResponse{User: &userpb.UserInfo{Id: 1, Status: userStatusActive}}},
	}, "Authorization", "Bearer", testJWTSecret)

	c, recorder := newFeedContext("/api/v1/topics", "")
	c.Set("user_id", int64(1))

	require.False(t, h.ensureCurrentUserCanPost(c, context.Background()))
	require.Equal(t, stdhttp.StatusForbidden, recorder.Code)
	require.Contains(t, recorder.Body.String(), "email_not_verified")
}

func TestEnsureCurrentUserCanPostAllowsVerifiedWhenEmailGateEnabled(t *testing.T) {
	gin.SetMode(gin.TestMode)
	h := NewHandler(&clients.Clients{
		Admin: fakeAuthSettingsAdminClient{items: []*adminpb.SettingInfo{authSetting("auth.email_verification.required", "true")}},
		User:  &fakeUserClient{userResponse: &userpb.UserResponse{User: &userpb.UserInfo{Id: 1, Status: userStatusActive, EmailVerified: true}}},
	}, "Authorization", "Bearer", testJWTSecret)

	c, _ := newFeedContext("/api/v1/topics", "")
	c.Set("user_id", int64(1))

	require.True(t, h.ensureCurrentUserCanPost(c, context.Background()))
}

func newFeedContext(rawURL string, token string) (*gin.Context, *httptest.ResponseRecorder) {
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	req := httptest.NewRequest(stdhttp.MethodGet, rawURL, nil)
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	c.Request = req
	return c, recorder
}

type fakeFeedClient struct {
	latest []*feedpb.FeedItem
	active []*feedpb.FeedItem
}

type fakeUserSafetyClient struct {
	userpb.UserServiceClient
	blocked []*userpb.UserInfo
	muted   []*userpb.UserInfo
}

func (f *fakeUserSafetyClient) ListBlockedUsers(context.Context, *userpb.ListUserRelationsRequest, ...grpc.CallOption) (*userpb.UserListResponse, error) {
	return &userpb.UserListResponse{Items: f.blocked, Total: int64(len(f.blocked))}, nil
}

func (f *fakeUserSafetyClient) ListMutedUsers(context.Context, *userpb.ListUserRelationsRequest, ...grpc.CallOption) (*userpb.UserListResponse, error) {
	return &userpb.UserListResponse{Items: f.muted, Total: int64(len(f.muted))}, nil
}

func (f *fakeFeedClient) ListLatest(_ context.Context, in *feedpb.ListFeedRequest, _ ...grpc.CallOption) (*feedpb.FeedListResponse, error) {
	items := f.latest
	if authorIDs := in.GetAuthorIds(); len(authorIDs) > 0 {
		allowed := make(map[int64]struct{}, len(authorIDs))
		for _, authorID := range authorIDs {
			allowed[authorID] = struct{}{}
		}
		items = make([]*feedpb.FeedItem, 0, len(f.latest))
		for _, item := range f.latest {
			if item == nil {
				continue
			}
			if _, ok := allowed[item.GetAuthorId()]; ok {
				items = append(items, item)
			}
		}
	}
	return feedPage(items, in), nil
}

func (f *fakeFeedClient) ListHot(ctx context.Context, in *feedpb.ListFeedRequest, opts ...grpc.CallOption) (*feedpb.FeedListResponse, error) {
	return f.ListLatest(ctx, in, opts...)
}

func (f *fakeFeedClient) ListActive(_ context.Context, in *feedpb.ListFeedRequest, _ ...grpc.CallOption) (*feedpb.FeedListResponse, error) {
	return feedPage(f.active, in), nil
}

func feedPage(items []*feedpb.FeedItem, in *feedpb.ListFeedRequest) *feedpb.FeedListResponse {
	start := int(in.GetOffset())
	if start >= len(items) {
		return &feedpb.FeedListResponse{Items: []*feedpb.FeedItem{}}
	}
	limit := int(in.GetLimit())
	if limit <= 0 {
		limit = 20
	}
	end := start + limit
	if end > len(items) {
		end = len(items)
	}
	return &feedpb.FeedListResponse{Items: items[start:end]}
}

type fakeUserClient struct {
	following                 []*userpb.UserInfo
	users                     []*userpb.UserInfo
	userResponse              *userpb.UserResponse
	listUsersReq              *userpb.ListUsersRequest
	listUsersCalls            int
	passwordResetResponse     *userpb.PasswordResetResponse
	passwordResetErr          error
	emailVerificationResponse *userpb.EmailVerificationResponse
	emailVerificationErr      error
}

func (f *fakeUserClient) ListFollowing(_ context.Context, in *userpb.ListFollowsRequest, _ ...grpc.CallOption) (*userpb.UserListResponse, error) {
	page := int(in.GetPage())
	if page <= 0 {
		page = 1
	}
	pageSize := int(in.GetPageSize())
	if pageSize <= 0 {
		pageSize = 20
	}
	start := (page - 1) * pageSize
	if start >= len(f.following) {
		return &userpb.UserListResponse{Items: []*userpb.UserInfo{}, Total: int64(len(f.following))}, nil
	}
	end := start + pageSize
	if end > len(f.following) {
		end = len(f.following)
	}
	return &userpb.UserListResponse{Items: f.following[start:end], Total: int64(len(f.following))}, nil
}

func (f *fakeUserClient) Register(context.Context, *userpb.RegisterRequest, ...grpc.CallOption) (*userpb.AuthResponse, error) {
	return nil, nil
}

func (f *fakeUserClient) Login(context.Context, *userpb.LoginRequest, ...grpc.CallOption) (*userpb.AuthResponse, error) {
	return nil, nil
}

func (f *fakeUserClient) OAuthLogin(context.Context, *userpb.OAuthLoginRequest, ...grpc.CallOption) (*userpb.AuthResponse, error) {
	return nil, nil
}

func (f *fakeUserClient) WebmasterLogin(context.Context, *userpb.WebmasterLoginRequest, ...grpc.CallOption) (*userpb.AuthResponse, error) {
	return nil, nil
}

func (f *fakeUserClient) ListUsers(_ context.Context, req *userpb.ListUsersRequest, _ ...grpc.CallOption) (*userpb.UserListResponse, error) {
	f.listUsersCalls++
	f.listUsersReq = req
	return &userpb.UserListResponse{Items: f.users, Total: int64(len(f.users))}, nil
}

func (f *fakeUserClient) GetUser(context.Context, *userpb.UserIDRequest, ...grpc.CallOption) (*userpb.UserResponse, error) {
	return f.userResponse, nil
}

func (f *fakeUserClient) GetUserByUsername(context.Context, *userpb.UsernameRequest, ...grpc.CallOption) (*userpb.UserResponse, error) {
	return nil, nil
}

func (f *fakeUserClient) UpdateProfile(context.Context, *userpb.UpdateProfileRequest, ...grpc.CallOption) (*userpb.UserResponse, error) {
	return nil, nil
}

func (f *fakeUserClient) UpdateStatus(context.Context, *userpb.UpdateStatusRequest, ...grpc.CallOption) (*userpb.UserResponse, error) {
	return nil, nil
}

func (f *fakeUserClient) ChangePassword(context.Context, *userpb.ChangePasswordRequest, ...grpc.CallOption) (*userpb.SimpleResponse, error) {
	return nil, nil
}

func (f *fakeUserClient) RequestPasswordReset(context.Context, *userpb.PasswordResetRequest, ...grpc.CallOption) (*userpb.PasswordResetResponse, error) {
	return f.passwordResetResponse, f.passwordResetErr
}

func (f *fakeUserClient) ResetPassword(context.Context, *userpb.ResetPasswordRequest, ...grpc.CallOption) (*userpb.SimpleResponse, error) {
	return nil, nil
}

func (f *fakeUserClient) RequestEmailVerification(context.Context, *userpb.EmailVerificationRequest, ...grpc.CallOption) (*userpb.EmailVerificationResponse, error) {
	return f.emailVerificationResponse, f.emailVerificationErr
}

func (f *fakeUserClient) VerifyEmail(context.Context, *userpb.VerifyEmailRequest, ...grpc.CallOption) (*userpb.UserResponse, error) {
	return nil, nil
}

func (f *fakeUserClient) Follow(context.Context, *userpb.FollowRequest, ...grpc.CallOption) (*userpb.SimpleResponse, error) {
	return nil, nil
}

func (f *fakeUserClient) Unfollow(context.Context, *userpb.FollowRequest, ...grpc.CallOption) (*userpb.SimpleResponse, error) {
	return nil, nil
}

func (f *fakeUserClient) IsFollowing(context.Context, *userpb.FollowRequest, ...grpc.CallOption) (*userpb.IsFollowingResponse, error) {
	return nil, nil
}

func (f *fakeUserClient) ListFollowers(context.Context, *userpb.ListFollowsRequest, ...grpc.CallOption) (*userpb.UserListResponse, error) {
	return nil, nil
}
