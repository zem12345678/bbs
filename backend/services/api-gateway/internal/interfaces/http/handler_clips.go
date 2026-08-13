package http

import (
	"context"
	stdhttp "net/http"
	"strconv"
	"strings"

	"api-gateway/api/proto/contentpb"
	"api-gateway/api/proto/reactionpb"
	"api-gateway/api/proto/userpb"

	"github.com/gin-gonic/gin"
)

type clipCreateRequest struct {
	Name        string  `json:"name"`
	Description *string `json:"description"`
	IsPublic    bool    `json:"isPublic"`
}

type clipUpdateRequest struct {
	ClipID      jsonInt64 `json:"clipId"`
	Name        *string   `json:"name"`
	Description *string   `json:"description"`
	IsPublic    *bool     `json:"isPublic"`
}

type clipIDRequest struct {
	ClipID jsonInt64 `json:"clipId"`
}
type clipNoteRequest struct {
	ClipID jsonInt64 `json:"clipId"`
	NoteID jsonInt64 `json:"noteId"`
	Limit  *int32 `json:"limit"`
	SinceID jsonInt64 `json:"sinceId"`
	UntilID jsonInt64 `json:"untilId"`
}

type publicClipListRequest struct {
	UserID  jsonInt64 `json:"userId"`
	Limit   *int32    `json:"limit"`
	SinceID jsonInt64 `json:"sinceId"`
	UntilID jsonInt64 `json:"untilId"`
}

type noteClipsRequest struct {
	NoteID jsonInt64 `json:"noteId"`
}

type misskeyClip struct {
	ID             string          `json:"id"`
	CreatedAt      string          `json:"createdAt"`
	LastClippedAt  *string         `json:"lastClippedAt"`
	UserID         string          `json:"userId"`
	User           misskeyUserLite `json:"user"`
	Name           string          `json:"name"`
	Description    *string         `json:"description"`
	IsPublic       bool            `json:"isPublic"`
	FavoritedCount int64           `json:"favoritedCount"`
	IsFavorited    *bool           `json:"isFavorited,omitempty"`
	NotesCount     int64           `json:"notesCount,omitempty"`
}

type misskeyClipNote struct {
	ID                 string            `json:"id"`
	ThreadID           string            `json:"threadId"`
	CreatedAt          string            `json:"createdAt"`
	Text               string            `json:"text"`
	CW                 *string           `json:"cw"`
	UserID             string            `json:"userId"`
	UserHost           *string           `json:"userHost"`
	User               misskeyUserLite   `json:"user"`
	ReplyID            *string           `json:"replyId"`
	RenoteID           *string           `json:"renoteId"`
	Visibility         string            `json:"visibility"`
	Mentions           []string          `json:"mentions"`
	VisibleUserIDs     []string          `json:"visibleUserIds"`
	FileIDs            []string          `json:"fileIds"`
	Files              []any             `json:"files"`
	Tags               []string          `json:"tags"`
	IsMutingThread     bool              `json:"isMutingThread"`
	IsMutingNote       bool              `json:"isMutingNote"`
	IsFavorited        bool              `json:"isFavorited"`
	IsRenoted          bool              `json:"isRenoted"`
	BypassSilence      bool              `json:"bypassSilence"`
	Emojis             map[string]string `json:"emojis"`
	ReactionAcceptance *string           `json:"reactionAcceptance"`
	ReactionEmojis     map[string]string `json:"reactionEmojis"`
	Reactions          map[string]int64  `json:"reactions"`
	ReactionCount      int64             `json:"reactionCount"`
	RenoteCount        int64             `json:"renoteCount"`
	RepliesCount       int64             `json:"repliesCount"`
	ViewsCount         int64             `json:"viewsCount"`
}

func (h *Handler) createClip(c *gin.Context) {
	var req clipCreateRequest
	if !bindJSON(c, &req) {
		return
	}
	name := strings.TrimSpace(req.Name)
	description := ""
	if req.Description != nil {
		description = strings.TrimSpace(*req.Description)
	}
	if name == "" || len([]rune(name)) > collectionNameMaxLength || len([]rune(description)) > collectionDescriptionMaxLength {
		writeError(c, stdhttp.StatusBadRequest, "invalid clip fields", "invalid_argument")
		return
	}
	ctx, cancel := rpcContext(c)
	defer cancel()
	resp, err := h.clients.Reaction.CreateCollection(ctx, &reactionpb.CreateCollectionRequest{UserId: currentUserID(c), Name: name, Description: description, IsPublic: req.IsPublic})
	if err != nil {
		writeRPCError(c, err)
		return
	}
	clip, ok := h.clipFromCollection(c, ctx, resp.GetCollection(), currentUserID(c), false)
	if !ok {
		return
	}
	c.JSON(stdhttp.StatusOK, clip)
}

func (h *Handler) updateClip(c *gin.Context) {
	var req clipUpdateRequest
	if !bindJSON(c, &req) || req.ClipID.Int64() <= 0 {
		writeError(c, stdhttp.StatusBadRequest, "clipId must be a positive integer", "invalid_argument")
		return
	}
	ctx, cancel := rpcContext(c)
	defer cancel()
	current, err := h.clients.Reaction.GetCollection(ctx, &reactionpb.GetCollectionRequest{Id: req.ClipID.Int64(), ViewerUserId: currentUserID(c)})
	if err != nil { writeRPCError(c, err); return }
	if current.GetCollection() == nil { writeError(c, stdhttp.StatusNotFound, "clip not found", "not_found"); return }
	name := current.GetCollection().GetName()
	if req.Name != nil { name = strings.TrimSpace(*req.Name) }
	description := current.GetCollection().GetDescription()
	if req.Description != nil { description = strings.TrimSpace(*req.Description) }
	isPublic := current.GetCollection().GetIsPublic()
	if req.IsPublic != nil { isPublic = *req.IsPublic }
	resp, err := h.clients.Reaction.UpdateCollection(ctx, &reactionpb.UpdateCollectionRequest{UserId: currentUserID(c), Id: req.ClipID.Int64(), Name: name, Description: description, IsPublic: isPublic})
	if err != nil {
		writeRPCError(c, err)
		return
	}
	clip, ok := h.clipFromCollection(c, ctx, resp.GetCollection(), currentUserID(c), false)
	if !ok {
		return
	}
	c.JSON(stdhttp.StatusOK, clip)
}

func (h *Handler) deleteClip(c *gin.Context) {
	var req clipIDRequest
	if !bindJSON(c, &req) || req.ClipID.Int64() <= 0 {
		writeError(c, stdhttp.StatusBadRequest, "clipId must be a positive integer", "invalid_argument")
		return
	}
	ctx, cancel := rpcContext(c)
	defer cancel()
	if _, err := h.clients.Reaction.DeleteCollection(ctx, &reactionpb.DeleteCollectionRequest{UserId: currentUserID(c), Id: req.ClipID.Int64()}); err != nil {
		writeRPCError(c, err)
		return
	}
	c.Status(stdhttp.StatusNoContent)
}

func (h *Handler) listClips(c *gin.Context) {
	ctx, cancel := rpcContext(c)
	defer cancel()
	resp, err := h.clients.Reaction.ListCollections(ctx, &reactionpb.ListCollectionsRequest{UserId: currentUserID(c), Limit: collectionMaxLimit, Offset: 0})
	if err != nil {
		writeRPCError(c, err)
		return
	}
	out := make([]misskeyClip, 0, len(resp.GetItems()))
	for _, item := range resp.GetItems() {
		clip, ok := h.clipFromCollection(c, ctx, item, currentUserID(c), false)
		if !ok {
			return
		}
		out = append(out, clip)
	}
	c.JSON(stdhttp.StatusOK, out)
}

func (h *Handler) showClip(c *gin.Context) {
	var req clipIDRequest
	if !bindJSON(c, &req) || req.ClipID.Int64() <= 0 {
		writeError(c, stdhttp.StatusBadRequest, "clipId must be a positive integer", "invalid_argument")
		return
	}
	ctx, cancel := rpcContext(c)
	defer cancel()
	resp, err := h.clients.Reaction.GetCollection(ctx, &reactionpb.GetCollectionRequest{Id: req.ClipID.Int64(), ViewerUserId: currentUserID(c)})
	if err != nil {
		writeRPCError(c, err)
		return
	}
	clip, ok := h.clipFromCollection(c, ctx, resp.GetCollection(), currentUserID(c), true)
	if !ok {
		return
	}
	c.JSON(stdhttp.StatusOK, clip)
}

func (h *Handler) addClipNote(c *gin.Context)    { h.mutateClipNote(c, true) }
func (h *Handler) removeClipNote(c *gin.Context) { h.mutateClipNote(c, false) }
func (h *Handler) mutateClipNote(c *gin.Context, add bool) {
	var req clipNoteRequest
	if !bindJSON(c, &req) || req.ClipID.Int64() <= 0 || req.NoteID.Int64() <= 0 {
		writeError(c, stdhttp.StatusBadRequest, "clipId and noteId must be positive integers", "invalid_argument")
		return
	}
	ctx, cancel := rpcContext(c)
	defer cancel()
	entityType := "article"
	if add {
		var ok bool
		entityType, ok = h.resolvePublishedClipNote(c, ctx, req.NoteID.Int64())
		if !ok { return }
	}
	item := &reactionpb.CollectionItemRequest{UserId: currentUserID(c), CollectionId: req.ClipID.Int64(), Entity: &reactionpb.EntityRef{EntityType: entityType, EntityId: req.NoteID.Int64()}}
	var err error
	if add {
		_, err = h.clients.Reaction.AddCollectionItem(ctx, item)
	} else {
		_, err = h.clients.Reaction.RemoveCollectionItem(ctx, item)
		if err == nil {
			otherType := "topic"
			if entityType == otherType { otherType = "article" }
			_, err = h.clients.Reaction.RemoveCollectionItem(ctx, &reactionpb.CollectionItemRequest{UserId: currentUserID(c), CollectionId: req.ClipID.Int64(), Entity: &reactionpb.EntityRef{EntityType: otherType, EntityId: req.NoteID.Int64()}})
		}
	}
	if err != nil {
		writeRPCError(c, err)
		return
	}
	c.Status(stdhttp.StatusNoContent)
}

func (h *Handler) listClipNotes(c *gin.Context) {
	var req clipNoteRequest
	if !bindJSON(c, &req) || req.ClipID.Int64() <= 0 {
		writeError(c, stdhttp.StatusBadRequest, "clipId must be a positive integer", "invalid_argument")
		return
	}
	limit := collectionDefaultLimit
	if req.Limit != nil { limit = *req.Limit } else if v, ok := strictQueryInt32(c, "limit", collectionDefaultLimit); ok { limit = v }
	if limit < 1 || limit > 100 {
		writeError(c, stdhttp.StatusBadRequest, "limit must be between 1 and 100", "invalid_argument")
		return
	}
	ctx, cancel := rpcContext(c)
	defer cancel()
	resp, err := h.clients.Reaction.ListPublicCollectionItems(ctx, &reactionpb.ListPublicCollectionItemsRequest{CollectionId: req.ClipID.Int64(), ViewerUserId: currentUserID(c), Limit: limit})
	if err != nil {
		writeRPCError(c, err)
		return
	}
	notes := make([]misskeyClipNote, 0, len(resp.GetItems()))
	for _, item := range resp.GetItems() {
		note, ok := h.clipNoteFromItem(c, ctx, item)
		if !ok {
			return
		}
		notes = append(notes, note)
	}
	c.JSON(stdhttp.StatusOK, notes)
}

func (h *Handler) mutateClipFavorite(c *gin.Context) {
	var req clipIDRequest
	if !bindJSON(c, &req) || req.ClipID.Int64() <= 0 { writeError(c, stdhttp.StatusBadRequest, "clipId must be a positive integer", "invalid_argument"); return }
	ctx, cancel := rpcContext(c); defer cancel()
	if _, err := h.clients.Reaction.GetCollection(ctx, &reactionpb.GetCollectionRequest{Id: req.ClipID.Int64(), ViewerUserId: currentUserID(c)}); err != nil { writeRPCError(c, err); return }
	ref := &reactionpb.EntityRef{EntityType: "collection", EntityId: req.ClipID.Int64()}
	var err error
	if strings.HasSuffix(c.Request.URL.Path, "/unfavorite") { _, err = h.clients.Reaction.Unfavorite(ctx, &reactionpb.ReactRequest{Entity: ref, UserId: currentUserID(c)}) } else { _, err = h.clients.Reaction.Favorite(ctx, &reactionpb.ReactRequest{Entity: ref, UserId: currentUserID(c)}) }
	if err != nil { writeRPCError(c, err); return }
	c.Status(stdhttp.StatusNoContent)
}

func (h *Handler) listFavoriteClips(c *gin.Context) {
	ctx, cancel := rpcContext(c)
	defer cancel()
	resp, err := h.clients.Reaction.ListFavorites(ctx, &reactionpb.ListFavoritesRequest{UserId: currentUserID(c), EntityType: "collection", Limit: 100})
	if err != nil { writeRPCError(c, err); return }
	out := make([]misskeyClip, 0, len(resp.GetItems()))
	for _, favorite := range resp.GetItems() {
		collection, err := h.clients.Reaction.GetCollection(ctx, &reactionpb.GetCollectionRequest{Id: favorite.GetEntity().GetEntityId(), ViewerUserId: currentUserID(c)})
		if err != nil { writeRPCError(c, err); return }
		clip, ok := h.clipFromCollection(c, ctx, collection.GetCollection(), currentUserID(c), false)
		if !ok { return }
		out = append(out, clip)
	}
	c.JSON(stdhttp.StatusOK, out)
}

func (h *Handler) listPublicClips(c *gin.Context) {
	var req publicClipListRequest
	if !bindJSON(c, &req) || req.UserID.Int64() <= 0 {
		writeError(c, stdhttp.StatusBadRequest, "userId must be a positive integer", "invalid_argument")
		return
	}
	limit := int32(10)
	if req.Limit != nil {
		limit = *req.Limit
	}
	if limit < 1 || limit > 100 || req.SinceID.Int64() < 0 || req.UntilID.Int64() < 0 {
		writeError(c, stdhttp.StatusBadRequest, "limit must be between 1 and 100 and cursors must be non-negative", "invalid_argument")
		return
	}
	ctx, cancel := rpcContext(c)
	defer cancel()
	resp, err := h.clients.Reaction.ListPublicCollections(ctx, &reactionpb.ListPublicCollectionsRequest{
		UserId: req.UserID.Int64(), Limit: limit, SinceId: req.SinceID.Int64(), UntilId: req.UntilID.Int64(), ViewerUserId: currentUserID(c),
	})
	if err != nil {
		writeRPCError(c, err)
		return
	}
	h.writePublicClipList(c, ctx, resp.GetItems())
}

func (h *Handler) listNoteClips(c *gin.Context) {
	var req noteClipsRequest
	if !bindJSON(c, &req) || req.NoteID.Int64() <= 0 {
		writeError(c, stdhttp.StatusBadRequest, "noteId must be a positive integer", "invalid_argument")
		return
	}
	ctx, cancel := rpcContext(c)
	defer cancel()
	entityType, ok := h.resolvePublishedClipNote(c, ctx, req.NoteID.Int64())
	if !ok {
		return
	}
	resp, err := h.clients.Reaction.ListPublicCollectionsForEntity(ctx, &reactionpb.ListPublicCollectionsForEntityRequest{
		Entity: &reactionpb.EntityRef{EntityType: entityType, EntityId: req.NoteID.Int64()}, Limit: 100, ViewerUserId: currentUserID(c),
	})
	if err != nil {
		writeRPCError(c, err)
		return
	}
	h.writePublicClipList(c, ctx, resp.GetItems())
}

func (h *Handler) writePublicClipList(c *gin.Context, ctx context.Context, items []*reactionpb.CollectionInfo) {
	out := make([]misskeyClip, 0, len(items))
	for _, item := range items {
		clip, ok := h.clipFromCollection(c, ctx, item, currentUserID(c), false)
		if !ok {
			return
		}
		out = append(out, clip)
	}
	c.JSON(stdhttp.StatusOK, out)
}

func (h *Handler) exportClipsUnavailable(c *gin.Context) {
	writeError(c, stdhttp.StatusNotImplemented, "clip export is not implemented", "not_implemented")
}

func (h *Handler) requirePublishedClipNote(c *gin.Context, ctx context.Context, id int64) bool {
	_, ok := h.resolvePublishedClipNote(c, ctx, id)
	return ok
}

func (h *Handler) resolvePublishedClipNote(c *gin.Context, ctx context.Context, id int64) (string, bool) {
	article, articleErr := h.clients.Content.GetArticle(ctx, &contentpb.GetArticleRequest{Key: &contentpb.GetArticleRequest_Id{Id: id}})
	topic, topicErr := h.clients.Content.GetTopic(ctx, &contentpb.GetTopicRequest{Key: &contentpb.GetTopicRequest_Id{Id: id}})
	articlePublished := articleErr == nil && article.GetArticle() != nil && article.GetArticle().GetStatus() == contentStatusPublished
	topicPublished := topicErr == nil && topic.GetTopic() != nil && topic.GetTopic().GetStatus() == contentStatusPublished
	if articlePublished && topicPublished { writeError(c, stdhttp.StatusConflict, "note id matches both an article and a topic", "ambiguous_note_id"); return "", false }
	if articlePublished { return "article", true }
	if topicPublished { return "topic", true }
	if articleErr != nil { writeRPCError(c, articleErr) } else if topicErr != nil { writeRPCError(c, topicErr) } else { writeError(c, stdhttp.StatusNotFound, "note not found", "not_found") }
	return "", false
}

func (h *Handler) clipFromCollection(c *gin.Context, ctx context.Context, item *reactionpb.CollectionInfo, viewerID int64, includeNotes bool) (misskeyClip, bool) {
	if item == nil {
		writeError(c, stdhttp.StatusNotFound, "clip not found", "not_found")
		return misskeyClip{}, false
	}
	userResp, err := h.clients.User.GetUser(ctx, &userpb.UserIDRequest{Id: item.GetUserId()})
	if err != nil {
		writeRPCError(c, err)
		return misskeyClip{}, false
	}
	user := userResp.GetUser()
	if user == nil {
		writeError(c, stdhttp.StatusBadGateway, "clip owner not found", "upstream_error")
		return misskeyClip{}, false
	}
	description := optionalMisskeyText(item.GetDescription())
	var lastClippedAt *string
	if item.GetLastClippedAt() > 0 {
		formatted := formatUnixMilli(item.GetLastClippedAt())
		lastClippedAt = &formatted
	}
	out := misskeyClip{ID: strconv.FormatInt(item.GetId(), 10), CreatedAt: formatUnixMilli(item.GetCreatedAt()), LastClippedAt: lastClippedAt, UserID: strconv.FormatInt(item.GetUserId(), 10), User: toMisskeyUserLite(user), Name: item.GetName(), Description: description, IsPublic: item.GetIsPublic()}
	if viewerID == item.GetUserId() {
		out.NotesCount = item.GetItemCount()
	}
	if counts, err := h.clients.Reaction.GetCounts(ctx, &reactionpb.EntityRequest{Entity: &reactionpb.EntityRef{EntityType: "collection", EntityId: item.GetId()}}); err == nil {
		out.FavoritedCount = counts.GetFavoriteCount()
	}
	if viewerID > 0 {
		isFavorited := false
		if favorites, err := h.clients.Reaction.ListFavorites(ctx, &reactionpb.ListFavoritesRequest{UserId: viewerID, EntityType: "collection", Limit: 100, Offset: 0}); err == nil {
			for _, favorite := range favorites.GetItems() {
				if favorite.GetEntity().GetEntityId() == item.GetId() {
					isFavorited = true
					break
				}
			}
		}
		out.IsFavorited = &isFavorited
	}
	return out, true
}

func (h *Handler) clipNoteFromItem(c *gin.Context, ctx context.Context, item *reactionpb.CollectionItemInfo) (misskeyClipNote, bool) {
	if item == nil || item.GetEntity() == nil {
		return misskeyClipNote{}, false
	}
	entityID := item.GetEntity().GetEntityId()
	var text, title string
	var author int64
	var created int64
	var tags []string
	if item.GetEntity().GetEntityType() == "topic" {
		resp, err := h.clients.Content.GetTopic(ctx, &contentpb.GetTopicRequest{Key: &contentpb.GetTopicRequest_Id{Id: entityID}})
		if err != nil || resp.GetTopic() == nil || resp.GetTopic().GetStatus() != contentStatusPublished {
			return misskeyClipNote{}, false
		}
		topic := resp.GetTopic()
		text, title, author, created, tags = topic.GetBody(), topic.GetTitle(), topic.GetAuthorId(), topic.GetCreatedAt(), topic.GetTags()
	} else {
		resp, err := h.clients.Content.GetArticle(ctx, &contentpb.GetArticleRequest{Key: &contentpb.GetArticleRequest_Id{Id: entityID}})
		if err != nil || resp.GetArticle() == nil || resp.GetArticle().GetStatus() != contentStatusPublished {
			return misskeyClipNote{}, false
		}
		article := resp.GetArticle()
		text, title, author, created, tags = article.GetBody(), article.GetTitle(), article.GetAuthorId(), article.GetCreatedAt(), article.GetTags()
	}
	userResp, err := h.clients.User.GetUser(ctx, &userpb.UserIDRequest{Id: author})
	if err != nil || userResp.GetUser() == nil {
		return misskeyClipNote{}, false
	}
	if strings.TrimSpace(text) == "" {
		text = title
	}
	id := strconv.FormatInt(entityID, 10)
	return misskeyClipNote{ID: id, ThreadID: id, CreatedAt: formatUnixMilli(created), Text: text, UserID: strconv.FormatInt(author, 10), User: toMisskeyUserLite(userResp.GetUser()), Visibility: "public", Mentions: []string{}, VisibleUserIDs: []string{}, FileIDs: []string{}, Files: []any{}, Tags: tags, Emojis: map[string]string{}, ReactionEmojis: map[string]string{}, Reactions: map[string]int64{}}, true
}
