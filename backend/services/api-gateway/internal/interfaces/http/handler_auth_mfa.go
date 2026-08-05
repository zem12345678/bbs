package http

import (
	"api-gateway/api/proto/userpb"
	"api-gateway/pkg/http/response"

	"github.com/gin-gonic/gin"
)

func (h *Handler) completeMFALogin(c *gin.Context) {
	var req completeMFALoginRequest
	if !bindJSON(c, &req) {
		return
	}
	if !h.allowAuthRateLimit(c, h.authRateLimits.Login, authRateLimitLogin, req.Challenge) {
		return
	}
	ctx, cancel := rpcContext(c)
	defer cancel()
	resp, err := h.clients.UserMFA.CompleteMFALogin(ctx, &userpb.CompleteMFALoginRequest{
		Challenge: req.Challenge,
		Code:      req.Code,
		Client:    sessionClientInfo(c),
	})
	if err != nil {
		writeRPCError(c, err)
		return
	}
	h.sanitizeAuthResponseProfileTheme(ctx, resp)
	response.Success(c, resp)
}

func (h *Handler) getMFAStatus(c *gin.Context) {
	ctx, cancel := rpcContext(c)
	defer cancel()
	resp, err := h.clients.UserMFA.GetMFAStatus(ctx, &userpb.UserIDRequest{Id: currentUserID(c)})
	if err != nil {
		writeRPCError(c, err)
		return
	}
	response.Success(c, resp)
}

func (h *Handler) beginTOTPEnrollment(c *gin.Context) {
	var req beginTOTPEnrollmentRequest
	if !bindJSON(c, &req) {
		return
	}
	ctx, cancel := rpcContext(c)
	defer cancel()
	resp, err := h.clients.UserMFA.BeginTOTPEnrollment(ctx, &userpb.BeginTOTPEnrollmentRequest{
		UserId:      currentUserID(c),
		Password:    req.Password,
		CurrentCode: req.CurrentCode,
	})
	if err != nil {
		writeRPCError(c, err)
		return
	}
	response.Success(c, resp)
}

func (h *Handler) confirmTOTPEnrollment(c *gin.Context) {
	var req confirmTOTPEnrollmentRequest
	if !bindJSON(c, &req) {
		return
	}
	ctx, cancel := rpcContext(c)
	defer cancel()
	resp, err := h.clients.UserMFA.ConfirmTOTPEnrollment(ctx, &userpb.ConfirmTOTPEnrollmentRequest{
		UserId: currentUserID(c),
		Code:   req.Code,
	})
	if err != nil {
		writeRPCError(c, err)
		return
	}
	response.Success(c, resp)
}

func (h *Handler) regenerateMFARecoveryCodes(c *gin.Context) {
	var req mfaReauthenticateRequest
	if !bindJSON(c, &req) {
		return
	}
	ctx, cancel := rpcContext(c)
	defer cancel()
	resp, err := h.clients.UserMFA.RegenerateMFARecoveryCodes(ctx, &userpb.MFAReauthenticateRequest{
		UserId:   currentUserID(c),
		Password: req.Password,
		Code:     req.Code,
	})
	if err != nil {
		writeRPCError(c, err)
		return
	}
	response.Success(c, resp)
}

func (h *Handler) disableTOTP(c *gin.Context) {
	var req mfaReauthenticateRequest
	if !bindJSON(c, &req) {
		return
	}
	ctx, cancel := rpcContext(c)
	defer cancel()
	resp, err := h.clients.UserMFA.DisableTOTP(ctx, &userpb.MFAReauthenticateRequest{
		UserId:   currentUserID(c),
		Password: req.Password,
		Code:     req.Code,
	})
	if err != nil {
		writeRPCError(c, err)
		return
	}
	response.Success(c, resp)
}
