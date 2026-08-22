package http

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	stdhttp "net/http"
	"strconv"
	"strings"
	"time"

	"api-gateway/api/proto/userpb"
	"api-gateway/internal/clients"
	"api-gateway/pkg/http/response"

	"github.com/gin-gonic/gin"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

const (
	followingCreateNoSuchUserID    = "fcd2eef9-a9b2-4c4f-8624-038099e90aa5"
	followingCreateSelfID          = "26fbe7bb-a331-4857-af17-205b426669a9"
	followingCreateAlreadyID       = "35387507-38c7-4cb9-9197-300b93783fa0"
	followingCreateBlockingID      = "4e2206ec-aa4f-4960-b865-6c23ac38e2d9"
	followingCreateBlockedID       = "c4ab57cc-4e41-45e9-bfd9-584f61e35ce0"
	followingDeleteNoSuchUserID    = "5b12c78d-2b28-4dca-99d2-f56139b42ff8"
	followingDeleteSelfID          = "d9e400b9-36b0-4808-b1d8-79e707f1296c"
	followingDeleteNotFollowingID  = "5dbf82f5-c92b-40b1-87d1-6c8c0741fd09"
	followingUpdateNoSuchUserID    = "14318698-f67e-492a-99da-5353a5ac52be"
	followingUpdateSelfID          = "4c4cbaf9-962a-463b-8418-a5e365dbf2eb"
	followingUpdateNotFollowingID  = "b8dc75cf-1cb5-46c9-b14b-5f1ffbd782c9"
	followersListNoSuchUserID      = "27fa5435-88ab-43de-9360-387de88727cd"
	followingListNoSuchUserID      = "63e4aba4-4156-4e53-be25-c9559e42d71b"
	followersListForbiddenID       = "3c6a84db-d619-26af-ca14-06232a21df8a"
	followingListForbiddenID       = "f6cdb0df-c19f-ec5c-7dbb-0ba84a1f92ba"
	followingListBirthdayInvalidID = "a2b007b9-4782-4eba-abd3-93b05ed4130d"
)

type followingMutationRequest struct {
	UserID      *jsonInt64 `json:"userId"`
	WithReplies *bool      `json:"withReplies,omitempty"`
	Notify      *string    `json:"notify,omitempty"`
}

type canonicalFollowRequest struct {
	WithReplies *bool `json:"withReplies,omitempty"`
}

// decodeOptionalJSONBody accepts the existing empty-body canonical requests
// while still rejecting malformed JSON and unknown fields when a body is sent.
func decodeOptionalJSONBody(c *gin.Context, out any) (present bool, valid bool) {
	raw, err := io.ReadAll(c.Request.Body)
	if err != nil {
		return false, false
	}
	trimmed := bytes.TrimSpace(raw)
	if len(trimmed) == 0 {
		return false, true
	}
	decoder := json.NewDecoder(bytes.NewReader(trimmed))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(out); err != nil {
		return true, false
	}
	var extra any
	if err := decoder.Decode(&extra); err != io.EOF {
		return true, false
	}
	return true, true
}

type followingUpdateAllRequest struct {
	WithReplies *bool   `json:"withReplies,omitempty"`
	Notify      *string `json:"notify,omitempty"`
}

type followingListRequest struct {
	SinceID  *jsonInt64      `json:"sinceId,omitempty"`
	UntilID  *jsonInt64      `json:"untilId,omitempty"`
	Limit    *int32          `json:"limit,omitempty"`
	UserID   *jsonInt64      `json:"userId,omitempty"`
	Username string          `json:"username,omitempty"`
	Host     json.RawMessage `json:"host,omitempty"`
	Birthday json.RawMessage `json:"birthday,omitempty"`
}

type misskeyFollowing struct {
	ID          string           `json:"id"`
	CreatedAt   string           `json:"createdAt"`
	FolloweeID  string           `json:"followeeId"`
	FollowerID  string           `json:"followerId"`
	WithReplies bool             `json:"withReplies"`
	Notify      string           `json:"notify"`
	Followee    *misskeyUserLite `json:"followee,omitempty"`
	Follower    *misskeyUserLite `json:"follower,omitempty"`
}

func (h *Handler) registerFollowingCompatRoutes(router *gin.Engine) {
	for _, prefix := range []string{"", "/api", "/api/v1"} {
		following := router.Group(prefix + "/following")
		following.POST("/create", h.requireAuthScope("write"), h.createFollowingCompat)
		following.POST("/delete", h.requireAuthScope("write"), h.deleteFollowingCompat)
		following.POST("/update", h.requireAuthScope("write"), h.updateFollowingCompat)
		following.POST("/update-all", h.requireAuthScope("write"), h.updateAllFollowingsCompat)
		router.POST(prefix+"/users/following", h.optionalAuth(), h.listFollowingEdgesCompat)
		router.POST(prefix+"/users/followers", h.optionalAuth(), h.listFollowerEdgesCompat)
	}
}

func (h *Handler) createFollowingCompat(c *gin.Context) {
	var request followingMutationRequest
	if !decodeSensitiveAccountCompatRequest(c, &request) || !validFollowingMutationRequest(request, followingMutationCreate) {
		writeFollowingCompatError(c, "Invalid param.", "INVALID_PARAM", "")
		return
	}
	targetID := request.UserID.Int64()
	if targetID == currentUserID(c) {
		writeFollowingCompatError(c, "Followee is yourself.", "FOLLOWEE_IS_YOURSELF", followingCreateSelfID)
		return
	}
	ctx, cancel := rpcContext(c)
	defer cancel()
	target, ok := h.followingCompatUser(c, ctx, targetID, followingCreateNoSuchUserID)
	if !ok {
		return
	}
	if h.clients.UserSafety != nil {
		relation, err := h.clients.UserSafety.GetSafetyRelation(ctx, &userpb.UserRelationRequest{ActorId: currentUserID(c), TargetId: targetID})
		if err == nil {
			switch {
			case relation.GetBlocked():
				writeFollowingCompatError(c, "You are blocking this user.", "BLOCKING", followingCreateBlockingID)
				return
			case relation.GetBlockedBy():
				writeFollowingCompatError(c, "You are blocked by this user.", "BLOCKED", followingCreateBlockedID)
				return
			}
		}
	}
	_, err := h.clients.User.Follow(ctx, &userpb.FollowRequest{
		FollowerId: currentUserID(c), FolloweeId: targetID, WithReplies: request.WithReplies,
	})
	if err != nil {
		writeFollowingMutationRPCError(c, err, followingMutationCreate)
		return
	}
	h.sanitizeUserProfileTheme(ctx, target)
	c.JSON(stdhttp.StatusOK, toMisskeyUserLite(target))
}

func (h *Handler) deleteFollowingCompat(c *gin.Context) {
	var request followingMutationRequest
	if !decodeSensitiveAccountCompatRequest(c, &request) || !validFollowingMutationRequest(request, followingMutationDelete) {
		writeFollowingCompatError(c, "Invalid param.", "INVALID_PARAM", "")
		return
	}
	targetID := request.UserID.Int64()
	if targetID == currentUserID(c) {
		writeFollowingCompatError(c, "Followee is yourself.", "FOLLOWEE_IS_YOURSELF", followingDeleteSelfID)
		return
	}
	ctx, cancel := rpcContext(c)
	defer cancel()
	target, ok := h.followingCompatUser(c, ctx, targetID, followingDeleteNoSuchUserID)
	if !ok {
		return
	}
	if _, err := h.clients.User.Unfollow(ctx, &userpb.FollowRequest{FollowerId: currentUserID(c), FolloweeId: targetID}); err != nil {
		writeFollowingMutationRPCError(c, err, followingMutationDelete)
		return
	}
	h.sanitizeUserProfileTheme(ctx, target)
	c.JSON(stdhttp.StatusOK, toMisskeyUserLite(target))
}

func (h *Handler) updateFollowingCompat(c *gin.Context) {
	var request followingMutationRequest
	if !decodeSensitiveAccountCompatRequest(c, &request) || !validFollowingMutationRequest(request, followingMutationUpdate) {
		writeFollowingCompatError(c, "Invalid param.", "INVALID_PARAM", "")
		return
	}
	targetID := request.UserID.Int64()
	if targetID == currentUserID(c) {
		writeFollowingCompatError(c, "Followee is yourself.", "FOLLOWEE_IS_YOURSELF", followingUpdateSelfID)
		return
	}
	ctx, cancel := rpcContext(c)
	defer cancel()
	followingClient, ok := h.followingCompatClient(c)
	if !ok {
		return
	}
	if _, ok := h.followingCompatUser(c, ctx, targetID, followingUpdateNoSuchUserID); !ok {
		return
	}
	if _, err := followingClient.UpdateFollowing(ctx, &userpb.UpdateFollowingRequest{
		FollowerId: currentUserID(c), FolloweeId: targetID, WithReplies: request.WithReplies, Notify: request.Notify,
	}); err != nil {
		writeFollowingMutationRPCError(c, err, followingMutationUpdate)
		return
	}
	current, ok := h.followingCompatUser(c, ctx, currentUserID(c), followingUpdateNoSuchUserID)
	if !ok {
		return
	}
	h.sanitizeUserProfileTheme(ctx, current)
	c.JSON(stdhttp.StatusOK, toMisskeyUserLite(current))
}

func (h *Handler) updateAllFollowingsCompat(c *gin.Context) {
	var request followingUpdateAllRequest
	if !decodeSensitiveAccountCompatRequest(c, &request) || !validFollowingNotify(request.Notify) {
		writeFollowingCompatError(c, "Invalid param.", "INVALID_PARAM", "")
		return
	}
	ctx, cancel := rpcContext(c)
	defer cancel()
	followingClient, ok := h.followingCompatClient(c)
	if !ok {
		return
	}
	if _, err := followingClient.UpdateAllFollowings(ctx, &userpb.UpdateAllFollowingsRequest{
		FollowerId: currentUserID(c), WithReplies: request.WithReplies, Notify: request.Notify,
	}); err != nil {
		if status.Code(err) == codes.InvalidArgument {
			writeFollowingCompatError(c, "Invalid param.", "INVALID_PARAM", "")
			return
		}
		writeRPCError(c, err)
		return
	}
	c.Status(stdhttp.StatusNoContent)
}

func (h *Handler) listFollowingEdgesCompat(c *gin.Context) {
	h.listFollowingCompat(c, false)
}

func (h *Handler) listFollowerEdgesCompat(c *gin.Context) {
	h.listFollowingCompat(c, true)
}

func (h *Handler) listFollowingCompat(c *gin.Context, followers bool) {
	var request followingListRequest
	if !decodeSensitiveAccountCompatRequest(c, &request) {
		writeFollowingCompatError(c, "Invalid param.", "INVALID_PARAM", "")
		return
	}
	limit := int32(10)
	if request.Limit != nil {
		limit = *request.Limit
	}
	if limit < 1 || limit > 100 || jsonIDValue(request.SinceID) < 0 || jsonIDValue(request.UntilID) < 0 {
		writeFollowingCompatError(c, "Invalid param.", "INVALID_PARAM", "")
		return
	}
	birthdaySet, birthday, validBirthday := decodeFollowingBirthday(request.Birthday)
	if !validBirthday {
		writeFollowingCompatError(c, "Birthday date format is invalid.", "BIRTHDAY_DATE_FORMAT_INVALID", followingListBirthdayInvalidID)
		return
	}
	if followers && birthdaySet {
		writeFollowingCompatError(c, "Invalid param.", "INVALID_PARAM", "")
		return
	}
	ctx, cancel := rpcContext(c)
	defer cancel()
	followingClient, ok := h.followingCompatClient(c)
	if !ok {
		return
	}
	user, ok := h.resolveFollowingListUser(c, ctx, request, followers)
	if !ok {
		return
	}
	rpcRequest := &userpb.ListFollowingEdgesRequest{
		UserId: user.GetId(), ViewerId: currentUserID(c), SinceId: jsonIDValue(request.SinceID), UntilId: jsonIDValue(request.UntilID), Limit: limit,
	}
	if birthdaySet && birthday != "" {
		rpcRequest.BirthdayMmdd = birthday[5:]
	}
	var responseEdges *userpb.FollowingListResponse
	var err error
	if followers {
		responseEdges, err = followingClient.ListFollowerEdges(ctx, rpcRequest)
	} else {
		responseEdges, err = followingClient.ListFollowingEdges(ctx, rpcRequest)
	}
	if err != nil {
		if status.Code(err) == codes.InvalidArgument {
			writeFollowingCompatError(c, "Invalid param.", "INVALID_PARAM", "")
			return
		}
		if status.Code(err) == codes.PermissionDenied {
			errorID := followingListForbiddenID
			if followers {
				errorID = followersListForbiddenID
			}
			writeFollowingCompatError(c, "Forbidden.", "FORBIDDEN", errorID)
			return
		}
		writeRPCError(c, err)
		return
	}
	items := make([]misskeyFollowing, 0, len(responseEdges.GetItems()))
	for _, edge := range responseEdges.GetItems() {
		items = append(items, toMisskeyFollowing(edge, followers))
	}
	c.JSON(stdhttp.StatusOK, items)
}

func validFollowingMutationRequest(request followingMutationRequest, kind followingMutationKind) bool {
	if request.UserID == nil || request.UserID.Int64() <= 0 {
		return false
	}
	if kind == followingMutationDelete && (request.WithReplies != nil || request.Notify != nil) {
		return false
	}
	if kind == followingMutationCreate && request.Notify != nil {
		return false
	}
	return validFollowingNotify(request.Notify)
}

func validFollowingNotify(notify *string) bool {
	return notify == nil || *notify == "normal" || *notify == "none"
}

func jsonIDValue(id *jsonInt64) int64 {
	if id == nil {
		return 0
	}
	return id.Int64()
}

func (h *Handler) followingCompatClient(c *gin.Context) (clients.UserFollowingClient, bool) {
	if h == nil || h.clients == nil || h.clients.UserFollowing == nil {
		writeError(c, stdhttp.StatusServiceUnavailable, "following service unavailable", "service_unavailable")
		return nil, false
	}
	return h.clients.UserFollowing, true
}

func (h *Handler) followingCompatUser(c *gin.Context, ctx context.Context, id int64, errorID string) (*userpb.UserInfo, bool) {
	result, err := h.clients.User.GetUser(ctx, &userpb.UserIDRequest{Id: id})
	if err != nil {
		if status.Code(err) == codes.NotFound {
			writeFollowingCompatError(c, "No such user.", "NO_SUCH_USER", errorID)
			return nil, false
		}
		writeRPCError(c, err)
		return nil, false
	}
	if result.GetUser() == nil {
		writeError(c, stdhttp.StatusBadGateway, "user service returned an empty user", "internal_error")
		return nil, false
	}
	return result.GetUser(), true
}

func (h *Handler) resolveFollowingListUser(c *gin.Context, ctx context.Context, request followingListRequest, followers bool) (*userpb.UserInfo, bool) {
	errorID := followingListNoSuchUserID
	if followers {
		errorID = followersListNoSuchUserID
	}
	if request.UserID != nil {
		if request.UserID.Int64() <= 0 {
			writeFollowingCompatError(c, "Invalid param.", "INVALID_PARAM", "")
			return nil, false
		}
		return h.followingCompatUser(c, ctx, request.UserID.Int64(), errorID)
	}
	username := strings.ToLower(strings.TrimSpace(request.Username))
	hostSet, local, valid := decodeFollowingHost(request.Host)
	if username == "" || len(username) > 256 || !hostSet || !valid {
		writeFollowingCompatError(c, "Invalid param.", "INVALID_PARAM", "")
		return nil, false
	}
	if !local {
		writeFollowingCompatError(c, "No such user.", "NO_SUCH_USER", errorID)
		return nil, false
	}
	result, err := h.clients.User.GetUserByUsername(ctx, &userpb.UsernameRequest{Username: username})
	if err != nil {
		if status.Code(err) == codes.NotFound {
			writeFollowingCompatError(c, "No such user.", "NO_SUCH_USER", errorID)
			return nil, false
		}
		writeRPCError(c, err)
		return nil, false
	}
	if result.GetUser() == nil {
		writeError(c, stdhttp.StatusBadGateway, "user service returned an empty user", "internal_error")
		return nil, false
	}
	return result.GetUser(), true
}

func decodeFollowingHost(raw json.RawMessage) (set bool, local bool, valid bool) {
	if len(bytes.TrimSpace(raw)) == 0 {
		return false, false, true
	}
	if bytes.Equal(bytes.TrimSpace(raw), []byte("null")) {
		return true, true, true
	}
	var host string
	if err := json.Unmarshal(raw, &host); err != nil || len(host) > 253 || strings.TrimSpace(host) == "" {
		return true, false, false
	}
	return true, false, true
}

func decodeFollowingBirthday(raw json.RawMessage) (set bool, value string, valid bool) {
	if len(bytes.TrimSpace(raw)) == 0 || bytes.Equal(bytes.TrimSpace(raw), []byte("null")) {
		return len(bytes.TrimSpace(raw)) > 0, "", true
	}
	if err := json.Unmarshal(raw, &value); err != nil || len(value) != 10 || value[4] != '-' || value[7] != '-' {
		return true, "", false
	}
	_, err := time.Parse("2006-01-02", value)
	return true, value, err == nil
}

func toMisskeyFollowing(edge *userpb.FollowingInfo, followers bool) misskeyFollowing {
	item := misskeyFollowing{
		ID: strconv.FormatInt(edge.GetId(), 10), CreatedAt: formatUnixMilli(edge.GetCreatedAt()),
		FolloweeID: strconv.FormatInt(edge.GetFolloweeId(), 10), FollowerID: strconv.FormatInt(edge.GetFollowerId(), 10),
		WithReplies: edge.GetWithReplies(), Notify: edge.GetNotify(),
	}
	if followers && edge.GetFollower() != nil {
		user := toMisskeyUserLite(edge.GetFollower())
		item.Follower = &user
	}
	if !followers && edge.GetFollowee() != nil {
		user := toMisskeyUserLite(edge.GetFollowee())
		item.Followee = &user
	}
	return item
}

type followingMutationKind int

const (
	followingMutationCreate followingMutationKind = iota
	followingMutationDelete
	followingMutationUpdate
)

func writeFollowingMutationRPCError(c *gin.Context, err error, kind followingMutationKind) {
	grpcStatus := status.Convert(err)
	message := strings.ToLower(strings.TrimSpace(grpcStatus.Message()))
	switch kind {
	case followingMutationCreate:
		switch {
		case grpcStatus.Code() == codes.NotFound:
			writeFollowingCompatError(c, "No such user.", "NO_SUCH_USER", followingCreateNoSuchUserID)
		case grpcStatus.Code() == codes.InvalidArgument && strings.Contains(message, "self"):
			writeFollowingCompatError(c, "Followee is yourself.", "FOLLOWEE_IS_YOURSELF", followingCreateSelfID)
		case grpcStatus.Code() == codes.AlreadyExists:
			writeFollowingCompatError(c, "Already following.", "ALREADY_FOLLOWING", followingCreateAlreadyID)
		case grpcStatus.Code() == codes.FailedPrecondition && strings.Contains(message, "blocked"):
			writeFollowingCompatError(c, "Following is blocked.", "BLOCKED", followingCreateBlockedID)
		case grpcStatus.Code() == codes.InvalidArgument:
			writeFollowingCompatError(c, "Invalid param.", "INVALID_PARAM", "")
		default:
			writeRPCError(c, err)
		}
	case followingMutationDelete:
		switch {
		case grpcStatus.Code() == codes.NotFound:
			writeFollowingCompatError(c, "No such user.", "NO_SUCH_USER", followingDeleteNoSuchUserID)
		case grpcStatus.Code() == codes.FailedPrecondition && strings.Contains(message, "not following"):
			writeFollowingCompatError(c, "Not following.", "NOT_FOLLOWING", followingDeleteNotFollowingID)
		case grpcStatus.Code() == codes.InvalidArgument && strings.Contains(message, "self"):
			writeFollowingCompatError(c, "Followee is yourself.", "FOLLOWEE_IS_YOURSELF", followingDeleteSelfID)
		case grpcStatus.Code() == codes.InvalidArgument:
			writeFollowingCompatError(c, "Invalid param.", "INVALID_PARAM", "")
		default:
			writeRPCError(c, err)
		}
	case followingMutationUpdate:
		switch {
		case grpcStatus.Code() == codes.NotFound:
			writeFollowingCompatError(c, "No such user.", "NO_SUCH_USER", followingUpdateNoSuchUserID)
		case grpcStatus.Code() == codes.FailedPrecondition && strings.Contains(message, "not following"):
			writeFollowingCompatError(c, "Not following.", "NOT_FOLLOWING", followingUpdateNotFollowingID)
		case grpcStatus.Code() == codes.InvalidArgument && strings.Contains(message, "self"):
			writeFollowingCompatError(c, "Followee is yourself.", "FOLLOWEE_IS_YOURSELF", followingUpdateSelfID)
		case grpcStatus.Code() == codes.InvalidArgument:
			writeFollowingCompatError(c, "Invalid param.", "INVALID_PARAM", "")
		default:
			writeRPCError(c, err)
		}
	}
}

func writeFollowingCompatError(c *gin.Context, message, code, errorID string) {
	apiError := newHTTPException(stdhttp.StatusBadRequest, message).WithMeta("legacy_code", code)
	if errorID != "" {
		apiError.WithMeta("error_id", errorID)
	}
	response.Failed(c, apiError)
}
