package http

import (
	"bytes"
	"encoding/json"
	stdhttp "net/http"
	"strings"

	"api-gateway/api/proto/userpb"

	"github.com/gin-gonic/gin"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

const usersShowNoSuchUserID = "4362f8dc-731f-4ad8-a694-be5a88922a24"

type usersShowCompatRequest struct {
	UserID   *jsonInt64      `json:"userId"`
	UserIDs  []jsonInt64     `json:"userIds"`
	Username string          `json:"username"`
	Host     json.RawMessage `json:"host"`
	Detail   *bool           `json:"detail"`
}

type usersListCompatRequest struct {
	Limit    *int32          `json:"limit"`
	Offset   *int32          `json:"offset"`
	Sort     string          `json:"sort"`
	State    string          `json:"state"`
	Origin   string          `json:"origin"`
	Hostname json.RawMessage `json:"hostname"`
	Detail   *bool           `json:"detail"`
}

func (h *Handler) registerUsersCompatRoutes(router *gin.Engine) {
	for _, prefix := range []string{"", "/api", "/api/v1"} {
		router.POST(prefix+"/users/show", h.optionalAuth(), h.showUsersCompat)
		router.POST(prefix+"/users", h.optionalAuth(), h.listUsersCompat)
		router.POST(prefix+"/i", h.requireAuthScope("read"), h.showCurrentUserCompat)
	}
}

func (h *Handler) listUsersCompat(c *gin.Context) {
	var request usersListCompatRequest
	if !decodeSensitiveAccountCompatRequest(c, &request) {
		writeFollowingCompatError(c, "Invalid param.", "INVALID_PARAM", "")
		return
	}
	limit := int32(10)
	if request.Limit != nil {
		limit = *request.Limit
	}
	offset := int32(0)
	if request.Offset != nil {
		offset = *request.Offset
	}
	if limit < 1 || limit > 100 || offset < 0 || offset > 10000 {
		writeFollowingCompatError(c, "Invalid param.", "INVALID_PARAM", "")
		return
	}
	switch request.Sort {
	case "", "+follower", "-follower", "+createdAt", "-createdAt", "+updatedAt", "-updatedAt":
	default:
		writeFollowingCompatError(c, "Invalid param.", "INVALID_PARAM", "")
		return
	}
	switch request.State {
	case "", "all", "alive":
	default:
		writeFollowingCompatError(c, "Invalid param.", "INVALID_PARAM", "")
		return
	}
	switch request.Origin {
	case "", "local":
	default:
		writeFollowingCompatError(c, "Invalid param.", "INVALID_PARAM", "")
		return
	}
	if len(bytes.TrimSpace(request.Hostname)) > 0 && !bytes.Equal(bytes.TrimSpace(request.Hostname), []byte("null")) {
		writeFollowingCompatError(c, "Invalid param.", "INVALID_PARAM", "")
		return
	}
	page := offset/limit + 1
	ctx, cancel := rpcContext(c)
	defer cancel()
	result, err := h.clients.User.ListUsers(ctx, &userpb.ListUsersRequest{
		Status:   userStatusActive,
		Page:     int32(page),
		PageSize: limit,
		Sort:     request.Sort,
	})
	if err != nil {
		writeRPCError(c, err)
		return
	}
	items := make([]misskeyUserLite, 0, len(result.GetItems()))
	for _, user := range result.GetItems() {
		if user == nil || user.GetStatus() != userStatusActive || !publicAccountStateActive(user.GetAccountState()) {
			continue
		}
		h.sanitizeUserProfileTheme(ctx, user)
		items = append(items, toMisskeyUserLite(user))
	}
	c.JSON(stdhttp.StatusOK, items)
}

func (h *Handler) showCurrentUserCompat(c *gin.Context) {
	ctx, cancel := rpcContext(c)
	defer cancel()
	result, err := h.clients.User.GetUser(ctx, &userpb.UserIDRequest{Id: currentUserID(c)})
	if err != nil {
		writeRPCError(c, err)
		return
	}
	user := result.GetUser()
	if user == nil {
		writeError(c, stdhttp.StatusBadGateway, "user service returned an empty user", "internal_error")
		return
	}
	h.sanitizeUserProfileTheme(ctx, user)
	c.JSON(stdhttp.StatusOK, toMisskeyUserLite(user))
}

func (h *Handler) showUsersCompat(c *gin.Context) {
	var request usersShowCompatRequest
	if !decodeSensitiveAccountCompatRequest(c, &request) {
		writeFollowingCompatError(c, "Invalid param.", "INVALID_PARAM", "")
		return
	}
	if len(request.UserIDs) > 500 || (request.UserID != nil && len(request.UserIDs) > 0) {
		writeFollowingCompatError(c, "Invalid param.", "INVALID_PARAM", "")
		return
	}
	ctx, cancel := rpcContext(c)
	defer cancel()
	if len(request.UserIDs) > 0 || (request.UserIDs != nil && len(request.UserIDs) == 0) {
		items := make([]misskeyUserLite, 0, len(request.UserIDs))
		if len(request.UserIDs) == 0 {
			c.JSON(stdhttp.StatusOK, items)
			return
		}
		ids := make([]int64, 0, len(request.UserIDs))
		seen := make(map[int64]struct{}, len(request.UserIDs))
		for _, id := range request.UserIDs {
			value := id.Int64()
			if value <= 0 {
				writeFollowingCompatError(c, "Invalid param.", "INVALID_PARAM", "")
				return
			}
			if _, exists := seen[value]; exists {
				writeFollowingCompatError(c, "Invalid param.", "INVALID_PARAM", "")
				return
			}
			seen[value] = struct{}{}
			ids = append(ids, value)
		}
		result, err := h.clients.User.ListUsers(ctx, &userpb.ListUsersRequest{Ids: ids, Page: 1, PageSize: int32(len(ids))})
		if err != nil {
			writeRPCError(c, err)
			return
		}
		byID := make(map[int64]*userpb.UserInfo, len(result.GetItems()))
		for _, user := range result.GetItems() {
			byID[user.GetId()] = user
		}
		for _, id := range ids {
			if user := byID[id]; user != nil {
				h.sanitizeUserProfileTheme(ctx, user)
				items = append(items, toMisskeyUserLite(user))
			}
		}
		c.JSON(stdhttp.StatusOK, items)
		return
	}

	if request.UserID != nil {
		id := request.UserID.Int64()
		if id <= 0 {
			writeFollowingCompatError(c, "Invalid param.", "INVALID_PARAM", "")
			return
		}
		result, err := h.clients.User.GetUser(ctx, &userpb.UserIDRequest{Id: id})
		if err != nil {
			writeUsersShowRPCError(c, err)
			return
		}
		user := result.GetUser()
		if user == nil {
			writeError(c, stdhttp.StatusBadGateway, "user service returned an empty user", "internal_error")
			return
		}
		h.sanitizeUserProfileTheme(ctx, user)
		c.JSON(stdhttp.StatusOK, toMisskeyUserLite(user))
		return
	}

	username := strings.ToLower(strings.TrimSpace(request.Username))
	hostSet, local, validHost := decodeFollowingHost(request.Host)
	if !hostSet {
		local = true
	}
	if username == "" || len(username) > 256 || !validHost || !local {
		writeFollowingCompatError(c, "Invalid param.", "INVALID_PARAM", "")
		return
	}
	result, err := h.clients.User.GetUserByUsername(ctx, &userpb.UsernameRequest{Username: username})
	if err != nil {
		writeUsersShowRPCError(c, err)
		return
	}
	user := result.GetUser()
	if user == nil {
		writeError(c, stdhttp.StatusBadGateway, "user service returned an empty user", "internal_error")
		return
	}
	h.sanitizeUserProfileTheme(ctx, user)
	c.JSON(stdhttp.StatusOK, toMisskeyUserLite(user))
}

func writeUsersShowRPCError(c *gin.Context, err error) {
	if status.Code(err) == codes.NotFound {
		writeFollowingCompatError(c, "No such user.", "NO_SUCH_USER", usersShowNoSuchUserID)
		return
	}
	writeRPCError(c, err)
}
