package http

import (
	"net/http"
	"sort"
	"strings"

	"api-gateway/api/proto/feedpb"
	"api-gateway/api/proto/userpb"
	"api-gateway/pkg/http/response"

	"github.com/gin-gonic/gin"
)

const (
	userListDefaultPageSize int32 = 20
	userListMaxPageSize     int32 = 100
	userListNameMaxRunes          = 100
)

type userListMutationRequest struct {
	Name     string `json:"name"`
	IsPublic bool   `json:"is_public"`
}

type userListMemberMutationRequest struct {
	UserID jsonInt64 `json:"user_id"`
}

type userListView struct {
	ID            string `json:"id"`
	OwnerID       string `json:"owner_id"`
	Name          string `json:"name"`
	IsPublic      bool   `json:"is_public"`
	MemberCount   int64  `json:"member_count"`
	FavoriteCount int64  `json:"favorite_count"`
	IsFavorited   bool   `json:"is_favorited"`
	CreatedAt     int64  `json:"created_at"`
	UpdatedAt     int64  `json:"updated_at"`
}

func (h *Handler) listCurrentUserLists(c *gin.Context) {
	h.listUserListsForOwner(c, currentUserID(c))
}

func (h *Handler) listUserLists(c *gin.Context) {
	ownerID, ok := pathInt64(c, "id")
	if !ok {
		return
	}
	h.listUserListsForOwner(c, ownerID)
}

func (h *Handler) listUserListsForOwner(c *gin.Context, ownerID int64) {
	if !h.userListClientAvailable(c) {
		return
	}
	ctx, cancel := rpcContext(c)
	defer cancel()
	resp, err := h.clients.UserLists.ListUserLists(ctx, &userpb.ListUserListsRequest{
		ViewerId: currentUserID(c),
		OwnerId:  ownerID,
		Page:     userListPage(c),
		PageSize: userListPageSize(c),
	})
	if err != nil {
		writeRPCError(c, err)
		return
	}
	response.Success(c, gin.H{"items": toUserListViews(resp.GetItems()), "total": resp.GetTotal()})
}

func (h *Handler) listFavoriteUserLists(c *gin.Context) {
	if !h.userListClientAvailable(c) {
		return
	}
	ctx, cancel := rpcContext(c)
	defer cancel()
	resp, err := h.clients.UserLists.ListFavoriteUserLists(ctx, &userpb.ListFavoriteUserListsRequest{
		UserId: currentUserID(c), Page: userListPage(c), PageSize: userListPageSize(c),
	})
	if err != nil {
		writeRPCError(c, err)
		return
	}
	response.Success(c, gin.H{"items": toUserListViews(resp.GetItems()), "total": resp.GetTotal()})
}

func (h *Handler) createUserList(c *gin.Context) {
	if !h.userListClientAvailable(c) {
		return
	}
	var req userListMutationRequest
	if !bindJSON(c, &req) {
		return
	}
	name, ok := validateUserListName(c, req.Name)
	if !ok {
		return
	}
	ctx, cancel := rpcContext(c)
	defer cancel()
	resp, err := h.clients.UserLists.CreateUserList(ctx, &userpb.CreateUserListRequest{
		OwnerId: currentUserID(c), Name: name, IsPublic: req.IsPublic,
	})
	if err != nil {
		writeRPCError(c, err)
		return
	}
	response.Success(c, userListResponsePayload(resp))
}

func (h *Handler) getUserList(c *gin.Context) {
	if !h.userListClientAvailable(c) {
		return
	}
	listID, ok := pathInt64(c, "id")
	if !ok {
		return
	}
	ctx, cancel := rpcContext(c)
	defer cancel()
	resp, err := h.clients.UserLists.GetUserList(ctx, &userpb.GetUserListRequest{ViewerId: currentUserID(c), ListId: listID})
	if err != nil {
		writeRPCError(c, err)
		return
	}
	response.Success(c, userListResponsePayload(resp))
}

func (h *Handler) updateUserList(c *gin.Context) {
	if !h.userListClientAvailable(c) {
		return
	}
	listID, ok := pathInt64(c, "id")
	if !ok {
		return
	}
	var req userListMutationRequest
	if !bindJSON(c, &req) {
		return
	}
	name, ok := validateUserListName(c, req.Name)
	if !ok {
		return
	}
	ctx, cancel := rpcContext(c)
	defer cancel()
	resp, err := h.clients.UserLists.UpdateUserList(ctx, &userpb.UpdateUserListRequest{
		OwnerId: currentUserID(c), ListId: listID, Name: name, IsPublic: req.IsPublic,
	})
	if err != nil {
		writeRPCError(c, err)
		return
	}
	response.Success(c, userListResponsePayload(resp))
}

func (h *Handler) deleteUserList(c *gin.Context) {
	if !h.userListClientAvailable(c) {
		return
	}
	listID, ok := pathInt64(c, "id")
	if !ok {
		return
	}
	ctx, cancel := rpcContext(c)
	defer cancel()
	resp, err := h.clients.UserLists.DeleteUserList(ctx, &userpb.DeleteUserListRequest{OwnerId: currentUserID(c), ListId: listID})
	if err != nil {
		writeRPCError(c, err)
		return
	}
	response.Success(c, resp)
}

func (h *Handler) listUserListMembers(c *gin.Context) {
	if !h.userListClientAvailable(c) {
		return
	}
	listID, ok := pathInt64(c, "id")
	if !ok {
		return
	}
	ctx, cancel := rpcContext(c)
	defer cancel()
	resp, err := h.clients.UserLists.ListUserListMembers(ctx, &userpb.ListUserListMembersRequest{
		ViewerId: currentUserID(c), ListId: listID, Page: userListPage(c), PageSize: userListPageSize(c),
	})
	if err != nil {
		writeRPCError(c, err)
		return
	}
	h.sanitizeUserProfileThemes(ctx, resp.GetItems())
	response.Success(c, toPublicUserListResponse(resp))
}

func (h *Handler) addUserListMember(c *gin.Context) {
	h.mutateUserListMember(c, true)
}

func (h *Handler) removeUserListMember(c *gin.Context) {
	h.mutateUserListMember(c, false)
}

func (h *Handler) mutateUserListMember(c *gin.Context, add bool) {
	if !h.userListClientAvailable(c) {
		return
	}
	listID, ok := pathInt64(c, "id")
	if !ok {
		return
	}
	var req userListMemberMutationRequest
	if !bindJSON(c, &req) {
		return
	}
	if req.UserID.Int64() <= 0 {
		writeError(c, http.StatusBadRequest, "invalid user_id", "invalid_argument")
		return
	}
	ctx, cancel := rpcContext(c)
	defer cancel()
	rpcReq := &userpb.UserListMemberRequest{OwnerId: currentUserID(c), ListId: listID, UserId: req.UserID.Int64()}
	var (
		resp *userpb.SimpleResponse
		err  error
	)
	if add {
		resp, err = h.clients.UserLists.AddUserListMember(ctx, rpcReq)
	} else {
		resp, err = h.clients.UserLists.RemoveUserListMember(ctx, rpcReq)
	}
	if err != nil {
		writeRPCError(c, err)
		return
	}
	response.Success(c, resp)
}

func (h *Handler) copyUserList(c *gin.Context) {
	if !h.userListClientAvailable(c) {
		return
	}
	listID, ok := pathInt64(c, "id")
	if !ok {
		return
	}
	var req struct {
		Name string `json:"name"`
	}
	if !bindJSON(c, &req) {
		return
	}
	name, ok := validateUserListName(c, req.Name)
	if !ok {
		return
	}
	ctx, cancel := rpcContext(c)
	defer cancel()
	resp, err := h.clients.UserLists.CopyUserList(ctx, &userpb.CopyUserListRequest{
		OwnerId: currentUserID(c), SourceListId: listID, Name: name,
	})
	if err != nil {
		writeRPCError(c, err)
		return
	}
	response.Success(c, userListResponsePayload(resp))
}

func (h *Handler) favoriteUserList(c *gin.Context) {
	h.mutateUserListFavorite(c, true)
}

func (h *Handler) unfavoriteUserList(c *gin.Context) {
	h.mutateUserListFavorite(c, false)
}

func (h *Handler) mutateUserListFavorite(c *gin.Context, favorite bool) {
	if !h.userListClientAvailable(c) {
		return
	}
	listID, ok := pathInt64(c, "id")
	if !ok {
		return
	}
	ctx, cancel := rpcContext(c)
	defer cancel()
	rpcReq := &userpb.UserListFavoriteRequest{UserId: currentUserID(c), ListId: listID}
	var (
		resp *userpb.UserListInfoResponse
		err  error
	)
	if favorite {
		resp, err = h.clients.UserLists.FavoriteUserList(ctx, rpcReq)
	} else {
		resp, err = h.clients.UserLists.UnfavoriteUserList(ctx, rpcReq)
	}
	if err != nil {
		writeRPCError(c, err)
		return
	}
	response.Success(c, userListResponsePayload(resp))
}

func (h *Handler) userListFeed(c *gin.Context) {
	if !h.userListClientAvailable(c) {
		return
	}
	if h.clients == nil || h.clients.Feed == nil {
		writeError(c, http.StatusServiceUnavailable, "feed service unavailable", "service_unavailable")
		return
	}
	listID, ok := pathInt64(c, "id")
	if !ok {
		return
	}
	viewerID := currentUserID(c)
	ctx, cancel := rpcContext(c)
	defer cancel()
	if _, err := h.clients.UserLists.GetUserList(ctx, &userpb.GetUserListRequest{ViewerId: viewerID, ListId: listID}); err != nil {
		writeRPCError(c, err)
		return
	}
	members, err := h.clients.UserLists.ListUserListMembers(ctx, &userpb.ListUserListMembersRequest{
		ViewerId: viewerID, ListId: listID, Page: 1, PageSize: userListMaxPageSize,
	})
	if err != nil {
		writeRPCError(c, err)
		return
	}
	hidden := map[int64]struct{}{}
	if viewerID > 0 {
		hidden, err = h.hiddenUserIDSet(ctx, viewerID)
		if err != nil {
			writeRPCError(c, err)
			return
		}
	}
	authorIDs := make([]int64, 0, len(members.GetItems()))
	for _, member := range members.GetItems() {
		if member == nil || member.GetId() <= 0 {
			continue
		}
		if _, excluded := hidden[member.GetId()]; !excluded {
			authorIDs = append(authorIDs, member.GetId())
		}
	}
	if len(authorIDs) == 0 {
		response.Success(c, &feedpb.FeedListResponse{Items: []*feedpb.FeedItem{}})
		return
	}
	sort.Slice(authorIDs, func(i, j int) bool { return authorIDs[i] < authorIDs[j] })
	resp, err := h.clients.Feed.ListLatest(ctx, &feedpb.ListFeedRequest{
		Limit: normalizeFeedLimit(queryInt32(c, "limit", 20)), Offset: normalizeFeedOffset(queryInt32(c, "offset", 0)), AuthorIds: authorIDs,
	})
	if err != nil {
		writeRPCError(c, err)
		return
	}
	response.Success(c, resp)
}

func (h *Handler) userListClientAvailable(c *gin.Context) bool {
	if h != nil && h.clients != nil && h.clients.UserLists != nil {
		return true
	}
	writeError(c, http.StatusServiceUnavailable, "user list service unavailable", "service_unavailable")
	return false
}

func validateUserListName(c *gin.Context, raw string) (string, bool) {
	name := strings.TrimSpace(raw)
	if name == "" || len([]rune(name)) > userListNameMaxRunes {
		writeError(c, http.StatusBadRequest, "user list name must be between 1 and 100 characters", "invalid_argument")
		return "", false
	}
	return name, true
}

func userListPage(c *gin.Context) int32 {
	page := queryInt32(c, "page", 1)
	if page <= 0 {
		return 1
	}
	return page
}

func userListPageSize(c *gin.Context) int32 {
	pageSize := queryInt32(c, "page_size", userListDefaultPageSize)
	if pageSize <= 0 {
		return userListDefaultPageSize
	}
	if pageSize > userListMaxPageSize {
		return userListMaxPageSize
	}
	return pageSize
}

func userListResponsePayload(resp *userpb.UserListInfoResponse) gin.H {
	if resp == nil {
		return gin.H{}
	}
	list, _ := toUserListView(resp.GetUserList())
	return gin.H{"success": resp.GetSuccess(), "message": resp.GetMessage(), "user_list": list}
}

func toUserListViews(items []*userpb.UserListInfo) []userListView {
	views := make([]userListView, 0, len(items))
	for _, item := range items {
		if view, ok := toUserListView(item); ok {
			views = append(views, view)
		}
	}
	return views
}

func toUserListView(item *userpb.UserListInfo) (userListView, bool) {
	if item == nil {
		return userListView{}, false
	}
	return userListView{
		ID: itemIDString(item.GetId()), OwnerID: itemIDString(item.GetOwnerId()), Name: item.GetName(), IsPublic: item.GetIsPublic(),
		MemberCount: item.GetMemberCount(), FavoriteCount: item.GetFavoriteCount(), IsFavorited: item.GetIsFavorited(),
		CreatedAt: item.GetCreatedAt(), UpdatedAt: item.GetUpdatedAt(),
	}, true
}
