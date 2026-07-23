package http

import (
	"context"
	"net/http"
	"strconv"
	"strings"

	"api-gateway/api/proto/chatpb"
	"api-gateway/api/proto/userpb"
	"api-gateway/pkg/http/response"

	"github.com/gin-gonic/gin"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

const chatUserLookupLimit = 100

type chatCreateRoomRequest struct {
	Name string `json:"name"`
}

type chatSendMessageRequest struct {
	ClientMessageID string `json:"client_message_id"`
	Body            string `json:"body"`
}

type chatAdvanceReadRequest struct {
	ReadSeq jsonInt64 `json:"read_seq"`
}

type chatCreateGroupRequest struct {
	Name      string `json:"name"`
	SortOrder int32  `json:"sort_order"`
}

type chatUpdateGroupRequest struct {
	Name      string `json:"name"`
	SortOrder int32  `json:"sort_order"`
}

type chatPlaceRoomRequest struct {
	GroupID   jsonInt64 `json:"group_id"`
	SortOrder int32     `json:"sort_order"`
}

type chatAnnouncementRequest struct {
	Announcement string `json:"announcement"`
}

type chatAnnouncementSeenRequest struct {
	AnnouncementVersion jsonInt64 `json:"announcement_version"`
}

type chatUserView struct {
	ID        int64  `json:"id"`
	Username  string `json:"username"`
	Nickname  string `json:"nickname"`
	AvatarURL string `json:"avatar_url,omitempty"`
}

type chatRoomDetailsResponse struct {
	Details *chatpb.RoomDetails `json:"details"`
	Users   []chatUserView      `json:"users"`
}

type chatSidebarResponse struct {
	Groups []*chatpb.RoomGroup   `json:"groups"`
	Rooms  []*chatpb.SidebarRoom `json:"rooms"`
	Users  []chatUserView        `json:"users"`
}

type chatMessagePageResponse struct {
	Messages  []*chatpb.ChatMessage `json:"messages"`
	LatestSeq int64                 `json:"latest_seq"`
	AnchorSeq int64                 `json:"anchor_seq"`
	HasOlder  bool                  `json:"has_older"`
	HasNewer  bool                  `json:"has_newer"`
	Users     []chatUserView        `json:"users"`
}

type chatSendMessageResponse struct {
	Message   *chatpb.ChatMessage `json:"message"`
	LatestSeq int64               `json:"latest_seq"`
	Users     []chatUserView      `json:"users"`
}

type chatRoomResponse struct {
	Room  *chatpb.Room   `json:"room"`
	Users []chatUserView `json:"users"`
}

type chatRoomPreviewResponse struct {
	RoomNo      string `json:"room_no"`
	Name        string `json:"name"`
	Status      int32  `json:"status"`
	MemberCount int64  `json:"member_count"`
	Joined      bool   `json:"joined"`
}

func (h *Handler) registerChatRoutes(api *gin.RouterGroup) {
	chat := api.Group("/chat")
	auth := h.requireAuth()

	chat.POST("/rooms", auth, h.createChatRoom)
	chat.GET("/rooms/lookup", auth, h.lookupChatRoom)
	chat.GET("/rooms/:roomNo", auth, h.getChatRoom)
	chat.POST("/rooms/:roomNo/join", auth, h.joinChatRoom)
	chat.GET("/sidebar", auth, h.listChatSidebar)
	chat.GET("/rooms/:roomNo/messages", auth, h.listChatMessages)
	chat.POST("/rooms/:roomNo/messages", auth, h.sendChatMessage)
	chat.PUT("/rooms/:roomNo/read", auth, h.advanceChatRead)
	chat.POST("/groups", auth, h.createChatGroup)
	chat.PATCH("/groups/:groupId", auth, h.updateChatGroup)
	chat.DELETE("/groups/:groupId", auth, h.deleteChatGroup)
	chat.PUT("/rooms/:roomNo/placement", auth, h.placeChatRoom)
	chat.PATCH("/rooms/:roomNo/announcement", auth, h.updateChatAnnouncement)
	chat.PUT("/rooms/:roomNo/announcement-seen", auth, h.markChatAnnouncementSeen)
}

func (h *Handler) chatClientAvailable(c *gin.Context) bool {
	if h.clients != nil && h.clients.Chat != nil {
		return true
	}
	writeRPCError(c, status.Error(codes.Unavailable, "chat service unavailable"))
	return false
}

func (h *Handler) createChatRoom(c *gin.Context) {
	if !h.chatClientAvailable(c) {
		return
	}
	var req chatCreateRoomRequest
	if !bindJSON(c, &req) {
		return
	}
	ctx, cancel := rpcContext(c)
	defer cancel()
	resp, err := h.clients.Chat.CreateRoom(ctx, &chatpb.CreateRoomRequest{
		CreatorId: currentUserID(c),
		Name:      req.Name,
	})
	if err != nil {
		writeRPCError(c, err)
		return
	}
	details := resp.GetDetails()
	users, err := h.hydrateChatUsers(ctx, chatRoomDetailUserIDs(details))
	if err != nil {
		writeRPCError(c, err)
		return
	}
	response.Success(c, chatRoomDetailsResponse{Details: details, Users: users})
}

func (h *Handler) lookupChatRoom(c *gin.Context) {
	if !h.chatClientAvailable(c) {
		return
	}
	roomNo := strings.TrimSpace(c.Query("room_no"))
	if roomNo == "" {
		writeError(c, http.StatusBadRequest, "room_no is required", "bad_request")
		return
	}
	ctx, cancel := rpcContext(c)
	defer cancel()
	resp, err := h.clients.Chat.LookupRoom(ctx, &chatpb.LookupRoomRequest{
		RoomNo: roomNo,
		UserId: currentUserID(c),
	})
	if err != nil {
		writeRPCError(c, err)
		return
	}
	details := resp.GetDetails()
	if details == nil || details.GetRoom() == nil {
		writeRPCError(c, status.Error(codes.Internal, "chat room response is empty"))
		return
	}
	room := details.GetRoom()
	response.Success(c, chatRoomPreviewResponse{
		RoomNo:      room.GetRoomNo(),
		Name:        room.GetName(),
		Status:      room.GetStatus(),
		MemberCount: details.GetMemberCount(),
		Joined:      details.GetMembership() != nil,
	})
}

func (h *Handler) getChatRoom(c *gin.Context) {
	if !h.chatClientAvailable(c) {
		return
	}
	roomNo, ok := chatRoomNo(c)
	if !ok {
		return
	}
	ctx, cancel := rpcContext(c)
	defer cancel()
	resp, err := h.clients.Chat.LookupRoom(ctx, &chatpb.LookupRoomRequest{
		RoomNo: roomNo,
		UserId: currentUserID(c),
	})
	if err != nil {
		writeRPCError(c, err)
		return
	}
	details := resp.GetDetails()
	if details == nil || details.GetRoom() == nil {
		writeRPCError(c, status.Error(codes.Internal, "chat room response is empty"))
		return
	}
	if details.GetMembership() == nil {
		writeError(c, http.StatusForbidden, "chat membership required", "permission_denied")
		return
	}
	users, err := h.hydrateChatUsers(ctx, chatRoomDetailUserIDs(details))
	if err != nil {
		writeRPCError(c, err)
		return
	}
	response.Success(c, chatRoomDetailsResponse{Details: details, Users: users})
}

func (h *Handler) joinChatRoom(c *gin.Context) {
	if !h.chatClientAvailable(c) {
		return
	}
	roomNo, ok := chatRoomNo(c)
	if !ok {
		return
	}
	ctx, cancel := rpcContext(c)
	defer cancel()
	resp, err := h.clients.Chat.JoinRoom(ctx, &chatpb.JoinRoomRequest{
		RoomNo: roomNo,
		UserId: currentUserID(c),
	})
	if err != nil {
		writeRPCError(c, err)
		return
	}
	details := resp.GetDetails()
	users, err := h.hydrateChatUsers(ctx, chatRoomDetailUserIDs(details))
	if err != nil {
		writeRPCError(c, err)
		return
	}
	response.Success(c, chatRoomDetailsResponse{Details: details, Users: users})
}

func (h *Handler) listChatSidebar(c *gin.Context) {
	if !h.chatClientAvailable(c) {
		return
	}
	ctx, cancel := rpcContext(c)
	defer cancel()
	resp, err := h.clients.Chat.ListSidebar(ctx, &chatpb.ListSidebarRequest{UserId: currentUserID(c)})
	if err != nil {
		writeRPCError(c, err)
		return
	}
	users, err := h.hydrateChatUsers(ctx, chatSidebarUserIDs(resp))
	if err != nil {
		writeRPCError(c, err)
		return
	}
	groups := resp.GetGroups()
	rooms := resp.GetRooms()
	if groups == nil {
		groups = []*chatpb.RoomGroup{}
	}
	if rooms == nil {
		rooms = []*chatpb.SidebarRoom{}
	}
	response.Success(c, chatSidebarResponse{Groups: groups, Rooms: rooms, Users: users})
}

func (h *Handler) listChatMessages(c *gin.Context) {
	if !h.chatClientAvailable(c) {
		return
	}
	roomNo, ok := chatRoomNo(c)
	if !ok {
		return
	}
	req, ok := chatMessageRequest(c, roomNo)
	if !ok {
		return
	}
	ctx, cancel := rpcContext(c)
	defer cancel()
	resp, err := h.clients.Chat.ListMessages(ctx, req)
	if err != nil {
		writeRPCError(c, err)
		return
	}
	users, err := h.hydrateChatUsers(ctx, chatMessageUserIDs(resp.GetMessages()))
	if err != nil {
		writeRPCError(c, err)
		return
	}
	messages := resp.GetMessages()
	if messages == nil {
		messages = []*chatpb.ChatMessage{}
	}
	response.Success(c, chatMessagePageResponse{
		Messages: messages, LatestSeq: resp.GetLatestSeq(), AnchorSeq: resp.GetAnchorSeq(),
		HasOlder: resp.GetHasOlder(), HasNewer: resp.GetHasNewer(), Users: users,
	})
}

func (h *Handler) sendChatMessage(c *gin.Context) {
	if !h.chatClientAvailable(c) {
		return
	}
	roomNo, ok := chatRoomNo(c)
	if !ok {
		return
	}
	var body chatSendMessageRequest
	if !bindJSON(c, &body) {
		return
	}
	ctx, cancel := rpcContext(c)
	defer cancel()
	resp, err := h.clients.Chat.SendMessage(ctx, &chatpb.SendMessageRequest{
		RoomNo:          roomNo,
		UserId:          currentUserID(c),
		ClientMessageId: body.ClientMessageID,
		Body:            body.Body,
	})
	if err != nil {
		writeRPCError(c, err)
		return
	}
	users, err := h.hydrateChatUsers(ctx, chatMessageUserIDs([]*chatpb.ChatMessage{resp.GetMessage()}))
	if err != nil {
		writeRPCError(c, err)
		return
	}
	response.Success(c, chatSendMessageResponse{Message: resp.GetMessage(), LatestSeq: resp.GetLatestSeq(), Users: users})
}

func (h *Handler) advanceChatRead(c *gin.Context) {
	if !h.chatClientAvailable(c) {
		return
	}
	roomNo, ok := chatRoomNo(c)
	if !ok {
		return
	}
	var body chatAdvanceReadRequest
	if !bindJSON(c, &body) {
		return
	}
	ctx, cancel := rpcContext(c)
	defer cancel()
	resp, err := h.clients.Chat.AdvanceRead(ctx, &chatpb.AdvanceReadRequest{
		RoomNo: roomNo, UserId: currentUserID(c), ReadSeq: body.ReadSeq.Int64(),
	})
	if err != nil {
		writeRPCError(c, err)
		return
	}
	response.Success(c, resp)
}

func (h *Handler) createChatGroup(c *gin.Context) {
	if !h.chatClientAvailable(c) {
		return
	}
	var body chatCreateGroupRequest
	if !bindJSON(c, &body) {
		return
	}
	ctx, cancel := rpcContext(c)
	defer cancel()
	resp, err := h.clients.Chat.CreateGroup(ctx, &chatpb.CreateGroupRequest{
		UserId: currentUserID(c), Name: body.Name, SortOrder: body.SortOrder,
	})
	if err != nil {
		writeRPCError(c, err)
		return
	}
	response.Success(c, resp)
}

func (h *Handler) updateChatGroup(c *gin.Context) {
	if !h.chatClientAvailable(c) {
		return
	}
	groupID, ok := pathInt64(c, "groupId")
	if !ok {
		return
	}
	var body chatUpdateGroupRequest
	if !bindJSON(c, &body) {
		return
	}
	ctx, cancel := rpcContext(c)
	defer cancel()
	resp, err := h.clients.Chat.UpdateGroup(ctx, &chatpb.UpdateGroupRequest{
		UserId: currentUserID(c), GroupId: groupID, Name: body.Name, SortOrder: body.SortOrder,
	})
	if err != nil {
		writeRPCError(c, err)
		return
	}
	response.Success(c, resp)
}

func (h *Handler) deleteChatGroup(c *gin.Context) {
	if !h.chatClientAvailable(c) {
		return
	}
	groupID, ok := pathInt64(c, "groupId")
	if !ok {
		return
	}
	ctx, cancel := rpcContext(c)
	defer cancel()
	resp, err := h.clients.Chat.DeleteGroup(ctx, &chatpb.DeleteGroupRequest{UserId: currentUserID(c), GroupId: groupID})
	if err != nil {
		writeRPCError(c, err)
		return
	}
	response.Success(c, resp)
}

func (h *Handler) placeChatRoom(c *gin.Context) {
	if !h.chatClientAvailable(c) {
		return
	}
	roomNo, ok := chatRoomNo(c)
	if !ok {
		return
	}
	var body chatPlaceRoomRequest
	if !bindJSON(c, &body) {
		return
	}
	ctx, cancel := rpcContext(c)
	defer cancel()
	resp, err := h.clients.Chat.PlaceRoom(ctx, &chatpb.PlaceRoomRequest{
		RoomNo: roomNo, UserId: currentUserID(c), GroupId: body.GroupID.Int64(), SortOrder: body.SortOrder,
	})
	if err != nil {
		writeRPCError(c, err)
		return
	}
	response.Success(c, resp)
}

func (h *Handler) updateChatAnnouncement(c *gin.Context) {
	if !h.chatClientAvailable(c) {
		return
	}
	roomNo, ok := chatRoomNo(c)
	if !ok {
		return
	}
	var body chatAnnouncementRequest
	if !bindJSON(c, &body) {
		return
	}
	ctx, cancel := rpcContext(c)
	defer cancel()
	resp, err := h.clients.Chat.UpdateAnnouncement(ctx, &chatpb.UpdateAnnouncementRequest{
		RoomNo: roomNo, UserId: currentUserID(c), Announcement: body.Announcement,
	})
	if err != nil {
		writeRPCError(c, err)
		return
	}
	room := resp.GetRoom()
	users, err := h.hydrateChatUsers(ctx, chatRoomUserIDs(room))
	if err != nil {
		writeRPCError(c, err)
		return
	}
	response.Success(c, chatRoomResponse{Room: room, Users: users})
}

func (h *Handler) markChatAnnouncementSeen(c *gin.Context) {
	if !h.chatClientAvailable(c) {
		return
	}
	roomNo, ok := chatRoomNo(c)
	if !ok {
		return
	}
	var body chatAnnouncementSeenRequest
	if !bindJSON(c, &body) {
		return
	}
	ctx, cancel := rpcContext(c)
	defer cancel()
	resp, err := h.clients.Chat.MarkAnnouncementSeen(ctx, &chatpb.MarkAnnouncementSeenRequest{
		RoomNo: roomNo, UserId: currentUserID(c), AnnouncementVersion: body.AnnouncementVersion.Int64(),
	})
	if err != nil {
		writeRPCError(c, err)
		return
	}
	response.Success(c, resp)
}

func chatRoomNo(c *gin.Context) (string, bool) {
	roomNo := strings.TrimSpace(c.Param("roomNo"))
	if roomNo == "" {
		writeError(c, http.StatusBadRequest, "invalid roomNo", "bad_request")
		return "", false
	}
	return roomNo, true
}

func chatMessageRequest(c *gin.Context, roomNo string) (*chatpb.ListMessagesRequest, bool) {
	request := &chatpb.ListMessagesRequest{RoomNo: roomNo, UserId: currentUserID(c)}
	var ok bool
	if request.AnchorSeq, ok = chatQueryInt64(c, "anchor_seq"); !ok {
		return nil, false
	}
	if request.BeforeSeq, ok = chatQueryInt64(c, "before_seq"); !ok {
		return nil, false
	}
	if request.AfterSeq, ok = chatQueryInt64(c, "after_seq"); !ok {
		return nil, false
	}
	if request.Before, ok = chatQueryInt32(c, "before"); !ok {
		return nil, false
	}
	if request.After, ok = chatQueryInt32(c, "after"); !ok {
		return nil, false
	}
	if request.Limit, ok = chatQueryInt32(c, "limit"); !ok {
		return nil, false
	}
	if request.AnchorSeq < 0 || request.BeforeSeq < 0 || request.AfterSeq < 0 || request.Before < 0 || request.After < 0 || request.Limit < 0 {
		writeError(c, http.StatusBadRequest, "pagination values cannot be negative", "bad_request")
		return nil, false
	}
	if request.BeforeSeq > 0 && request.AfterSeq > 0 {
		writeError(c, http.StatusBadRequest, "before_seq and after_seq are mutually exclusive", "bad_request")
		return nil, false
	}
	if (request.BeforeSeq > 0 || request.AfterSeq > 0) && (request.AnchorSeq > 0 || request.Before > 0 || request.After > 0) {
		writeError(c, http.StatusBadRequest, "directional and anchor pagination are mutually exclusive", "bad_request")
		return nil, false
	}
	return request, true
}

func chatQueryInt64(c *gin.Context, name string) (int64, bool) {
	value, exists := c.GetQuery(name)
	if !exists || strings.TrimSpace(value) == "" {
		return 0, true
	}
	parsed, err := strconv.ParseInt(strings.TrimSpace(value), 10, 64)
	if err != nil {
		writeError(c, http.StatusBadRequest, "invalid "+name, "bad_request")
		return 0, false
	}
	return parsed, true
}

func chatQueryInt32(c *gin.Context, name string) (int32, bool) {
	value, exists := c.GetQuery(name)
	if !exists || strings.TrimSpace(value) == "" {
		return 0, true
	}
	parsed, err := strconv.ParseInt(strings.TrimSpace(value), 10, 32)
	if err != nil {
		writeError(c, http.StatusBadRequest, "invalid "+name, "bad_request")
		return 0, false
	}
	return int32(parsed), true
}

func chatRoomDetailUserIDs(details *chatpb.RoomDetails) []int64 {
	if details == nil {
		return nil
	}
	return chatRoomUserIDs(details.GetRoom())
}

func chatRoomUserIDs(room *chatpb.Room) []int64 {
	if room == nil || room.GetCreatorId() <= 0 {
		return nil
	}
	return []int64{room.GetCreatorId()}
}

func chatSidebarUserIDs(sidebar *chatpb.SidebarResponse) []int64 {
	if sidebar == nil {
		return nil
	}
	ids := make([]int64, 0, len(sidebar.GetRooms())*2)
	for _, item := range sidebar.GetRooms() {
		if item == nil {
			continue
		}
		if room := item.GetRoom(); room != nil {
			ids = append(ids, room.GetCreatorId())
		}
		if message := item.GetLastMessage(); message != nil {
			ids = append(ids, message.GetSenderId())
		}
	}
	return ids
}

func chatMessageUserIDs(messages []*chatpb.ChatMessage) []int64 {
	ids := make([]int64, 0, len(messages))
	for _, message := range messages {
		if message != nil {
			ids = append(ids, message.GetSenderId())
		}
	}
	return ids
}

func (h *Handler) hydrateChatUsers(ctx context.Context, ids []int64) ([]chatUserView, error) {
	ordered := make([]int64, 0, len(ids))
	seen := make(map[int64]struct{}, len(ids))
	for _, id := range ids {
		if id <= 0 {
			continue
		}
		if _, exists := seen[id]; exists {
			continue
		}
		seen[id] = struct{}{}
		ordered = append(ordered, id)
	}
	if len(ordered) == 0 {
		return []chatUserView{}, nil
	}
	if len(ordered) > chatUserLookupLimit {
		ordered = ordered[:chatUserLookupLimit]
	}
	if h.clients == nil || h.clients.User == nil {
		return nil, status.Error(codes.Unavailable, "user service unavailable")
	}
	resp, err := h.clients.User.ListUsers(ctx, &userpb.ListUsersRequest{
		Ids: ordered, Page: 1, PageSize: int32(len(ordered)),
	})
	if err != nil {
		return nil, err
	}
	usersByID := make(map[int64]*userpb.UserInfo, len(resp.GetItems()))
	for _, user := range resp.GetItems() {
		if user == nil || user.GetId() <= 0 {
			continue
		}
		if _, requested := seen[user.GetId()]; requested {
			usersByID[user.GetId()] = user
		}
	}
	users := make([]chatUserView, 0, len(usersByID))
	for _, id := range ordered {
		user := usersByID[id]
		if user == nil {
			continue
		}
		users = append(users, chatUserView{
			ID: user.GetId(), Username: user.GetUsername(), Nickname: user.GetNickname(), AvatarURL: user.GetAvatarUrl(),
		})
	}
	return users, nil
}
