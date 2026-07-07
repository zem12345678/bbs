package http

import (
	"context"
	"encoding/json"
	stdhttp "net/http"
	"net/http/httptest"
	"testing"
	"time"

	"api-gateway/api/proto/adminpb"
	"api-gateway/internal/clients"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
	"golang.org/x/crypto/bcrypt"
	"google.golang.org/grpc"
)

func TestGitHubAccountMeetsMinAge(t *testing.T) {
	now := time.Date(2026, 7, 6, 12, 0, 0, 0, time.UTC)
	if !githubAccountMeetsMinAge(time.Date(2023, 7, 6, 11, 59, 0, 0, time.UTC), 3, now) {
		t.Fatalf("expected account older than three years to pass")
	}
	if githubAccountMeetsMinAge(time.Date(2023, 7, 7, 0, 0, 0, 0, time.UTC), 3, now) {
		t.Fatalf("expected account younger than three years to fail")
	}
	if githubAccountMeetsMinAge(time.Time{}, 3, now) {
		t.Fatalf("expected missing created_at to fail")
	}
}

func TestWebmasterPasswordMatches(t *testing.T) {
	if !webmasterPasswordMatches("webmaster123", "webmaster123") {
		t.Fatalf("expected plaintext compatibility match")
	}
	if webmasterPasswordMatches("webmaster123", "wrong") {
		t.Fatalf("expected plaintext mismatch")
	}
	hash, err := bcrypt.GenerateFromPassword([]byte("webmaster123"), bcrypt.DefaultCost)
	if err != nil {
		t.Fatalf("hash password: %v", err)
	}
	if !webmasterPasswordMatches(string(hash), "webmaster123") {
		t.Fatalf("expected bcrypt match")
	}
	if webmasterPasswordMatches(string(hash), "wrong") {
		t.Fatalf("expected bcrypt mismatch")
	}
}

func TestAuthConfigRequiresEnabledProviderWithCredentials(t *testing.T) {
	gin.SetMode(gin.TestMode)
	h := NewHandler(&clients.Clients{
		Admin: fakeAuthSettingsAdminClient{items: []*adminpb.SettingInfo{
			authSetting("auth.password.enabled", "false"),
			authSetting("auth.register.enabled", "true"),
			authSetting("auth.oauth.frontend_callback_url", "https://bbs.example.com/auth/callback"),
			authSetting("auth.github.enabled", "true"),
			authSetting("auth.github.client_id", "github-id"),
			authSetting("auth.github.client_secret", "github-secret"),
			authSetting("auth.github.min_account_years", "4"),
			authSetting("auth.google.enabled", "true"),
			authSetting("auth.google.client_id", "google-id"),
			authSetting("auth.google.client_secret", ""),
			authSetting("auth.qq.enabled", "false"),
			authSetting("auth.qq.client_id", "qq-id"),
			authSetting("auth.qq.client_secret", "qq-secret"),
			authSetting("site.webmaster.username", "owner"),
			authSetting("site.webmaster.password", "owner-secret"),
		}},
	}, "Authorization", "Bearer", testJWTSecret)

	c, recorder := newAuthConfigContext("https://api.example.com/api/v1/auth/config")
	h.authConfig(c)

	require.Equal(t, stdhttp.StatusOK, recorder.Code)
	var envelope struct {
		Data struct {
			OAuthCallbackHint string `json:"oauth_callback_hint"`
			PasswordEnabled   bool   `json:"password_enabled"`
			RegisterEnabled   bool   `json:"register_enabled"`
			WebmasterEnabled  bool   `json:"webmaster_enabled"`
			Providers         []struct {
				Enabled         bool   `json:"enabled"`
				Label           string `json:"label"`
				MinAccountYears int    `json:"min_account_years"`
				Provider        string `json:"provider"`
				StartURL        string `json:"start_url"`
			} `json:"providers"`
		} `json:"data"`
	}
	require.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &envelope))
	require.False(t, envelope.Data.PasswordEnabled)
	require.True(t, envelope.Data.RegisterEnabled)
	require.True(t, envelope.Data.WebmasterEnabled)
	require.Equal(t, "https://bbs.example.com/auth/callback", envelope.Data.OAuthCallbackHint)

	providers := map[string]struct {
		Enabled         bool
		Label           string
		MinAccountYears int
		StartURL        string
	}{}
	for _, provider := range envelope.Data.Providers {
		providers[provider.Provider] = struct {
			Enabled         bool
			Label           string
			MinAccountYears int
			StartURL        string
		}{
			Enabled:         provider.Enabled,
			Label:           provider.Label,
			MinAccountYears: provider.MinAccountYears,
			StartURL:        provider.StartURL,
		}
	}
	require.Len(t, providers, 3)
	require.True(t, providers["github"].Enabled)
	require.Equal(t, "GitHub", providers["github"].Label)
	require.Equal(t, 4, providers["github"].MinAccountYears)
	require.Equal(t, "https://api.example.com/api/v1/auth/oauth/github/start", providers["github"].StartURL)
	require.False(t, providers["google"].Enabled)
	require.Equal(t, "Google", providers["google"].Label)
	require.Equal(t, "https://api.example.com/api/v1/auth/oauth/google/start", providers["google"].StartURL)
	require.False(t, providers["qq"].Enabled)
	require.Equal(t, "QQ", providers["qq"].Label)
	require.Equal(t, "https://api.example.com/api/v1/auth/oauth/qq/start", providers["qq"].StartURL)
}

func authSetting(key string, value string) *adminpb.SettingInfo {
	return &adminpb.SettingInfo{Key: key, Value: value}
}

func newAuthConfigContext(rawURL string) (*gin.Context, *httptest.ResponseRecorder) {
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest(stdhttp.MethodGet, rawURL, nil)
	return c, recorder
}

type fakeAuthSettingsAdminClient struct {
	adminpb.AdminServiceClient
	items []*adminpb.SettingInfo
}

func (f fakeAuthSettingsAdminClient) ListAuthSettings(context.Context, *adminpb.ListAuthSettingsRequest, ...grpc.CallOption) (*adminpb.SettingListResponse, error) {
	return &adminpb.SettingListResponse{Items: f.items, Total: int64(len(f.items))}, nil
}
