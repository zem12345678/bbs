package http

import (
	"context"
	stdhttp "net/http"
	"strconv"
	"strings"

	"api-gateway/api/proto/commentpb"
	"api-gateway/api/proto/userpb"
	"api-gateway/pkg/http/response"

	"github.com/gin-gonic/gin"
)

const (
	commentConversationDefaultLimit int32 = 10
	commentConversationMaxLimit     int32 = 100
	commentConversationMaxOffset    int32 = 10000
)

type notesConversationRequest struct {
	NoteID jsonInt64 `json:"noteId"`
	Limit  *int32    `json:"limit"`
	Offset *int32    `json:"offset"`
}

type misskeyConversationNote struct {
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

func (h *Handler) getCommentConversation(c *gin.Context) {
	commentID, ok := pathInt64(c, "id")
	if !ok {
		return
	}
	limit, offset, ok := commentConversationQuery(c)
	if !ok {
		return
	}
	ctx, cancel := rpcContext(c)
	defer cancel()
	result, ok := h.loadCommentConversation(c, ctx, commentID, limit, offset)
	if !ok {
		return
	}
	response.Success(c, result)
}

func (h *Handler) notesConversationCompat(c *gin.Context) {
	var req notesConversationRequest
	if !bindJSON(c, &req) {
		return
	}
	commentID := req.NoteID.Int64()
	if commentID <= 0 {
		writeError(c, stdhttp.StatusBadRequest, "noteId must be a positive integer", "invalid_argument")
		return
	}
	limit := commentConversationDefaultLimit
	if req.Limit != nil {
		limit = *req.Limit
	}
	offset := int32(0)
	if req.Offset != nil {
		offset = *req.Offset
	}
	if !validCommentConversationPage(c, limit, offset) {
		return
	}
	ctx, cancel := rpcContext(c)
	defer cancel()
	result, ok := h.loadCommentConversation(c, ctx, commentID, limit, offset)
	if !ok {
		return
	}
	notes, ok := h.toMisskeyConversationNotes(c, ctx, result.GetItems())
	if !ok {
		return
	}
	c.JSON(stdhttp.StatusOK, notes)
}

func (h *Handler) loadCommentConversation(c *gin.Context, ctx context.Context, commentID int64, limit, offset int32) (*commentpb.CommentListResponse, bool) {
	target, ok := h.requireVisibleComment(c, ctx, commentID)
	if !ok {
		return nil, false
	}
	if !h.requirePublishedContentTarget(c, ctx, target.GetEntityType(), target.GetEntityId()) {
		return nil, false
	}
	result, err := h.clients.Comment.GetCommentConversation(ctx, &commentpb.GetCommentConversationRequest{
		CommentId: commentID,
		Limit:     limit,
		Offset:    offset,
	})
	if err != nil {
		writeRPCError(c, err)
		return nil, false
	}
	return result, true
}

func commentConversationQuery(c *gin.Context) (int32, int32, bool) {
	limit, ok := strictQueryInt32(c, "limit", commentConversationDefaultLimit)
	if !ok {
		return 0, 0, false
	}
	offset, ok := strictQueryInt32(c, "offset", 0)
	if !ok || !validCommentConversationPage(c, limit, offset) {
		return 0, 0, false
	}
	return limit, offset, true
}

func strictQueryInt32(c *gin.Context, name string, fallback int32) (int32, bool) {
	raw := strings.TrimSpace(c.Query(name))
	if raw == "" {
		return fallback, true
	}
	value, err := strconv.ParseInt(raw, 10, 32)
	if err != nil {
		writeError(c, stdhttp.StatusBadRequest, name+" must be an integer", "invalid_argument")
		return 0, false
	}
	return int32(value), true
}

func validCommentConversationPage(c *gin.Context, limit, offset int32) bool {
	if limit < 1 || limit > commentConversationMaxLimit {
		writeError(c, stdhttp.StatusBadRequest, "limit must be between 1 and 100", "invalid_argument")
		return false
	}
	if offset < 0 || offset > commentConversationMaxOffset {
		writeError(c, stdhttp.StatusBadRequest, "offset must be between 0 and 10000", "invalid_argument")
		return false
	}
	return true
}

func (h *Handler) toMisskeyConversationNotes(c *gin.Context, ctx context.Context, comments []*commentpb.CommentInfo) ([]misskeyConversationNote, bool) {
	authorIDs := make([]int64, 0, len(comments))
	seen := make(map[int64]struct{}, len(comments))
	for _, comment := range comments {
		if comment == nil || comment.GetAuthorId() <= 0 {
			continue
		}
		if _, exists := seen[comment.GetAuthorId()]; exists {
			continue
		}
		seen[comment.GetAuthorId()] = struct{}{}
		authorIDs = append(authorIDs, comment.GetAuthorId())
	}
	usersByID := make(map[int64]*userpb.UserInfo, len(authorIDs))
	if len(authorIDs) > 0 {
		users, err := h.clients.User.ListUsers(ctx, &userpb.ListUsersRequest{Ids: authorIDs, Page: 1, PageSize: int32(len(authorIDs))})
		if err != nil {
			writeRPCError(c, err)
			return nil, false
		}
		for _, user := range users.GetItems() {
			if user != nil {
				usersByID[user.GetId()] = user
			}
		}
	}
	notes := make([]misskeyConversationNote, 0, len(comments))
	for _, comment := range comments {
		if comment == nil {
			continue
		}
		user := usersByID[comment.GetAuthorId()]
		if user == nil {
			writeError(c, stdhttp.StatusBadGateway, "comment author not found", "upstream_error")
			return nil, false
		}
		notes = append(notes, misskeyConversationNoteFromProto(comment, user))
	}
	return notes, true
}

func misskeyConversationNoteFromProto(comment *commentpb.CommentInfo, user *userpb.UserInfo) misskeyConversationNote {
	id := strconv.FormatInt(comment.GetId(), 10)
	threadID := comment.GetRootId()
	if threadID <= 0 {
		threadID = comment.GetId()
	}
	var replyID *string
	if comment.GetParentId() > 0 {
		value := strconv.FormatInt(comment.GetParentId(), 10)
		replyID = &value
	}
	return misskeyConversationNote{
		ID: id, ThreadID: strconv.FormatInt(threadID, 10), CreatedAt: formatUnixMilli(comment.GetCreatedAt()), Text: comment.GetContent(),
		UserID: strconv.FormatInt(comment.GetAuthorId(), 10), User: toMisskeyUserLite(user), ReplyID: replyID, Visibility: "public",
		Mentions: []string{}, VisibleUserIDs: []string{}, FileIDs: []string{}, Files: []any{}, Tags: []string{}, Emojis: map[string]string{},
		ReactionEmojis: map[string]string{}, Reactions: map[string]int64{}, ReactionCount: comment.GetLikeCount(), RepliesCount: comment.GetReplyCount(),
	}
}
