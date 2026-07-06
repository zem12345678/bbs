package http

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	stdhttp "net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"api-gateway/internal/clients/pb/adminpb"
	"api-gateway/internal/clients/pb/userpb"
	"api-gateway/pkg/http/response"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
)

const oauthStateTTL = 10 * time.Minute

type authSettings map[string]string

type oauthProviderConfig struct {
	Provider        string
	ClientID        string
	ClientSecret    string
	Enabled         bool
	MinAccountYears int
}

type oauthUserProfile struct {
	Provider       string
	ProviderUserID string
	Username       string
	Email          string
	Nickname       string
	AvatarURL      string
	CreatedAt      time.Time
}

type oauthStateClaims struct {
	Provider string `json:"provider"`
	ReturnTo string `json:"return_to"`
	Nonce    string `json:"nonce"`
	jwt.RegisteredClaims
}

func (h *Handler) authConfig(c *gin.Context) {
	ctx, cancel := rpcContext(c)
	defer cancel()
	settings, err := h.loadAuthSettings(ctx, true)
	if err != nil {
		writeRPCError(c, err)
		return
	}
	providers := make([]gin.H, 0, 3)
	for _, provider := range []string{"github", "qq", "google"} {
		cfg := providerConfig(settings, provider)
		item := gin.H{
			"provider":  provider,
			"label":     providerLabel(provider),
			"enabled":   cfg.Enabled && cfg.ClientID != "" && cfg.ClientSecret != "",
			"start_url": publicRequestURL(c, "/api/v1/auth/oauth/"+provider+"/start"),
		}
		if provider == "github" {
			item["min_account_years"] = cfg.MinAccountYears
		}
		providers = append(providers, item)
	}
	response.Success(c, gin.H{
		"password_enabled":    settingBool(settings, "auth.password.enabled", true),
		"register_enabled":    settingBool(settings, "auth.register.enabled", true),
		"webmaster_enabled":   strings.TrimSpace(settings["site.webmaster.username"]) != "" && strings.TrimSpace(settings["site.webmaster.password"]) != "",
		"oauth_callback_hint": oauthReturnToFallback(c, settings),
		"providers":           providers,
	})
}

func (h *Handler) oauthStart(c *gin.Context) {
	provider := normalizeOAuthProvider(c.Param("provider"))
	if provider == "" {
		writeError(c, stdhttp.StatusBadRequest, "unsupported oauth provider", "bad_request")
		return
	}
	ctx, cancel := rpcContext(c)
	defer cancel()
	settings, err := h.loadAuthSettings(ctx, true)
	if err != nil {
		writeRPCError(c, err)
		return
	}
	cfg := providerConfig(settings, provider)
	if !cfg.Enabled || cfg.ClientID == "" || cfg.ClientSecret == "" {
		writeError(c, stdhttp.StatusForbidden, "oauth provider disabled", "permission_denied")
		return
	}
	returnTo := allowedOAuthReturnTo(c, settings, c.Query("redirect"))
	state, err := h.signOAuthState(provider, returnTo)
	if err != nil {
		writeError(c, stdhttp.StatusInternalServerError, "create oauth state failed", "internal_error")
		return
	}
	redirectURI := publicRequestURL(c, "/api/v1/auth/oauth/"+provider+"/callback")
	authURL, err := buildOAuthAuthorizeURL(provider, cfg.ClientID, redirectURI, state)
	if err != nil {
		writeError(c, stdhttp.StatusBadRequest, err.Error(), "bad_request")
		return
	}
	c.Redirect(stdhttp.StatusFound, authURL)
}

func (h *Handler) oauthCallback(c *gin.Context) {
	provider := normalizeOAuthProvider(c.Param("provider"))
	if provider == "" {
		writeError(c, stdhttp.StatusBadRequest, "unsupported oauth provider", "bad_request")
		return
	}
	returnTo, err := h.verifyOAuthState(c.Query("state"), provider)
	if err != nil {
		writeError(c, stdhttp.StatusBadRequest, "invalid oauth state", "bad_request")
		return
	}
	if providerErr := strings.TrimSpace(c.Query("error")); providerErr != "" {
		c.Redirect(stdhttp.StatusFound, oauthRedirectWithError(returnTo, providerErr))
		return
	}
	code := strings.TrimSpace(c.Query("code"))
	if code == "" {
		c.Redirect(stdhttp.StatusFound, oauthRedirectWithError(returnTo, "missing oauth code"))
		return
	}

	ctx, cancel := rpcContext(c)
	defer cancel()
	settings, err := h.loadAuthSettings(ctx, true)
	if err != nil {
		c.Redirect(stdhttp.StatusFound, oauthRedirectWithError(returnTo, "auth settings unavailable"))
		return
	}
	cfg := providerConfig(settings, provider)
	if !cfg.Enabled || cfg.ClientID == "" || cfg.ClientSecret == "" {
		c.Redirect(stdhttp.StatusFound, oauthRedirectWithError(returnTo, "oauth provider not configured"))
		return
	}
	redirectURI := publicRequestURL(c, "/api/v1/auth/oauth/"+provider+"/callback")
	accessToken, err := exchangeOAuthCode(ctx, provider, cfg, redirectURI, code)
	if err != nil {
		c.Redirect(stdhttp.StatusFound, oauthRedirectWithError(returnTo, "oauth token exchange failed"))
		return
	}
	profile, err := fetchOAuthProfile(ctx, provider, cfg, accessToken)
	if err != nil {
		c.Redirect(stdhttp.StatusFound, oauthRedirectWithError(returnTo, "oauth profile fetch failed"))
		return
	}
	if provider == "github" && !githubAccountMeetsMinAge(profile.CreatedAt, cfg.MinAccountYears, time.Now()) {
		c.Redirect(stdhttp.StatusFound, oauthRedirectWithError(returnTo, fmt.Sprintf("github account must be at least %d years old", cfg.MinAccountYears)))
		return
	}
	resp, err := h.clients.User.OAuthLogin(ctx, &userpb.OAuthLoginRequest{
		Provider:       profile.Provider,
		ProviderUserId: profile.ProviderUserID,
		Username:       profile.Username,
		Email:          profile.Email,
		Nickname:       profile.Nickname,
		AvatarUrl:      profile.AvatarURL,
	})
	if err != nil {
		c.Redirect(stdhttp.StatusFound, oauthRedirectWithError(returnTo, "community login failed"))
		return
	}
	c.Redirect(stdhttp.StatusFound, oauthRedirectWithAuth(returnTo, resp))
}

func (h *Handler) tryWebmasterLogin(c *gin.Context, ctx context.Context, req loginRequest) (*userpb.AuthResponse, bool) {
	settings, err := h.loadAuthSettings(ctx, true)
	if err != nil {
		return nil, false
	}
	username := strings.ToLower(strings.TrimSpace(settings["site.webmaster.username"]))
	password := strings.TrimSpace(settings["site.webmaster.password"])
	account := strings.ToLower(strings.TrimSpace(req.Account))
	if username == "" || password == "" || account != username {
		return nil, false
	}
	if req.Password != password {
		writeError(c, stdhttp.StatusUnauthorized, "invalid password", "unauthorized")
		return nil, true
	}
	resp, err := h.clients.User.WebmasterLogin(ctx, &userpb.WebmasterLoginRequest{
		Username: username,
		Password: password,
		Nickname: "Webmaster",
	})
	if err != nil {
		writeRPCError(c, err)
		return nil, true
	}
	return resp, true
}

func (h *Handler) passwordLoginEnabled(ctx context.Context) bool {
	settings, err := h.loadAuthSettings(ctx, false)
	if err != nil {
		return true
	}
	return settingBool(settings, "auth.password.enabled", true)
}

func (h *Handler) registrationEnabled(ctx context.Context) bool {
	settings, err := h.loadAuthSettings(ctx, false)
	if err != nil {
		return true
	}
	return settingBool(settings, "auth.password.enabled", true) && settingBool(settings, "auth.register.enabled", true)
}

func (h *Handler) loadAuthSettings(ctx context.Context, includeSecrets bool) (authSettings, error) {
	resp, err := h.clients.Admin.ListAuthSettings(ctx, &adminpb.ListAuthSettingsRequest{IncludeSecrets: includeSecrets})
	if err != nil {
		return nil, err
	}
	out := authSettings{}
	for _, item := range resp.GetItems() {
		out[strings.ToLower(strings.TrimSpace(item.GetKey()))] = item.GetValue()
	}
	return out, nil
}

func providerConfig(settings authSettings, provider string) oauthProviderConfig {
	provider = normalizeOAuthProvider(provider)
	minYears := settingInt(settings, "auth.github.min_account_years", 3)
	if minYears <= 0 {
		minYears = 3
	}
	return oauthProviderConfig{
		Provider:        provider,
		ClientID:        strings.TrimSpace(settings["auth."+provider+".client_id"]),
		ClientSecret:    strings.TrimSpace(settings["auth."+provider+".client_secret"]),
		Enabled:         settingBool(settings, "auth."+provider+".enabled", false),
		MinAccountYears: minYears,
	}
}

func buildOAuthAuthorizeURL(provider string, clientID string, redirectURI string, state string) (string, error) {
	q := url.Values{}
	q.Set("client_id", clientID)
	q.Set("redirect_uri", redirectURI)
	q.Set("state", state)
	switch provider {
	case "github":
		q.Set("scope", "read:user user:email")
		return "https://github.com/login/oauth/authorize?" + q.Encode(), nil
	case "google":
		q.Set("response_type", "code")
		q.Set("scope", "openid email profile")
		q.Set("access_type", "online")
		q.Set("prompt", "select_account")
		return "https://accounts.google.com/o/oauth2/v2/auth?" + q.Encode(), nil
	case "qq":
		q.Set("response_type", "code")
		q.Set("scope", "get_user_info")
		return "https://graph.qq.com/oauth2.0/authorize?" + q.Encode(), nil
	default:
		return "", errors.New("unsupported oauth provider")
	}
}

func exchangeOAuthCode(ctx context.Context, provider string, cfg oauthProviderConfig, redirectURI string, code string) (string, error) {
	form := url.Values{}
	form.Set("client_id", cfg.ClientID)
	form.Set("client_secret", cfg.ClientSecret)
	form.Set("code", code)
	form.Set("redirect_uri", redirectURI)
	switch provider {
	case "github":
		req, err := stdhttp.NewRequestWithContext(ctx, stdhttp.MethodPost, "https://github.com/login/oauth/access_token", strings.NewReader(form.Encode()))
		if err != nil {
			return "", err
		}
		req.Header.Set("Accept", "application/json")
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		var body struct {
			AccessToken      string `json:"access_token"`
			Error            string `json:"error"`
			ErrorDescription string `json:"error_description"`
		}
		if err := doJSON(req, &body); err != nil {
			return "", err
		}
		if body.Error != "" {
			return "", errors.New(body.ErrorDescription)
		}
		return requireAccessToken(body.AccessToken)
	case "google":
		form.Set("grant_type", "authorization_code")
		req, err := stdhttp.NewRequestWithContext(ctx, stdhttp.MethodPost, "https://oauth2.googleapis.com/token", strings.NewReader(form.Encode()))
		if err != nil {
			return "", err
		}
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		var body struct {
			AccessToken      string `json:"access_token"`
			Error            string `json:"error"`
			ErrorDescription string `json:"error_description"`
		}
		if err := doJSON(req, &body); err != nil {
			return "", err
		}
		if body.Error != "" {
			return "", errors.New(body.ErrorDescription)
		}
		return requireAccessToken(body.AccessToken)
	case "qq":
		form.Set("grant_type", "authorization_code")
		req, err := stdhttp.NewRequestWithContext(ctx, stdhttp.MethodGet, "https://graph.qq.com/oauth2.0/token?"+form.Encode(), nil)
		if err != nil {
			return "", err
		}
		text, err := doText(req)
		if err != nil {
			return "", err
		}
		values, err := url.ParseQuery(text)
		if err != nil {
			return "", err
		}
		return requireAccessToken(values.Get("access_token"))
	default:
		return "", errors.New("unsupported oauth provider")
	}
}

func fetchOAuthProfile(ctx context.Context, provider string, cfg oauthProviderConfig, accessToken string) (oauthUserProfile, error) {
	switch provider {
	case "github":
		return fetchGitHubProfile(ctx, accessToken)
	case "google":
		return fetchGoogleProfile(ctx, accessToken)
	case "qq":
		return fetchQQProfile(ctx, cfg.ClientID, accessToken)
	default:
		return oauthUserProfile{}, errors.New("unsupported oauth provider")
	}
}

func fetchGitHubProfile(ctx context.Context, accessToken string) (oauthUserProfile, error) {
	req, err := stdhttp.NewRequestWithContext(ctx, stdhttp.MethodGet, "https://api.github.com/user", nil)
	if err != nil {
		return oauthUserProfile{}, err
	}
	req.Header.Set("Authorization", "Bearer "+accessToken)
	req.Header.Set("Accept", "application/vnd.github+json")
	var body struct {
		ID        int64  `json:"id"`
		Login     string `json:"login"`
		Name      string `json:"name"`
		Email     string `json:"email"`
		AvatarURL string `json:"avatar_url"`
		CreatedAt string `json:"created_at"`
	}
	if err := doJSON(req, &body); err != nil {
		return oauthUserProfile{}, err
	}
	email := strings.TrimSpace(body.Email)
	if email == "" {
		email = fetchGitHubPrimaryEmail(ctx, accessToken)
	}
	createdAt, _ := time.Parse(time.RFC3339, body.CreatedAt)
	return oauthUserProfile{
		Provider:       "github",
		ProviderUserID: strconv.FormatInt(body.ID, 10),
		Username:       body.Login,
		Email:          email,
		Nickname:       firstNonEmpty(body.Name, body.Login),
		AvatarURL:      body.AvatarURL,
		CreatedAt:      createdAt,
	}, nil
}

func fetchGitHubPrimaryEmail(ctx context.Context, accessToken string) string {
	req, err := stdhttp.NewRequestWithContext(ctx, stdhttp.MethodGet, "https://api.github.com/user/emails", nil)
	if err != nil {
		return ""
	}
	req.Header.Set("Authorization", "Bearer "+accessToken)
	req.Header.Set("Accept", "application/vnd.github+json")
	var emails []struct {
		Email    string `json:"email"`
		Primary  bool   `json:"primary"`
		Verified bool   `json:"verified"`
	}
	if err := doJSON(req, &emails); err != nil {
		return ""
	}
	for _, item := range emails {
		if item.Primary && item.Verified {
			return item.Email
		}
	}
	return ""
}

func fetchGoogleProfile(ctx context.Context, accessToken string) (oauthUserProfile, error) {
	req, err := stdhttp.NewRequestWithContext(ctx, stdhttp.MethodGet, "https://www.googleapis.com/oauth2/v3/userinfo", nil)
	if err != nil {
		return oauthUserProfile{}, err
	}
	req.Header.Set("Authorization", "Bearer "+accessToken)
	var body struct {
		Sub     string `json:"sub"`
		Email   string `json:"email"`
		Name    string `json:"name"`
		Picture string `json:"picture"`
	}
	if err := doJSON(req, &body); err != nil {
		return oauthUserProfile{}, err
	}
	return oauthUserProfile{
		Provider:       "google",
		ProviderUserID: body.Sub,
		Username:       "google_" + body.Sub,
		Email:          body.Email,
		Nickname:       firstNonEmpty(body.Name, body.Email),
		AvatarURL:      body.Picture,
	}, nil
}

func fetchQQProfile(ctx context.Context, clientID string, accessToken string) (oauthUserProfile, error) {
	openID, err := fetchQQOpenID(ctx, accessToken)
	if err != nil {
		return oauthUserProfile{}, err
	}
	q := url.Values{}
	q.Set("access_token", accessToken)
	q.Set("oauth_consumer_key", clientID)
	q.Set("openid", openID)
	req, err := stdhttp.NewRequestWithContext(ctx, stdhttp.MethodGet, "https://graph.qq.com/user/get_user_info?"+q.Encode(), nil)
	if err != nil {
		return oauthUserProfile{}, err
	}
	var body struct {
		Ret          int    `json:"ret"`
		Msg          string `json:"msg"`
		Nickname     string `json:"nickname"`
		FigureURLQQ2 string `json:"figureurl_qq_2"`
		FigureURLQQ1 string `json:"figureurl_qq_1"`
	}
	if err := doJSON(req, &body); err != nil {
		return oauthUserProfile{}, err
	}
	if body.Ret != 0 {
		return oauthUserProfile{}, errors.New(body.Msg)
	}
	return oauthUserProfile{
		Provider:       "qq",
		ProviderUserID: openID,
		Username:       "qq_" + openID,
		Nickname:       firstNonEmpty(body.Nickname, "QQ User"),
		AvatarURL:      firstNonEmpty(body.FigureURLQQ2, body.FigureURLQQ1),
	}, nil
}

func fetchQQOpenID(ctx context.Context, accessToken string) (string, error) {
	req, err := stdhttp.NewRequestWithContext(ctx, stdhttp.MethodGet, "https://graph.qq.com/oauth2.0/me?access_token="+url.QueryEscape(accessToken), nil)
	if err != nil {
		return "", err
	}
	text, err := doText(req)
	if err != nil {
		return "", err
	}
	start := strings.Index(text, "{")
	end := strings.LastIndex(text, "}")
	if start < 0 || end <= start {
		return "", errors.New("invalid qq openid response")
	}
	var body struct {
		OpenID string `json:"openid"`
	}
	if err := json.Unmarshal([]byte(text[start:end+1]), &body); err != nil {
		return "", err
	}
	if strings.TrimSpace(body.OpenID) == "" {
		return "", errors.New("qq openid missing")
	}
	return body.OpenID, nil
}

func githubAccountMeetsMinAge(createdAt time.Time, minYears int, now time.Time) bool {
	if createdAt.IsZero() {
		return false
	}
	if minYears <= 0 {
		minYears = 3
	}
	return !createdAt.After(now.AddDate(-minYears, 0, 0))
}

func doJSON(req *stdhttp.Request, out any) error {
	text, err := doText(req)
	if err != nil {
		return err
	}
	return json.Unmarshal([]byte(text), out)
}

func doText(req *stdhttp.Request) (string, error) {
	client := &stdhttp.Client{Timeout: 8 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	data, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return "", err
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return "", fmt.Errorf("oauth provider returned %d", resp.StatusCode)
	}
	return string(data), nil
}

func requireAccessToken(token string) (string, error) {
	token = strings.TrimSpace(token)
	if token == "" {
		return "", errors.New("missing access token")
	}
	return token, nil
}

func (h *Handler) signOAuthState(provider string, returnTo string) (string, error) {
	nonce := make([]byte, 12)
	if _, err := rand.Read(nonce); err != nil {
		return "", err
	}
	now := time.Now()
	claims := oauthStateClaims{
		Provider: provider,
		ReturnTo: returnTo,
		Nonce:    hex.EncodeToString(nonce),
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(now.Add(oauthStateTTL)),
			IssuedAt:  jwt.NewNumericDate(now),
		},
	}
	return jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString(h.jwtSecret)
}

func (h *Handler) verifyOAuthState(raw string, provider string) (string, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return "", errors.New("missing state")
	}
	token, err := jwt.ParseWithClaims(raw, &oauthStateClaims{}, func(token *jwt.Token) (any, error) {
		if token.Method != jwt.SigningMethodHS256 {
			return nil, errors.New("unexpected signing method")
		}
		return h.jwtSecret, nil
	})
	if err != nil || !token.Valid {
		return "", errors.New("invalid state")
	}
	claims, ok := token.Claims.(*oauthStateClaims)
	if !ok || claims.Provider != provider || strings.TrimSpace(claims.ReturnTo) == "" {
		return "", errors.New("invalid state")
	}
	return claims.ReturnTo, nil
}

func oauthRedirectWithAuth(returnTo string, resp *userpb.AuthResponse) string {
	values := url.Values{}
	values.Set("access_token", resp.GetAccessToken())
	values.Set("expires_at", strconv.FormatInt(resp.GetExpiresAt(), 10))
	values.Set("status", "success")
	return appendFragmentValues(returnTo, values)
}

func oauthRedirectWithError(returnTo string, message string) string {
	values := url.Values{}
	values.Set("error", message)
	return appendFragmentValues(returnTo, values)
}

func appendFragmentValues(raw string, values url.Values) string {
	u, err := url.Parse(raw)
	if err != nil {
		return raw
	}
	current, _ := url.ParseQuery(u.Fragment)
	for key, list := range values {
		for _, value := range list {
			current.Set(key, value)
		}
	}
	u.Fragment = current.Encode()
	return u.String()
}

func allowedOAuthReturnTo(c *gin.Context, settings authSettings, raw string) string {
	raw = strings.TrimSpace(raw)
	if isAllowedReturnTo(c, settings, raw) {
		return raw
	}
	return oauthReturnToFallback(c, settings)
}

func oauthReturnToFallback(c *gin.Context, settings authSettings) string {
	configured := strings.TrimSpace(settings["auth.oauth.frontend_callback_url"])
	if isAllowedReturnTo(c, settings, configured) {
		return configured
	}
	if referer := strings.TrimSpace(c.Request.Referer()); referer != "" {
		if u, err := url.Parse(referer); err == nil && u.Scheme != "" && u.Host != "" {
			return u.Scheme + "://" + u.Host + "/auth/callback"
		}
	}
	return "http://127.0.0.1:5173/auth/callback"
}

func isAllowedReturnTo(c *gin.Context, settings authSettings, raw string) bool {
	u, err := url.Parse(raw)
	if err != nil || u.Scheme == "" || u.Host == "" || u.User != nil {
		return false
	}
	if u.Scheme != "http" && u.Scheme != "https" {
		return false
	}
	configured := strings.TrimSpace(settings["auth.oauth.frontend_callback_url"])
	if configured != "" && strings.EqualFold(strings.TrimRight(raw, "/"), strings.TrimRight(configured, "/")) {
		return true
	}
	host := strings.ToLower(u.Hostname())
	if host == "127.0.0.1" || host == "localhost" || host == "::1" {
		return true
	}
	if referer := strings.TrimSpace(c.Request.Referer()); referer != "" {
		if ref, err := url.Parse(referer); err == nil && strings.EqualFold(ref.Host, u.Host) {
			return true
		}
	}
	return false
}

func settingBool(settings authSettings, key string, fallback bool) bool {
	value := strings.ToLower(strings.TrimSpace(settings[strings.ToLower(key)]))
	switch value {
	case "":
		return fallback
	case "1", "true", "yes", "on":
		return true
	case "0", "false", "no", "off":
		return false
	default:
		return fallback
	}
}

func settingInt(settings authSettings, key string, fallback int) int {
	value := strings.TrimSpace(settings[strings.ToLower(key)])
	if value == "" {
		return fallback
	}
	parsed, err := strconv.Atoi(value)
	if err != nil {
		return fallback
	}
	return parsed
}

func normalizeOAuthProvider(provider string) string {
	provider = strings.ToLower(strings.TrimSpace(provider))
	switch provider {
	case "github", "google", "qq":
		return provider
	default:
		return ""
	}
}

func providerLabel(provider string) string {
	switch provider {
	case "github":
		return "GitHub"
	case "google":
		return "Google"
	case "qq":
		return "QQ"
	default:
		return provider
	}
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}
