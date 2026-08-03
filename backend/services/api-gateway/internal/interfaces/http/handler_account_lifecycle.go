package http

import (
	stdhttp "net/http"
	"strconv"

	"api-gateway/api/proto/userpb"
	"api-gateway/pkg/http/response"

	"github.com/gin-gonic/gin"
)

func (h *Handler) getAccountLifecycle(c *gin.Context) {
	if !h.accountLifecycleClientAvailable(c) {
		return
	}
	ctx, cancel := rpcContext(c)
	defer cancel()
	result, err := h.clients.UserAccountLifecycle.GetAccountLifecycle(ctx, &userpb.UserIDRequest{Id: currentUserID(c)})
	if err != nil {
		writeRPCError(c, err)
		return
	}
	response.Success(c, accountLifecyclePayload(result))
}

func (h *Handler) requestAccountDeletion(c *gin.Context) {
	if !h.accountLifecycleClientAvailable(c) {
		return
	}
	var req requestAccountDeletionRequest
	if !bindJSON(c, &req) {
		return
	}
	if !h.allowAuthRateLimit(c, h.authRateLimits.Login, authRateLimitLogin, strconv.FormatInt(currentUserID(c), 10)) {
		return
	}
	ctx, cancel := rpcContext(c)
	defer cancel()
	result, err := h.clients.UserAccountLifecycle.RequestAccountDeletion(ctx, &userpb.RequestAccountDeletionRequest{
		UserId: currentUserID(c), Password: req.Password, Code: req.Code,
	})
	if err != nil {
		writeRPCError(c, err)
		return
	}
	response.SuccessWithStatus(c, stdhttp.StatusAccepted, accountLifecyclePayload(result))
}

func accountLifecyclePayload(result *userpb.AccountLifecycleResponse) gin.H {
	if result == nil {
		return gin.H{}
	}
	payload := gin.H{
		"user_id": strconv.FormatInt(result.GetUserId(), 10), "state": result.GetState(),
		"state_version": strconv.FormatInt(result.GetStateVersion(), 10), "protected": result.GetProtected(),
		"deletion_requested_at": result.GetDeletionRequestedAt(), "deleted_at": result.GetDeletedAt(),
	}
	if job := result.GetActiveDeletionJob(); job != nil {
		payload["active_deletion_job"] = gin.H{
			"id": strconv.FormatInt(job.GetId(), 10), "status": job.GetStatus(), "policy_version": job.GetPolicyVersion(),
			"completed_steps": job.GetCompletedSteps(), "total_steps": job.GetTotalSteps(),
			"created_at": job.GetCreatedAt(), "updated_at": job.GetUpdatedAt(),
			"started_at": job.GetStartedAt(), "completed_at": job.GetCompletedAt(),
		}
	}
	return payload
}

func (h *Handler) accountLifecycleClientAvailable(c *gin.Context) bool {
	if h != nil && h.clients != nil && h.clients.UserAccountLifecycle != nil {
		return true
	}
	writeError(c, stdhttp.StatusServiceUnavailable, "account lifecycle service unavailable", "service_unavailable")
	return false
}
