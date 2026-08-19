package http

import (
	"encoding/json"
	"io"
	stdhttp "net/http"
	"strconv"
	"strings"

	"api-gateway/api/proto/userpb"

	"github.com/gin-gonic/gin"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type changePasswordCompatRequest struct {
	CurrentPassword string  `json:"currentPassword"`
	NewPassword     string  `json:"newPassword"`
	Token           *string `json:"token"`
}

type deleteAccountCompatRequest struct {
	Password string  `json:"password"`
	Token    *string `json:"token"`
}

func (h *Handler) changePasswordCompat(c *gin.Context) {
	var req changePasswordCompatRequest
	if !decodeSensitiveAccountCompatRequest(c, &req) || req.CurrentPassword == "" || req.NewPassword == "" {
		writeSensitiveAccountCompatInvalidParam(c)
		return
	}
	if h == nil || h.clients == nil || h.clients.User == nil {
		writeError(c, stdhttp.StatusServiceUnavailable, "user service unavailable", "service_unavailable")
		return
	}
	ctx, cancel := rpcContext(c)
	defer cancel()
	_, err := h.clients.User.ChangePassword(ctx, &userpb.ChangePasswordRequest{
		Id: currentUserID(c), OldPassword: req.CurrentPassword, NewPassword: req.NewPassword, MfaCode: optionalSensitiveAccountToken(req.Token),
	})
	if err != nil {
		writeSensitiveAccountCompatRPCError(c, err)
		return
	}
	c.Status(stdhttp.StatusNoContent)
}

func (h *Handler) deleteAccountCompat(c *gin.Context) {
	var req deleteAccountCompatRequest
	if !decodeSensitiveAccountCompatRequest(c, &req) || req.Password == "" {
		writeSensitiveAccountCompatInvalidParam(c)
		return
	}
	if !h.accountLifecycleClientAvailable(c) {
		return
	}
	if !h.allowAuthRateLimit(c, h.authRateLimits.Login, authRateLimitLogin, strconv.FormatInt(currentUserID(c), 10)) {
		return
	}
	ctx, cancel := rpcContext(c)
	defer cancel()
	_, err := h.clients.UserAccountLifecycle.RequestAccountDeletion(ctx, &userpb.RequestAccountDeletionRequest{
		UserId: currentUserID(c), Password: req.Password, Code: optionalSensitiveAccountToken(req.Token),
	})
	if err != nil {
		writeSensitiveAccountCompatRPCError(c, err)
		return
	}
	c.Status(stdhttp.StatusNoContent)
}

func decodeSensitiveAccountCompatRequest(c *gin.Context, out any) bool {
	decoder := json.NewDecoder(c.Request.Body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(out); err != nil {
		return false
	}
	return decoder.Decode(&struct{}{}) == io.EOF
}

func optionalSensitiveAccountToken(token *string) string {
	if token == nil {
		return ""
	}
	return strings.TrimSpace(*token)
}

func writeSensitiveAccountCompatRPCError(c *gin.Context, err error) {
	st := status.Convert(err)
	message := strings.ToLower(strings.TrimSpace(st.Message()))
	switch {
	case st.Code() == codes.InvalidArgument && message == "invalid password":
		writeError(c, stdhttp.StatusBadRequest, "Incorrect password.", "INCORRECT_PASSWORD")
	case st.Code() == codes.Unauthenticated && strings.Contains(message, "mfa"):
		writeError(c, stdhttp.StatusBadRequest, "Incorrect 2FA code.", "INCORRECT_TOTP")
	case st.Code() == codes.InvalidArgument:
		writeSensitiveAccountCompatInvalidParam(c)
	default:
		writeRPCError(c, err)
	}
}

func writeSensitiveAccountCompatInvalidParam(c *gin.Context) {
	writeError(c, stdhttp.StatusBadRequest, "Invalid param.", "INVALID_PARAM")
}
