package http

import (
	"encoding/json"
	stdhttp "net/http"
	"strings"

	"api-gateway/api/proto/userpb"
	"api-gateway/pkg/http/response"

	"github.com/gin-gonic/gin"
)

func (h *Handler) listPasskeys(c *gin.Context) {
	if !h.passkeyClientAvailable(c) {
		return
	}
	ctx, cancel := rpcContext(c)
	defer cancel()
	result, err := h.clients.UserPasskeys.ListPasskeys(ctx, &userpb.UserIDRequest{Id: currentUserID(c)})
	if err != nil {
		writeRPCError(c, err)
		return
	}
	response.Success(c, result)
}

func (h *Handler) beginPasskeyRegistration(c *gin.Context) {
	if !h.passkeyClientAvailable(c) {
		return
	}
	var req beginPasskeyRegistrationRequest
	if !bindJSON(c, &req) {
		return
	}
	ctx, cancel := rpcContext(c)
	defer cancel()
	result, err := h.clients.UserPasskeys.BeginPasskeyRegistration(ctx, &userpb.BeginPasskeyRegistrationRequest{
		UserId: currentUserID(c), Name: req.Name, Password: req.Password, Code: req.Code,
	})
	if err != nil {
		writeRPCError(c, err)
		return
	}
	writePasskeyOptions(c, result)
}

func (h *Handler) finishPasskeyRegistration(c *gin.Context) {
	if !h.passkeyClientAvailable(c) {
		return
	}
	var req finishPasskeyRegistrationRequest
	if !bindPasskeyCredential(c, &req.Credential, &req) {
		return
	}
	ctx, cancel := rpcContext(c)
	defer cancel()
	result, err := h.clients.UserPasskeys.FinishPasskeyRegistration(ctx, &userpb.FinishPasskeyRegistrationRequest{
		UserId: currentUserID(c), Challenge: req.Challenge, CredentialJson: string(req.Credential),
	})
	if err != nil {
		writeRPCError(c, err)
		return
	}
	response.Success(c, result)
}

func (h *Handler) updatePasskey(c *gin.Context) {
	if !h.passkeyClientAvailable(c) {
		return
	}
	credentialID := strings.TrimSpace(c.Param("credentialId"))
	if credentialID == "" {
		writeError(c, stdhttp.StatusBadRequest, "credential ID is required", "invalid_argument")
		return
	}
	var req updatePasskeyRequest
	if !bindJSON(c, &req) {
		return
	}
	ctx, cancel := rpcContext(c)
	defer cancel()
	result, err := h.clients.UserPasskeys.UpdatePasskey(ctx, &userpb.UpdatePasskeyRequest{UserId: currentUserID(c), CredentialId: credentialID, Name: req.Name})
	if err != nil {
		writeRPCError(c, err)
		return
	}
	response.Success(c, result)
}

func (h *Handler) deletePasskey(c *gin.Context) {
	if !h.passkeyClientAvailable(c) {
		return
	}
	credentialID := strings.TrimSpace(c.Param("credentialId"))
	if credentialID == "" {
		writeError(c, stdhttp.StatusBadRequest, "credential ID is required", "invalid_argument")
		return
	}
	var req deletePasskeyRequest
	if !bindJSON(c, &req) {
		return
	}
	ctx, cancel := rpcContext(c)
	defer cancel()
	result, err := h.clients.UserPasskeys.DeletePasskey(ctx, &userpb.DeletePasskeyRequest{
		UserId: currentUserID(c), CredentialId: credentialID, Password: req.Password, Code: req.Code,
	})
	if err != nil {
		writeRPCError(c, err)
		return
	}
	response.Success(c, result)
}

func (h *Handler) setPasskeyPasswordless(c *gin.Context) {
	if !h.passkeyClientAvailable(c) {
		return
	}
	var req setPasskeyPasswordlessRequest
	if !bindJSON(c, &req) {
		return
	}
	ctx, cancel := rpcContext(c)
	defer cancel()
	result, err := h.clients.UserPasskeys.SetPasskeyPasswordless(ctx, &userpb.SetPasskeyPasswordlessRequest{
		UserId: currentUserID(c), Enabled: req.Enabled, Password: req.Password, Code: req.Code,
	})
	if err != nil {
		writeRPCError(c, err)
		return
	}
	response.Success(c, result)
}

func (h *Handler) beginPasskeyMFALogin(c *gin.Context) {
	if !h.passkeyClientAvailable(c) {
		return
	}
	var req beginPasskeyMFARequest
	if !bindJSON(c, &req) {
		return
	}
	if !h.allowAuthRateLimit(c, h.authRateLimits.Login, authRateLimitLogin, req.MFAChallenge) {
		return
	}
	ctx, cancel := rpcContext(c)
	defer cancel()
	result, err := h.clients.UserPasskeys.BeginPasskeyMFALogin(ctx, &userpb.BeginPasskeyMFALoginRequest{MfaChallenge: req.MFAChallenge})
	if err != nil {
		writeRPCError(c, err)
		return
	}
	writePasskeyOptions(c, result)
}

func (h *Handler) completePasskeyMFALogin(c *gin.Context) {
	if !h.passkeyClientAvailable(c) {
		return
	}
	var req completePasskeyLoginRequest
	if !bindPasskeyCredential(c, &req.Credential, &req) {
		return
	}
	if !h.allowAuthRateLimit(c, h.authRateLimits.Login, authRateLimitLogin, req.Challenge) {
		return
	}
	ctx, cancel := rpcContext(c)
	defer cancel()
	result, err := h.clients.UserPasskeys.CompletePasskeyMFALogin(ctx, &userpb.CompletePasskeyLoginRequest{Challenge: req.Challenge, CredentialJson: string(req.Credential), Client: sessionClientInfo(c)})
	if err != nil {
		writeRPCError(c, err)
		return
	}
	h.sanitizeAuthResponseProfileTheme(ctx, result)
	response.Success(c, result)
}

func (h *Handler) beginPasswordlessPasskeyLogin(c *gin.Context) {
	if !h.passkeyClientAvailable(c) {
		return
	}
	if !h.allowAuthRateLimit(c, h.authRateLimits.Login, authRateLimitLogin, "") {
		return
	}
	ctx, cancel := rpcContext(c)
	defer cancel()
	result, err := h.clients.UserPasskeys.BeginPasswordlessPasskeyLogin(ctx, &userpb.PasswordlessPasskeyOptionsRequest{})
	if err != nil {
		writeRPCError(c, err)
		return
	}
	writePasskeyOptions(c, result)
}

func (h *Handler) completePasswordlessPasskeyLogin(c *gin.Context) {
	if !h.passkeyClientAvailable(c) {
		return
	}
	var req completePasskeyLoginRequest
	if !bindPasskeyCredential(c, &req.Credential, &req) {
		return
	}
	if !h.allowAuthRateLimit(c, h.authRateLimits.Login, authRateLimitLogin, req.Challenge) {
		return
	}
	ctx, cancel := rpcContext(c)
	defer cancel()
	result, err := h.clients.UserPasskeys.CompletePasswordlessPasskeyLogin(ctx, &userpb.CompletePasskeyLoginRequest{Challenge: req.Challenge, CredentialJson: string(req.Credential), Client: sessionClientInfo(c)})
	if err != nil {
		writeRPCError(c, err)
		return
	}
	h.sanitizeAuthResponseProfileTheme(ctx, result)
	response.Success(c, result)
}

func bindPasskeyCredential(c *gin.Context, credential *json.RawMessage, target any) bool {
	if !bindJSON(c, target) {
		return false
	}
	if credential == nil || len(*credential) == 0 || string(*credential) == "null" {
		writeError(c, stdhttp.StatusBadRequest, "passkey credential is required", "invalid_argument")
		return false
	}
	return true
}

func writePasskeyOptions(c *gin.Context, result *userpb.PasskeyOptionsResponse) {
	if result == nil {
		writeError(c, stdhttp.StatusBadGateway, "passkey service returned an empty response", "bad_gateway")
		return
	}
	var options any
	if err := json.Unmarshal([]byte(result.GetOptionsJson()), &options); err != nil {
		writeError(c, stdhttp.StatusBadGateway, "passkey service returned invalid options", "bad_gateway")
		return
	}
	response.Success(c, gin.H{"challenge": result.GetChallenge(), "options": options, "expires_at": result.GetExpiresAt()})
}

func (h *Handler) passkeyClientAvailable(c *gin.Context) bool {
	if h != nil && h.clients != nil && h.clients.UserPasskeys != nil {
		return true
	}
	writeError(c, stdhttp.StatusServiceUnavailable, "passkey service unavailable", "service_unavailable")
	return false
}
