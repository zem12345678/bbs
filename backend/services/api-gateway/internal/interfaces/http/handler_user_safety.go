package http

import (
	"net/http"

	"api-gateway/api/proto/userpb"
	"api-gateway/pkg/http/response"

	"github.com/gin-gonic/gin"
)

type userSafetyStateResponse struct {
	Blocked   bool `json:"blocked"`
	BlockedBy bool `json:"blocked_by"`
	Muted     bool `json:"muted"`
}

func (h *Handler) blockUser(c *gin.Context) {
	h.updateUserSafetyRelation(c, "block")
}

func (h *Handler) unblockUser(c *gin.Context) {
	h.updateUserSafetyRelation(c, "unblock")
}

func (h *Handler) muteUserRelation(c *gin.Context) {
	h.updateUserSafetyRelation(c, "mute")
}

func (h *Handler) unmuteUserRelation(c *gin.Context) {
	h.updateUserSafetyRelation(c, "unmute")
}

func (h *Handler) updateUserSafetyRelation(c *gin.Context, action string) {
	if h == nil || h.clients == nil || h.clients.UserSafety == nil {
		writeError(c, http.StatusServiceUnavailable, "user safety service unavailable", "unavailable")
		return
	}
	targetID, ok := pathInt64(c, "id")
	if !ok {
		return
	}
	ctx, cancel := rpcContext(c)
	defer cancel()
	req := &userpb.UserRelationRequest{ActorId: currentUserID(c), TargetId: targetID}
	var (
		result *userpb.SimpleResponse
		err    error
	)
	switch action {
	case "block":
		result, err = h.clients.UserSafety.Block(ctx, req)
	case "unblock":
		result, err = h.clients.UserSafety.Unblock(ctx, req)
	case "mute":
		result, err = h.clients.UserSafety.Mute(ctx, req)
	default:
		result, err = h.clients.UserSafety.Unmute(ctx, req)
	}
	if err != nil {
		writeRPCError(c, err)
		return
	}
	response.Success(c, result)
}

func (h *Handler) getUserSafetyState(c *gin.Context) {
	if h == nil || h.clients == nil || h.clients.UserSafety == nil {
		writeError(c, http.StatusServiceUnavailable, "user safety service unavailable", "unavailable")
		return
	}
	targetID, ok := pathInt64(c, "id")
	if !ok {
		return
	}
	ctx, cancel := rpcContext(c)
	defer cancel()
	result, err := h.clients.UserSafety.GetSafetyRelation(ctx, &userpb.UserRelationRequest{ActorId: currentUserID(c), TargetId: targetID})
	if err != nil {
		writeRPCError(c, err)
		return
	}
	response.Success(c, userSafetyStateResponse{Blocked: result.GetBlocked(), BlockedBy: result.GetBlockedBy(), Muted: result.GetMuted()})
}

func (h *Handler) listBlockedUsers(c *gin.Context) {
	h.listSafetyUsers(c, true)
}

func (h *Handler) listMutedUsers(c *gin.Context) {
	h.listSafetyUsers(c, false)
}

func (h *Handler) listSafetyUsers(c *gin.Context, blocked bool) {
	if h == nil || h.clients == nil || h.clients.UserSafety == nil {
		writeError(c, http.StatusServiceUnavailable, "user safety service unavailable", "unavailable")
		return
	}
	ctx, cancel := rpcContext(c)
	defer cancel()
	req := &userpb.ListUserRelationsRequest{
		ActorId:  currentUserID(c),
		Page:     queryInt32(c, "page", 1),
		PageSize: queryInt32(c, "page_size", 20),
	}
	var (
		result *userpb.UserListResponse
		err    error
	)
	if blocked {
		result, err = h.clients.UserSafety.ListBlockedUsers(ctx, req)
	} else {
		result, err = h.clients.UserSafety.ListMutedUsers(ctx, req)
	}
	if err != nil {
		writeRPCError(c, err)
		return
	}
	h.sanitizeUserProfileThemes(ctx, result.GetItems())
	response.Success(c, toPublicUserListResponse(result))
}
