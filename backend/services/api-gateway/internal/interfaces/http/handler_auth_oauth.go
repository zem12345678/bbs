package http

import (
	"context"
	"crypto/rand"
	"crypto/subtle"
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

	"api-gateway/api/proto/adminpb"
	"api-gateway/api/proto/userpb"
	"api-gateway/pkg/http/response"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
	"golang.org/x/crypto/bcrypt"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

const (
	oauthStateTTL        = 10 * time.Minute
	oauthStateCookieName = "bbs_oauth_state"
	oauthStateCookiePath = "/api/v1/auth/oauth"
)

type authSettings map[string]string

type registrationMode string

const (
	registrationModeOpen       registrationMode = "open"
	registrationModeInviteOnly registrationMode = "invite_only"
	registrationModeClosed     registrationMode = "closed"
)

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
			"start_url": h.publicURL(c, "/api/v1/auth/oauth/"+provider+"/start"),
		}
		if provider == "github" {
			item["min_account_years"] = cfg.MinAccountYears
		}
		providers = append(providers, item)
	}
	passwordEnabled := settingBool(settings, "auth.password.enabled", true)
	mode := registrationModeFromSettings(settings)
	response.Success(c, gin.H{
		"password_enabled":            passwordEnabled,
		"register_enabled":            passwordEnabled && mode != registrationModeClosed,
		"register_mode":               string(mode),
		"invite_required":             passwordEnabled && mode == registrationModeInviteOnly,
		"email_verification_required": settingBool(settings, "auth.email_verification.required", false),
		"webmaster_enabled":           strings.TrimSpace(settings["site.webmaster.username"]) != "" && strings.TrimSpace(settings["site.webmaster.password"]) != "",
		"oauth_callback_hint":         oauthReturnToFallback(settings),
		"providers":                   providers,
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
	returnTo := allowedOAuthReturnTo(settings, c.Query("redirect"))
	if returnTo == "" {
		writeError(c, stdhttp.StatusServiceUnavailable, "oauth frontend callback is not configured", "service_unavailable")
		return
	}
	state, nonce, err := h.signOAuthState(provider, returnTo)
	if err != nil {
		writeError(c, stdhttp.StatusInternalServerError, "create oauth state failed", "internal_error")
		return
	}
	redirectURI := h.publicURL(c, "/api/v1/auth/oauth/"+provider+"/callback")
	authURL, err := buildOAuthAuthorizeURL(provider, cfg.ClientID, redirectURI, state)
	if err != nil {
		writeError(c, stdhttp.StatusBadRequest, err.Error(), "bad_request")
		return
	}
	setOAuthStateCookie(c, nonce)
	c.Redirect(stdhttp.StatusFound, authURL)
}

func (h *Handler) oauthCallback(c *gin.Context) {
	clearOAuthStateCookie(c)
	provider := normalizeOAuthProvider(c.Param("provider"))
	if provider == "" {
		writeError(c, stdhttp.StatusBadRequest, "unsupported oauth provider", "bad_request")
		return
	}
	returnTo, nonce, err := h.verifyOAuthState(c.Query("state"), provider)
	if err != nil || !oauthStateCookieMatches(c, nonce) {
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
	mode := registrationModeFromSettings(settings)
	cfg := providerConfig(settings, provider)
	if !cfg.Enabled || cfg.ClientID == "" || cfg.ClientSecret == "" {
		c.Redirect(stdhttp.StatusFound, oauthRedirectWithError(returnTo, "oauth provider not configured"))
		return
	}
	redirectURI := h.publicURL(c, "/api/v1/auth/oauth/"+provider+"/callback")
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
		ExistingOnly:   mode != registrationModeOpen,
	})
	if err != nil {
		c.Redirect(stdhttp.StatusFound, oauthRedirectWithError(returnTo, oauthLoginErrorMessage(err, mode)))
		return
	}
	h.sanitizeAuthResponseProfileTheme(ctx, resp)
	c.Redirect(stdhttp.StatusFound, oauthRedirectWithAuth(returnTo, resp))
}

func oauthLoginErrorMessage(err error, mode registrationMode) string {
	if status.Code(err) == codes.PermissionDenied && mode != registrationModeOpen {
		if mode == registrationModeInviteOnly {
			return "oauth registration requires an invite code"
		}
		return "registration closed"
	}
	return "community login failed"
}

func (h *Handler) tryWebmasterLogin(c *gin.Context, ctx context.Context, req loginRequest) (*userpb.AuthResponse, bool) {
	settings, err := h.loadAuthSettings(ctx, true)
	if err != nil {
		return nil, false
	}
	username := strings.ToLower(strings.TrimSpace(settings["site.webmaster.username"]))
	configuredPassword := strings.TrimSpace(settings["site.webmaster.password"])
	account := strings.ToLower(strings.TrimSpace(req.Account))
	if username == "" || configuredPassword == "" || account != username {
		return nil, false
	}
	if !webmasterPasswordMatches(configuredPassword, req.Password) {
		writeError(c, stdhttp.StatusUnauthorized, "invalid password", "unauthorized")
		return nil, true
	}
	resp, err := h.clients.User.WebmasterLogin(ctx, &userpb.WebmasterLoginRequest{
		Username: username,
		Password: req.Password,
		Nickname: "Webmaster",
	})
	if err != nil {
		writeRPCError(c, err)
		return nil, true
	}
	return resp, true
}

func webmasterPasswordMatches(configured string, supplied string) bool {
	configured = strings.TrimSpace(configured)
	if configured == "" {
		return false
	}
	if isBcryptHash(configured) {
		return bcrypt.CompareHashAndPassword([]byte(configured), []byte(supplied)) == nil
	}
	return configured == supplied
}

func isBcryptHash(value string) bool {
	value = strings.TrimSpace(value)
	if len(value) != 60 {
		return false
	}
	return strings.HasPrefix(value, "$2a$") ||
		strings.HasPrefix(value, "$2b$") ||
		strings.HasPrefix(value, "$2x$") ||
		strings.HasPrefix(value, "$2y$")
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
		return false
	}
	mode := registrationModeFromSettings(settings)
	return settingBool(settings, "auth.password.enabled", true) && mode != registrationModeClosed
}

func registrationModeFromSettings(settings authSettings) registrationMode {
	raw := strings.ToLower(strings.TrimSpace(settings["auth.register.mode"]))
	if raw != "" {
		switch registrationMode(raw) {
		case registrationModeOpen, registrationModeInviteOnly, registrationModeClosed:
			return registrationMode(raw)
		default:
			// A malformed explicit mode must not silently fall back to open
			// registration. Operators can recover by correcting the setting.
			return registrationModeClosed
		}
	}
	if legacyRegistrationEnabled(settings) {
		return registrationModeOpen
	}
	return registrationModeClosed
}

func legacyRegistrationEnabled(settings authSettings) bool {
	raw := strings.ToLower(strings.TrimSpace(settings["auth.register.enabled"]))
	switch raw {
	case "", "1", "true", "yes", "on":
		return true
	case "0", "false", "no", "off":
		return false
	default:
		return false
	}
}

func (h *Handler) loadAuthSettings(ctx context.Context, includeSecrets bool) (authSettings, error) {
	if h == nil || h.clients == nil || h.clients.Admin == nil {
		return nil, status.Error(codes.Unavailable, "auth settings service unavailable")
	}
	resp, err := h.clients.Admin.ListAuthSettings(ctx, &adminpb.ListAuthSettingsRequest{IncludeSecrets: includeSecrets})
	if err != nil {
		return nil, err
	}
	if resp == nil {
		return nil, status.Error(codes.Unavailable, "auth settings service returned no response")
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

func (h *Handler) signOAuthState(provider string, returnTo string) (string, string, error) {
	nonce := make([]byte, 12)
	if _, err := rand.Read(nonce); err != nil {
		return "", "", err
	}
	nonceValue := hex.EncodeToString(nonce)
	now := time.Now()
	claims := oauthStateClaims{
		Provider: provider,
		ReturnTo: returnTo,
		Nonce:    nonceValue,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(now.Add(oauthStateTTL)),
			IssuedAt:  jwt.NewNumericDate(now),
		},
	}
	state, err := jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString(h.jwtSecret)
	if err != nil {
		return "", "", err
	}
	return state, nonceValue, nil
}

func (h *Handler) verifyOAuthState(raw string, provider string) (string, string, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return "", "", errors.New("missing state")
	}
	token, err := jwt.ParseWithClaims(raw, &oauthStateClaims{}, func(token *jwt.Token) (any, error) {
		if token.Method != jwt.SigningMethodHS256 {
			return nil, errors.New("unexpected signing method")
		}
		return h.jwtSecret, nil
	})
	if err != nil || !token.Valid {
		return "", "", errors.New("invalid state")
	}
	claims, ok := token.Claims.(*oauthStateClaims)
	if !ok || claims.Provider != provider || strings.TrimSpace(claims.ReturnTo) == "" || strings.TrimSpace(claims.Nonce) == "" {
		return "", "", errors.New("invalid state")
	}
	return claims.ReturnTo, claims.Nonce, nil
}

func setOAuthStateCookie(c *gin.Context, nonce string) {
	writeOAuthStateCookie(c, nonce, int(oauthStateTTL.Seconds()))
}

func clearOAuthStateCookie(c *gin.Context) {
	writeOAuthStateCookie(c, "", -1)
}

func writeOAuthStateCookie(c *gin.Context, value string, maxAge int) {
	cookie := &stdhttp.Cookie{
		Name:     oauthStateCookieName,
		Value:    value,
		Path:     oauthStateCookiePath,
		MaxAge:   maxAge,
		HttpOnly: true,
		SameSite: stdhttp.SameSiteLaxMode,
		Secure:   oauthStateCookieSecure(c),
	}
	if maxAge < 0 {
		cookie.Expires = time.Unix(1, 0)
	}
	stdhttp.SetCookie(c.Writer, cookie)
}

func oauthStateCookieSecure(c *gin.Context) bool {
	if c.Request != nil && c.Request.TLS != nil {
		return true
	}
	return strings.EqualFold(strings.TrimSpace(c.GetHeader("X-Forwarded-Proto")), "https")
}

func oauthStateCookieMatches(c *gin.Context, expected string) bool {
	actual, err := c.Cookie(oauthStateCookieName)
	if err != nil || expected == "" {
		return false
	}
	expectedBytes := []byte(expected)
	actualBytes := make([]byte, len(expectedBytes))
	copy(actualBytes, actual)
	return subtle.ConstantTimeCompare(expectedBytes, actualBytes)&
		subtle.ConstantTimeEq(int32(len(actual)), int32(len(expectedBytes))) == 1
}

func oauthRedirectWithAuth(returnTo string, resp *userpb.AuthResponse) string {
	values := url.Values{}
	if resp.GetMfaRequired() {
		values.Set("mfa_required", "true")
		values.Set("mfa_challenge", resp.GetMfaChallenge())
		values.Set("mfa_expires_at", strconv.FormatInt(resp.GetMfaExpiresAt(), 10))
		values.Set("status", "mfa_required")
		return appendFragmentValues(returnTo, values)
	}
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

func allowedOAuthReturnTo(settings authSettings, raw string) string {
	raw = strings.TrimSpace(raw)
	if isAllowedReturnTo(settings, raw) {
		return raw
	}
	return oauthReturnToFallback(settings)
}

func oauthReturnToFallback(settings authSettings) string {
	configured := strings.TrimSpace(settings["auth.oauth.frontend_callback_url"])
	if isAllowedReturnTo(settings, configured) {
		return configured
	}
	return ""
}

func isAllowedReturnTo(settings authSettings, raw string) bool {
	u, err := url.Parse(raw)
	if err != nil || u.Scheme == "" || u.Host == "" || u.User != nil {
		return false
	}
	if u.Scheme != "http" && u.Scheme != "https" {
		return false
	}
	configured := strings.TrimSpace(settings["auth.oauth.frontend_callback_url"])
	if configured != "" {
		// Keep the configured callback endpoint pinned while allowing its query
		// string to retain a client-side post-login destination.
		if callback, err := url.Parse(configured); err == nil && callback.User == nil &&
			strings.EqualFold(u.Scheme, callback.Scheme) && strings.EqualFold(u.Host, callback.Host) &&
			strings.TrimRight(u.Path, "/") == strings.TrimRight(callback.Path, "/") {
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
