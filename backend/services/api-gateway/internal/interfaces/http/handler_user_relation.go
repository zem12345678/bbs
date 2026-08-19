package http

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strconv"

	"api-gateway/api/proto/userpb"

	"github.com/gin-gonic/gin"
)

var errInvalidUserRelationID = errors.New("invalid user relation id")

type userRelationCompatRequest struct {
	UserID json.RawMessage `json:"userId"`
}

type userRelationCompatResponse struct {
	ID                             string  `json:"id"`
	IsFollowing                    bool    `json:"isFollowing"`
	HasPendingFollowRequestFromYou bool    `json:"hasPendingFollowRequestFromYou"`
	HasPendingFollowRequestToYou   bool    `json:"hasPendingFollowRequestToYou"`
	IsFollowed                     bool    `json:"isFollowed"`
	IsBlocking                     bool    `json:"isBlocking"`
	IsBlocked                      bool    `json:"isBlocked"`
	IsMuted                        bool    `json:"isMuted"`
	IsRenoteMuted                  bool    `json:"isRenoteMuted"`
	IsInstanceMuted                bool    `json:"isInstanceMuted"`
	Memo                           *string `json:"memo"`
}

func (h *Handler) getUserRelationsCompat(c *gin.Context) {
	if h == nil || h.clients == nil || h.clients.User == nil || h.clients.UserSafety == nil || h.clients.UserMemos == nil {
		writeError(c, http.StatusServiceUnavailable, "user relation service unavailable", "service_unavailable")
		return
	}
	var req userRelationCompatRequest
	if !bindJSON(c, &req) {
		return
	}
	targetIDs, batch, err := parseUserRelationIDs(req.UserID)
	if err != nil || len(targetIDs) == 0 || len(targetIDs) > publicUserBatchLookupLimit {
		writeError(c, http.StatusBadRequest, "Invalid param.", "INVALID_PARAM")
		return
	}

	ctx, cancel := rpcContext(c)
	defer cancel()
	viewerID := currentUserID(c)
	items := make([]userRelationCompatResponse, 0, len(targetIDs))
	for _, targetID := range targetIDs {
		item, err := h.loadUserRelationCompat(ctx, viewerID, targetID)
		if err != nil {
			writeRPCError(c, err)
			return
		}
		items = append(items, item)
	}
	if batch {
		c.JSON(http.StatusOK, items)
		return
	}
	c.JSON(http.StatusOK, items[0])
}

func (h *Handler) loadUserRelationCompat(ctx context.Context, viewerID, targetID int64) (userRelationCompatResponse, error) {
	item := userRelationCompatResponse{ID: strconv.FormatInt(targetID, 10)}
	if viewerID != targetID {
		outbound, err := h.clients.User.IsFollowing(ctx, &userpb.FollowRequest{FollowerId: viewerID, FolloweeId: targetID})
		if err != nil {
			return item, err
		}
		inbound, err := h.clients.User.IsFollowing(ctx, &userpb.FollowRequest{FollowerId: targetID, FolloweeId: viewerID})
		if err != nil {
			return item, err
		}
		safety, err := h.clients.UserSafety.GetSafetyRelation(ctx, &userpb.UserRelationRequest{ActorId: viewerID, TargetId: targetID})
		if err != nil {
			return item, err
		}
		item.IsFollowing = outbound.GetFollowing()
		item.HasPendingFollowRequestFromYou = outbound.GetPending()
		item.IsFollowed = inbound.GetFollowing()
		item.HasPendingFollowRequestToYou = inbound.GetPending()
		item.IsBlocking = safety.GetBlocked()
		item.IsBlocked = safety.GetBlockedBy()
		item.IsMuted = safety.GetMuted()
	}
	memo, err := h.clients.UserMemos.GetUserMemo(ctx, &userpb.GetUserMemoRequest{UserId: viewerID, TargetUserId: targetID})
	if err != nil {
		return item, err
	}
	if value := memo.GetMemo(); value != "" {
		item.Memo = &value
	}
	return item, nil
}

func parseUserRelationIDs(raw json.RawMessage) ([]int64, bool, error) {
	raw = bytes.TrimSpace(raw)
	if len(raw) == 0 {
		return nil, false, errInvalidUserRelationID
	}
	if raw[0] == '[' {
		var values []jsonInt64
		if err := json.Unmarshal(raw, &values); err != nil {
			return nil, true, err
		}
		ids := make([]int64, len(values))
		for index, value := range values {
			ids[index] = value.Int64()
			if ids[index] <= 0 {
				return nil, true, errInvalidUserRelationID
			}
		}
		return ids, true, nil
	}
	var value jsonInt64
	if err := json.Unmarshal(raw, &value); err != nil || value.Int64() <= 0 {
		return nil, false, errInvalidUserRelationID
	}
	return []int64{value.Int64()}, false, nil
}
