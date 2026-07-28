package http

import (
	"context"
	"net/http"
	"strconv"

	"api-gateway/api/proto/adminpb"
	"api-gateway/api/proto/chatpb"
	"api-gateway/internal/popularity"
	"api-gateway/pkg/http/response"

	"github.com/gin-gonic/gin"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

const (
	chatRoomStatusActive        int32 = 1
	popularResourceStatusActive int32 = 2
	popularResourcePageSize     int32 = 100
)

type popularChatRoomView struct {
	RoomNo      string `json:"room_no"`
	Name        string `json:"name"`
	MemberCount string `json:"member_count"`
	Score       string `json:"score"`
}

type popularResourceView struct {
	ID          int64  `json:"id"`
	Key         string `json:"key"`
	Title       string `json:"title"`
	URL         string `json:"url"`
	Description string `json:"description"`
	Score       string `json:"score"`
}

func (h *Handler) listPopularChatRooms(c *gin.Context) {
	if h == nil || h.popularity == nil {
		response.Success(c, gin.H{"items": []popularChatRoomView{}})
		return
	}
	if !h.chatClientAvailable(c) {
		return
	}
	ctx, cancel := rpcContext(c)
	defer cancel()
	entries, err := h.popularity.ListChatRooms(ctx, int(queryInt32(c, "limit", 5)))
	if err != nil {
		writeError(c, http.StatusServiceUnavailable, "popular chat rooms unavailable", "service_unavailable")
		return
	}
	items := make([]popularChatRoomView, 0, len(entries))
	for _, entry := range entries {
		room, ok, err := h.lookupPopularChatRoom(ctx, entry)
		if err != nil {
			writeRPCError(c, err)
			return
		}
		if ok {
			items = append(items, room)
		}
	}
	response.Success(c, gin.H{"items": items})
}

func (h *Handler) lookupPopularChatRoom(ctx context.Context, entry popularity.Entry) (popularChatRoomView, bool, error) {
	resp, err := h.clients.Chat.LookupRoom(ctx, &chatpb.LookupRoomRequest{RoomNo: entry.Key})
	if status.Code(err) == codes.NotFound {
		return popularChatRoomView{}, false, nil
	}
	if err != nil {
		return popularChatRoomView{}, false, err
	}
	details := resp.GetDetails()
	room := details.GetRoom()
	if room == nil || room.GetStatus() != chatRoomStatusActive {
		return popularChatRoomView{}, false, nil
	}
	return popularChatRoomView{
		RoomNo:      room.GetRoomNo(),
		Name:        room.GetName(),
		MemberCount: chatInt64String(details.GetMemberCount()),
		Score:       chatInt64String(entry.Score),
	}, true, nil
}

func (h *Handler) listPopularResources(c *gin.Context) {
	if h == nil || h.popularity == nil {
		response.Success(c, gin.H{"items": []popularResourceView{}})
		return
	}
	entries, err := h.popularity.ListResources(c.Request.Context(), int(queryInt32(c, "limit", 5)))
	if err != nil {
		writeError(c, http.StatusServiceUnavailable, "popular resources unavailable", "service_unavailable")
		return
	}
	if len(entries) == 0 {
		response.Success(c, gin.H{"items": []popularResourceView{}})
		return
	}
	if h.clients == nil || h.clients.Admin == nil {
		writeError(c, http.StatusServiceUnavailable, "admin service unavailable", "service_unavailable")
		return
	}
	ctx, cancel := rpcContext(c)
	defer cancel()
	linksByID, err := h.loadPopularResourceLinks(ctx, popularResourceIDs(entries))
	if err != nil {
		writeRPCError(c, err)
		return
	}
	items := make([]popularResourceView, 0, len(entries))
	for _, entry := range entries {
		id, err := strconv.ParseInt(entry.Key, 10, 64)
		if err != nil || id <= 0 {
			continue
		}
		link := linksByID[id]
		if link == nil {
			continue
		}
		items = append(items, popularResourceView{
			ID:          link.GetId(),
			Key:         link.GetKey(),
			Title:       link.GetTitle(),
			URL:         link.GetUrl(),
			Description: link.GetDescription(),
			Score:       chatInt64String(entry.Score),
		})
	}
	response.Success(c, gin.H{"items": items})
}

func popularResourceIDs(entries []popularity.Entry) map[int64]struct{} {
	ids := make(map[int64]struct{}, len(entries))
	for _, entry := range entries {
		id, err := strconv.ParseInt(entry.Key, 10, 64)
		if err == nil && id > 0 {
			ids[id] = struct{}{}
		}
	}
	return ids
}

func (h *Handler) loadPopularResourceLinks(ctx context.Context, wanted map[int64]struct{}) (map[int64]*adminpb.LinkInfo, error) {
	linksByID := make(map[int64]*adminpb.LinkInfo, len(wanted))
	if len(wanted) == 0 {
		return linksByID, nil
	}
	for offset := int32(0); ; offset += popularResourcePageSize {
		resp, err := h.clients.Admin.ListLinks(ctx, &adminpb.ListLinksRequest{
			Status: popularResourceStatusActive,
			Limit:  popularResourcePageSize,
			Offset: offset,
		})
		if err != nil {
			return nil, err
		}
		items := resp.GetItems()
		for _, link := range items {
			if link == nil || link.GetId() <= 0 || link.GetStatus() != popularResourceStatusActive {
				continue
			}
			if _, ok := wanted[link.GetId()]; ok {
				linksByID[link.GetId()] = link
			}
		}
		if len(linksByID) == len(wanted) || len(items) == 0 || len(items) < int(popularResourcePageSize) {
			break
		}
		if total := resp.GetTotal(); total <= int64(offset+popularResourcePageSize) {
			break
		}
	}
	return linksByID, nil
}

func (h *Handler) recordLinkVisit(c *gin.Context) {
	id, ok := pathInt64(c, "id")
	if !ok {
		return
	}
	if h != nil && h.popularity != nil {
		_ = h.popularity.RecordResourceVisit(c.Request.Context(), id)
	}
	response.Success(c, gin.H{"success": true})
}
