package http

import (
	"net/http"
	"strconv"
	"strings"

	"api-gateway/api/proto/reactionpb"
	"api-gateway/pkg/http/response"

	"github.com/gin-gonic/gin"
)

const (
	collectionNameMaxLength              = 100
	collectionDescriptionMaxLength       = 2048
	collectionDefaultLimit         int32 = 20
	collectionMaxLimit             int32 = 100
)

type collectionView struct {
	ID          string `json:"id"`
	UserID      string `json:"user_id"`
	Name        string `json:"name"`
	Description string `json:"description"`
	IsPublic    bool   `json:"is_public"`
	ItemCount   int64  `json:"item_count"`
	CreatedAt   int64  `json:"created_at"`
	UpdatedAt   int64  `json:"updated_at"`
}

type collectionEntityView struct {
	EntityType string `json:"entity_type"`
	EntityID   string `json:"entity_id"`
}

type collectionItemView struct {
	ID           string               `json:"id"`
	CollectionID string               `json:"collection_id"`
	Entity       collectionEntityView `json:"entity"`
	CreatedAt    int64                `json:"created_at"`
}

func (h *Handler) listCurrentUserCollections(c *gin.Context) {
	if !h.collectionClientAvailable(c) {
		return
	}
	ctx, cancel := rpcContext(c)
	defer cancel()
	resp, err := h.clients.Reaction.ListCollections(ctx, &reactionpb.ListCollectionsRequest{
		UserId: currentUserID(c), Limit: collectionLimit(c), Offset: collectionOffset(c),
	})
	if err != nil {
		writeRPCError(c, err)
		return
	}
	response.Success(c, gin.H{
		"items": toCollectionViews(resp.GetItems()),
		"total": resp.GetTotal(),
	})
}

func (h *Handler) createCurrentUserCollection(c *gin.Context) {
	if !h.collectionClientAvailable(c) {
		return
	}
	var req createCollectionRequest
	if !bindJSON(c, &req) {
		return
	}
	name, description, ok := validateCollectionFields(c, req.Name, req.Description)
	if !ok {
		return
	}
	ctx, cancel := rpcContext(c)
	defer cancel()
	resp, err := h.clients.Reaction.CreateCollection(ctx, &reactionpb.CreateCollectionRequest{
		UserId: currentUserID(c), Name: name, Description: description, IsPublic: req.IsPublic,
	})
	if err != nil {
		writeRPCError(c, err)
		return
	}
	response.Success(c, collectionResponsePayload(resp))
}

func (h *Handler) updateCurrentUserCollection(c *gin.Context) {
	if !h.collectionClientAvailable(c) {
		return
	}
	id, ok := pathInt64(c, "id")
	if !ok {
		return
	}
	var req updateCollectionRequest
	if !bindJSON(c, &req) {
		return
	}
	name, description, ok := validateCollectionFields(c, req.Name, req.Description)
	if !ok {
		return
	}
	ctx, cancel := rpcContext(c)
	defer cancel()
	resp, err := h.clients.Reaction.UpdateCollection(ctx, &reactionpb.UpdateCollectionRequest{
		UserId: currentUserID(c), Id: id, Name: name, Description: description, IsPublic: req.IsPublic,
	})
	if err != nil {
		writeRPCError(c, err)
		return
	}
	response.Success(c, collectionResponsePayload(resp))
}

func (h *Handler) deleteCurrentUserCollection(c *gin.Context) {
	if !h.collectionClientAvailable(c) {
		return
	}
	id, ok := pathInt64(c, "id")
	if !ok {
		return
	}
	ctx, cancel := rpcContext(c)
	defer cancel()
	resp, err := h.clients.Reaction.DeleteCollection(ctx, &reactionpb.DeleteCollectionRequest{UserId: currentUserID(c), Id: id})
	if err != nil {
		writeRPCError(c, err)
		return
	}
	response.Success(c, resp)
}

func (h *Handler) listCurrentUserCollectionItems(c *gin.Context) {
	if !h.collectionClientAvailable(c) {
		return
	}
	collectionID, ok := pathInt64(c, "id")
	if !ok {
		return
	}
	entityType, ok := collectionEntityType(c.Query("entity_type"))
	if c.Query("entity_type") != "" && !ok {
		writeError(c, http.StatusBadRequest, "invalid entity_type", "invalid_argument")
		return
	}
	ctx, cancel := rpcContext(c)
	defer cancel()
	resp, err := h.clients.Reaction.ListCollectionItems(ctx, &reactionpb.ListCollectionItemsRequest{
		UserId: currentUserID(c), CollectionId: collectionID, EntityType: entityType,
		Limit: collectionLimit(c), Offset: collectionOffset(c),
	})
	if err != nil {
		writeRPCError(c, err)
		return
	}
	response.Success(c, gin.H{
		"items": toCollectionItemViews(resp.GetItems()),
		"total": resp.GetTotal(),
	})
}

func (h *Handler) addCurrentUserCollectionItem(c *gin.Context) {
	h.mutateCurrentUserCollectionItem(c, true)
}

func (h *Handler) removeCurrentUserCollectionItem(c *gin.Context) {
	h.mutateCurrentUserCollectionItem(c, false)
}

func (h *Handler) mutateCurrentUserCollectionItem(c *gin.Context, add bool) {
	if !h.collectionClientAvailable(c) {
		return
	}
	collectionID, ok := pathInt64(c, "id")
	if !ok {
		return
	}
	var req collectionItemRequest
	if !bindJSON(c, &req) {
		return
	}
	entityType, valid := collectionEntityType(req.EntityType)
	if !valid || req.EntityID.Int64() <= 0 {
		writeError(c, http.StatusBadRequest, "invalid collection item", "invalid_argument")
		return
	}
	ctx, cancel := rpcContext(c)
	defer cancel()
	if add {
		if h.clients == nil || h.clients.Content == nil {
			writeError(c, http.StatusServiceUnavailable, "content service unavailable", "service_unavailable")
			return
		}
		if !h.requirePublishedContentTarget(c, ctx, entityType, req.EntityID.Int64()) {
			return
		}
	}
	itemReq := &reactionpb.CollectionItemRequest{
		UserId: currentUserID(c), CollectionId: collectionID,
		Entity: &reactionpb.EntityRef{EntityType: entityType, EntityId: req.EntityID.Int64()},
	}
	var (
		resp *reactionpb.CollectionActionResponse
		err  error
	)
	if add {
		resp, err = h.clients.Reaction.AddCollectionItem(ctx, itemReq)
	} else {
		resp, err = h.clients.Reaction.RemoveCollectionItem(ctx, itemReq)
	}
	if err != nil {
		writeRPCError(c, err)
		return
	}
	response.Success(c, resp)
}

func (h *Handler) collectionClientAvailable(c *gin.Context) bool {
	if h != nil && h.clients != nil && h.clients.Reaction != nil {
		return true
	}
	writeError(c, http.StatusServiceUnavailable, "reaction service unavailable", "service_unavailable")
	return false
}

func validateCollectionFields(c *gin.Context, rawName string, rawDescription string) (string, string, bool) {
	name := strings.TrimSpace(rawName)
	description := strings.TrimSpace(rawDescription)
	if name == "" || len([]rune(name)) > collectionNameMaxLength {
		writeError(c, http.StatusBadRequest, "collection name must be between 1 and 100 characters", "invalid_argument")
		return "", "", false
	}
	if len([]rune(description)) > collectionDescriptionMaxLength {
		writeError(c, http.StatusBadRequest, "collection description is too long", "invalid_argument")
		return "", "", false
	}
	return name, description, true
}

func collectionEntityType(raw string) (string, bool) {
	value := strings.ToLower(strings.TrimSpace(raw))
	switch value {
	case "article", "topic":
		return value, true
	default:
		return "", false
	}
}

func collectionLimit(c *gin.Context) int32 {
	value := queryInt32(c, "limit", collectionDefaultLimit)
	if value <= 0 {
		return collectionDefaultLimit
	}
	if value > collectionMaxLimit {
		return collectionMaxLimit
	}
	return value
}

func collectionOffset(c *gin.Context) int32 {
	value := queryInt32(c, "offset", 0)
	if value < 0 {
		return 0
	}
	return value
}

func collectionResponsePayload(resp *reactionpb.CollectionResponse) gin.H {
	if resp == nil {
		return gin.H{}
	}
	collection, _ := toCollectionView(resp.GetCollection())
	return gin.H{
		"success":    resp.GetSuccess(),
		"message":    resp.GetMessage(),
		"collection": collection,
	}
}

func toCollectionViews(items []*reactionpb.CollectionInfo) []collectionView {
	out := make([]collectionView, 0, len(items))
	for _, item := range items {
		if view, ok := toCollectionView(item); ok {
			out = append(out, view)
		}
	}
	return out
}

func toCollectionView(item *reactionpb.CollectionInfo) (collectionView, bool) {
	if item == nil {
		return collectionView{}, false
	}
	return collectionView{
		ID: itemIDString(item.GetId()), UserID: itemIDString(item.GetUserId()), Name: item.GetName(),
		Description: item.GetDescription(), IsPublic: item.GetIsPublic(), ItemCount: item.GetItemCount(),
		CreatedAt: item.GetCreatedAt(), UpdatedAt: item.GetUpdatedAt(),
	}, true
}

func toCollectionItemViews(items []*reactionpb.CollectionItemInfo) []collectionItemView {
	out := make([]collectionItemView, 0, len(items))
	for _, item := range items {
		if item == nil {
			continue
		}
		entity := item.GetEntity()
		entityView := collectionEntityView{}
		if entity != nil {
			entityView = collectionEntityView{EntityType: entity.GetEntityType(), EntityID: itemIDString(entity.GetEntityId())}
		}
		out = append(out, collectionItemView{
			ID: itemIDString(item.GetId()), CollectionID: itemIDString(item.GetCollectionId()),
			Entity: entityView, CreatedAt: item.GetCreatedAt(),
		})
	}
	return out
}

func itemIDString(value int64) string {
	if value <= 0 {
		return ""
	}
	return strconv.FormatInt(value, 10)
}
