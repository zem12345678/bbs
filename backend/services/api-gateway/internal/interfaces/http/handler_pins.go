package http

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"

	"api-gateway/api/proto/contentpb"
	"api-gateway/api/proto/reactionpb"
	"api-gateway/pkg/http/response"

	"github.com/gin-gonic/gin"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/encoding/protojson"
)

type pinNoteRequest struct {
	NoteID jsonInt64 `json:"noteId"`
}

type pinnedContentView struct {
	ID         string                 `json:"id"`
	EntityType string                 `json:"entity_type"`
	CreatedAt  int64                  `json:"created_at"`
	Article    *contentpb.ArticleInfo `json:"article,omitempty"`
	Topic      *contentpb.TopicInfo   `json:"topic,omitempty"`
}

// MarshalJSON keeps protobuf int64 fields as JSON strings so browser clients
// do not lose Snowflake-sized content or author IDs.
func (v pinnedContentView) MarshalJSON() ([]byte, error) {
	type wireView struct {
		ID         string          `json:"id"`
		EntityType string          `json:"entity_type"`
		CreatedAt  int64           `json:"created_at"`
		Article    json.RawMessage `json:"article,omitempty"`
		Topic      json.RawMessage `json:"topic,omitempty"`
	}
	view := wireView{ID: v.ID, EntityType: v.EntityType, CreatedAt: v.CreatedAt}
	if v.Article != nil {
		data, err := (protojson.MarshalOptions{UseProtoNames: true}).Marshal(v.Article)
		if err != nil {
			return nil, err
		}
		view.Article = data
	}
	if v.Topic != nil {
		data, err := (protojson.MarshalOptions{UseProtoNames: true}).Marshal(v.Topic)
		if err != nil {
			return nil, err
		}
		view.Topic = data
	}
	return json.Marshal(view)
}

func (h *Handler) pinNote(c *gin.Context) {
	h.mutatePin(c, true)
}

func (h *Handler) unpinNote(c *gin.Context) {
	h.mutatePin(c, false)
}

func (h *Handler) mutatePin(c *gin.Context, pin bool) {
	if h == nil || h.clients == nil || h.clients.Reaction == nil || (pin && h.clients.Content == nil) {
		writeError(c, http.StatusServiceUnavailable, "pin services unavailable", "service_unavailable")
		return
	}
	var req pinNoteRequest
	if !bindJSON(c, &req) || req.NoteID.Int64() <= 0 {
		writeError(c, http.StatusBadRequest, "noteId must be a positive integer", "invalid_argument")
		return
	}
	userID := currentUserID(c)
	if !h.allowPinActionRateLimit(c, userID) {
		return
	}
	ctx, cancel := rpcContext(c)
	defer cancel()
	var (
		ref string
		ok  bool
	)
	if pin {
		ref, ok = h.resolvePinTarget(c, ctx, req.NoteID.Int64(), userID)
	} else {
		ref, ok = h.resolveUnpinTarget(c, ctx, req.NoteID.Int64(), userID)
	}
	if !ok {
		return
	}
	request := &reactionpb.ReactRequest{
		Entity: &reactionpb.EntityRef{EntityType: ref, EntityId: req.NoteID.Int64()},
		UserId: userID,
	}
	var err error
	if pin {
		_, err = h.clients.Reaction.Pin(ctx, request)
	} else {
		_, err = h.clients.Reaction.Unpin(ctx, request)
	}
	if err != nil {
		writeRPCError(c, err)
		return
	}
	response.Success(c, gin.H{"success": true, "message": "ok"})
}

func (h *Handler) resolveUnpinTarget(c *gin.Context, ctx context.Context, id, userID int64) (string, bool) {
	resp, err := h.clients.Reaction.ListPins(ctx, &reactionpb.ListPinsRequest{UserId: userID, Limit: 100})
	if err != nil {
		writeRPCError(c, err)
		return "", false
	}
	var entityTypes []string
	for _, pin := range resp.GetItems() {
		if entity := pin.GetEntity(); entity != nil && entity.GetEntityId() == id {
			entityTypes = append(entityTypes, entity.GetEntityType())
		}
	}
	if len(entityTypes) == 1 {
		return entityTypes[0], true
	}
	if len(entityTypes) > 1 {
		writeError(c, http.StatusConflict, "note id matches multiple pinned content items", "ambiguous_note_id")
		return "", false
	}
	if h.clients.Content == nil {
		writeError(c, http.StatusServiceUnavailable, "content service unavailable", "service_unavailable")
		return "", false
	}
	return h.resolvePinTarget(c, ctx, id, userID)
}

func (h *Handler) resolvePinTarget(c *gin.Context, ctx context.Context, id, viewerID int64) (string, bool) {
	articleResp, articleErr := h.clients.Content.GetArticle(ctx, &contentpb.GetArticleRequest{Key: &contentpb.GetArticleRequest_Id{Id: id}})
	topicResp, topicErr := h.clients.Content.GetTopic(ctx, &contentpb.GetTopicRequest{Key: &contentpb.GetTopicRequest_Id{Id: id}, ViewerUserId: viewerID})
	article := articleResp.GetArticle()
	topic := topicResp.GetTopic()
	if articleErr != nil && status.Code(articleErr) != codes.NotFound {
		writeRPCError(c, articleErr)
		return "", false
	}
	if topicErr != nil && status.Code(topicErr) != codes.NotFound {
		writeRPCError(c, topicErr)
		return "", false
	}
	if article != nil && topic != nil {
		writeError(c, http.StatusConflict, "note id matches both an article and a topic", "ambiguous_note_id")
		return "", false
	}
	if article == nil && topic == nil {
		writeError(c, http.StatusNotFound, "note not found", "not_found")
		return "", false
	}
	if article != nil {
		if !pinTargetVisible(article.GetAuthorId(), article.GetStatus(), viewerID) {
			writeError(c, http.StatusNotFound, "note not found", "not_found")
			return "", false
		}
		return "article", true
	}
	if !pinTargetVisible(topic.GetAuthorId(), topic.GetStatus(), viewerID) {
		writeError(c, http.StatusNotFound, "note not found", "not_found")
		return "", false
	}
	return "topic", true
}

func pinTargetVisible(authorID int64, contentStatus int32, viewerID int64) bool {
	return contentStatus != contentStatusArchived && (authorID == viewerID || contentStatus == contentStatusPublished)
}

func (h *Handler) listCurrentUserPinned(c *gin.Context) {
	h.listPinnedForUser(c, currentUserID(c), currentUserID(c))
}

func (h *Handler) listUserPinned(c *gin.Context) {
	userID, ok := pathInt64(c, "id")
	if !ok {
		return
	}
	h.listPinnedForUser(c, userID, currentUserID(c))
}

func (h *Handler) listPinnedForUser(c *gin.Context, userID, viewerID int64) {
	if h == nil || h.clients == nil || h.clients.Reaction == nil || h.clients.Content == nil {
		writeError(c, http.StatusServiceUnavailable, "pin services unavailable", "service_unavailable")
		return
	}
	ctx, cancel := rpcContext(c)
	defer cancel()
	resp, err := h.clients.Reaction.ListPins(ctx, &reactionpb.ListPinsRequest{UserId: userID, Limit: 100})
	if err != nil {
		writeRPCError(c, err)
		return
	}
	items := make([]pinnedContentView, 0, len(resp.GetItems()))
	for _, pin := range resp.GetItems() {
		item, include, err := h.loadPinnedContent(ctx, pin, viewerID)
		if err != nil {
			writeRPCError(c, err)
			return
		}
		if include {
			items = append(items, item)
		}
	}
	response.Success(c, gin.H{"items": items, "total": int64(len(items))})
}

func (h *Handler) loadPinnedContent(ctx context.Context, pin *reactionpb.PinInfo, viewerID int64) (pinnedContentView, bool, error) {
	if pin == nil || pin.GetEntity() == nil {
		return pinnedContentView{}, false, nil
	}
	entity := pin.GetEntity()
	item := pinnedContentView{
		ID: itemIDString(entity.GetEntityId()), EntityType: entity.GetEntityType(), CreatedAt: pin.GetCreatedAt(),
	}
	switch entity.GetEntityType() {
	case "article":
		resp, err := h.clients.Content.GetArticle(ctx, &contentpb.GetArticleRequest{Key: &contentpb.GetArticleRequest_Id{Id: entity.GetEntityId()}})
		if err != nil {
			if status.Code(err) == codes.NotFound {
				return pinnedContentView{}, false, nil
			}
			return pinnedContentView{}, false, err
		}
		article := resp.GetArticle()
		if article == nil || !pinTargetVisible(article.GetAuthorId(), article.GetStatus(), viewerID) {
			return pinnedContentView{}, false, nil
		}
		item.Article = article
	case "topic":
		resp, err := h.clients.Content.GetTopic(ctx, &contentpb.GetTopicRequest{Key: &contentpb.GetTopicRequest_Id{Id: entity.GetEntityId()}, ViewerUserId: viewerID})
		if err != nil {
			if status.Code(err) == codes.NotFound {
				return pinnedContentView{}, false, nil
			}
			return pinnedContentView{}, false, err
		}
		topic := resp.GetTopic()
		if topic == nil || !pinTargetVisible(topic.GetAuthorId(), topic.GetStatus(), viewerID) {
			return pinnedContentView{}, false, nil
		}
		item.Topic = topic
	default:
		return pinnedContentView{}, false, errors.New("invalid pinned entity type")
	}
	return item, true, nil
}
