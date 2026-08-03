package http

import (
	"context"
	"encoding/json"
	stdhttp "net/http"
	"net/http/httptest"
	"net/url"
	"testing"
	"time"

	"api-gateway/api/proto/adminpb"
	"api-gateway/api/proto/userpb"
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
			RegisterMode      string `json:"register_mode"`
			InviteRequired    bool   `json:"invite_required"`
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
	require.False(t, envelope.Data.RegisterEnabled)
	require.Equal(t, "open", envelope.Data.RegisterMode)
	require.False(t, envelope.Data.InviteRequired)
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

func TestIsAllowedReturnToKeepsAConfiguredCallbackQuery(t *testing.T) {
	settings := authSettings{
		"auth.oauth.frontend_callback_url": "https://bbs.example.com/auth/callback",
	}

	require.True(t, isAllowedReturnTo(settings, "https://bbs.example.com/auth/callback?redirect=%2Froom%2FAB12CD3E"))
	require.True(t, isAllowedReturnTo(settings, "https://bbs.example.com/auth/callback/?redirect=%2Froom%2FAB12CD3E"))
	require.False(t, isAllowedReturnTo(settings, "https://evil.example.com/auth/callback?redirect=%2Froom%2FAB12CD3E"))
	require.False(t, isAllowedReturnTo(settings, "https://bbs.example.com/other?redirect=%2Froom%2FAB12CD3E"))
}

func TestOAuthReturnToRequiresConfiguredCallback(t *testing.T) {
	settings := authSettings{}

	require.Empty(t, oauthReturnToFallback(settings))
	require.Empty(t, allowedOAuthReturnTo(settings, "https://evil.example.com/auth/callback"))
}

func TestOAuthStartBindsStateNonceToHttpOnlyLaxCookie(t *testing.T) {
	gin.SetMode(gin.TestMode)
	h := newOAuthStateTestHandler()
	c, recorder := newAuthConfigContext("https://api.example.com/api/v1/auth/oauth/github/start?redirect=https%3A%2F%2Fbbs.example.com%2Fauth%2Fcallback")
	c.Params = gin.Params{{Key: "provider", Value: "github"}}
	c.Request.Header.Set("X-Forwarded-Proto", "https")

	h.oauthStart(c)

	require.Equal(t, stdhttp.StatusFound, recorder.Code)
	redirectURL, err := url.Parse(recorder.Header().Get("Location"))
	require.NoError(t, err)
	state := redirectURL.Query().Get("state")
	require.NotEmpty(t, state)
	returnTo, nonce, err := h.verifyOAuthState(state, "github")
	require.NoError(t, err)
	require.Equal(t, "https://bbs.example.com/auth/callback", returnTo)

	cookie := findOAuthStateCookie(t, recorder)
	require.Equal(t, nonce, cookie.Value)
	require.Equal(t, oauthStateCookiePath, cookie.Path)
	require.Equal(t, int(oauthStateTTL.Seconds()), cookie.MaxAge)
	require.True(t, cookie.HttpOnly)
	require.Equal(t, stdhttp.SameSiteLaxMode, cookie.SameSite)
	require.True(t, cookie.Secure)
}

func TestOAuthStartUsesConfiguredPublicBaseURL(t *testing.T) {
	gin.SetMode(gin.TestMode)
	h := newOAuthStateTestHandler()
	h.SetPublicBaseURL("https://bbs.example.com")
	c, recorder := newAuthConfigContext("http://gateway.internal/api/v1/auth/oauth/github/start")
	c.Params = gin.Params{{Key: "provider", Value: "github"}}
	c.Request.Header.Set("X-Forwarded-Host", "attacker.example")
	c.Request.Header.Set("X-Forwarded-Proto", "http")

	h.oauthStart(c)

	require.Equal(t, stdhttp.StatusFound, recorder.Code)
	redirectURL, err := url.Parse(recorder.Header().Get("Location"))
	require.NoError(t, err)
	require.Equal(t, "https://bbs.example.com/api/v1/auth/oauth/github/callback", redirectURL.Query().Get("redirect_uri"))
}

func TestOAuthCallbackRejectsMismatchedStateCookieAndClearsIt(t *testing.T) {
	gin.SetMode(gin.TestMode)
	h := newOAuthStateTestHandler()
	state, _, err := h.signOAuthState("github", "https://bbs.example.com/auth/callback")
	require.NoError(t, err)

	c, recorder := newAuthConfigContext("https://api.example.com/api/v1/auth/oauth/github/callback?state=" + url.QueryEscape(state))
	c.Params = gin.Params{{Key: "provider", Value: "github"}}
	c.Request.AddCookie(&stdhttp.Cookie{Name: oauthStateCookieName, Value: "wrong-nonce"})

	h.oauthCallback(c)

	require.Equal(t, stdhttp.StatusBadRequest, recorder.Code)
	assertOAuthStateCookieCleared(t, recorder)
}

func TestOAuthCallbackConsumesMatchingStateCookieBeforeProviderErrorRedirect(t *testing.T) {
	gin.SetMode(gin.TestMode)
	h := newOAuthStateTestHandler()
	state, nonce, err := h.signOAuthState("github", "https://bbs.example.com/auth/callback?redirect=%2Fchat")
	require.NoError(t, err)

	c, recorder := newAuthConfigContext("https://api.example.com/api/v1/auth/oauth/github/callback?state=" + url.QueryEscape(state) + "&error=access_denied")
	c.Params = gin.Params{{Key: "provider", Value: "github"}}
	c.Request.AddCookie(&stdhttp.Cookie{Name: oauthStateCookieName, Value: nonce})

	h.oauthCallback(c)

	require.Equal(t, stdhttp.StatusFound, recorder.Code)
	redirectURL, err := url.Parse(recorder.Header().Get("Location"))
	require.NoError(t, err)
	require.Equal(t, "/chat", redirectURL.Query().Get("redirect"))
	fragment, err := url.ParseQuery(redirectURL.Fragment)
	require.NoError(t, err)
	require.Equal(t, "access_denied", fragment.Get("error"))
	assertOAuthStateCookieCleared(t, recorder)
}

func TestOAuthRedirectWithAuthPreservesCallbackQuery(t *testing.T) {
	target := oauthRedirectWithAuth(
		"https://bbs.example.com/auth/callback?redirect=%2Froom%2FAB12CD3E",
		&userpb.AuthResponse{AccessToken: "access-token", ExpiresAt: 1784025000},
	)
	u, err := url.Parse(target)
	require.NoError(t, err)
	require.Equal(t, "/room/AB12CD3E", u.Query().Get("redirect"))
	fragment, err := url.ParseQuery(u.Fragment)
	require.NoError(t, err)
	require.Equal(t, "access-token", fragment.Get("access_token"))
	require.Equal(t, "1784025000", fragment.Get("expires_at"))
}

func TestOAuthRedirectWithAuthCarriesMFAChallengeWithoutAccessToken(t *testing.T) {
	target := oauthRedirectWithAuth(
		"https://bbs.example.com/auth/callback?redirect=%2Fchat",
		&userpb.AuthResponse{
			MfaRequired:  true,
			MfaChallenge: "oauth-mfa-challenge",
			MfaExpiresAt: 1784025300,
		},
	)
	u, err := url.Parse(target)
	require.NoError(t, err)
	require.Equal(t, "/chat", u.Query().Get("redirect"))
	fragment, err := url.ParseQuery(u.Fragment)
	require.NoError(t, err)
	require.Equal(t, "true", fragment.Get("mfa_required"))
	require.Equal(t, "oauth-mfa-challenge", fragment.Get("mfa_challenge"))
	require.Equal(t, "1784025300", fragment.Get("mfa_expires_at"))
	require.Equal(t, "mfa_required", fragment.Get("status"))
	require.Empty(t, fragment.Get("access_token"))
	require.Empty(t, fragment.Get("expires_at"))
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

func newOAuthStateTestHandler() *Handler {
	return NewHandler(&clients.Clients{
		Admin: fakeAuthSettingsAdminClient{items: []*adminpb.SettingInfo{
			authSetting("auth.oauth.frontend_callback_url", "https://bbs.example.com/auth/callback"),
			authSetting("auth.github.enabled", "true"),
			authSetting("auth.github.client_id", "github-id"),
			authSetting("auth.github.client_secret", "github-secret"),
		}},
	}, "Authorization", "Bearer", testJWTSecret)
}

func findOAuthStateCookie(t *testing.T, recorder *httptest.ResponseRecorder) *stdhttp.Cookie {
	t.Helper()
	for _, cookie := range recorder.Result().Cookies() {
		if cookie.Name == oauthStateCookieName {
			return cookie
		}
	}
	t.Fatalf("oauth state cookie not found")
	return nil
}

func assertOAuthStateCookieCleared(t *testing.T, recorder *httptest.ResponseRecorder) {
	t.Helper()
	cookie := findOAuthStateCookie(t, recorder)
	require.Empty(t, cookie.Value)
	require.Equal(t, -1, cookie.MaxAge)
	require.Equal(t, oauthStateCookiePath, cookie.Path)
	require.True(t, cookie.HttpOnly)
	require.Equal(t, stdhttp.SameSiteLaxMode, cookie.SameSite)
}

type fakeAuthSettingsAdminClient struct {
	adminpb.AdminServiceClient
	items []*adminpb.SettingInfo
}

func (f fakeAuthSettingsAdminClient) ListAuthSettings(context.Context, *adminpb.ListAuthSettingsRequest, ...grpc.CallOption) (*adminpb.SettingListResponse, error) {
	return &adminpb.SettingListResponse{Items: f.items, Total: int64(len(f.items))}, nil
}
