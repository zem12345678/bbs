package http

import (
	"context"
	stdhttp "net/http"
	"sort"
	"strconv"

	"api-gateway/api/proto/commentpb"
	"api-gateway/api/proto/userpb"

	"github.com/gin-gonic/gin"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

const (
	notesChildrenNoSuchNoteID = "24fcbfc6-2e37-42b6-8388-c29b3861a08d"
	notesRepliesInvalidID     = compatInvalidParamID
)

type notesReadTimelineCompatRequest struct {
	Limit            *int32     `json:"limit"`
	Offset           *int32     `json:"offset"`
	SinceID          *jsonInt64 `json:"sinceId"`
	UntilID          *jsonInt64 `json:"untilId"`
	SinceDate        *int64     `json:"sinceDate"`
	UntilDate        *int64     `json:"untilDate"`
	WithFiles        bool       `json:"withFiles"`
	WithReplies      bool       `json:"withReplies"`
	AllowPartial     bool       `json:"allowPartial"`
	TimelineMode     string     `json:"timelineMode"`
	List             string     `json:"list"`
	IncludeNonPublic bool       `json:"includeNonPublic"`
}

type notesReadFeaturedCompatRequest struct {
	Limit     *int32     `json:"limit"`
	UntilID   *jsonInt64 `json:"untilId"`
	ChannelID *jsonInt64 `json:"channelId"`
}

type usersFeaturedNotesCompatRequest struct {
	UserID  *jsonInt64 `json:"userId"`
	Limit   *int32     `json:"limit"`
	UntilID *jsonInt64 `json:"untilId"`
}

type notesChildrenCompatRequest struct {
	NoteID     *jsonInt64 `json:"noteId"`
	Limit      *int32     `json:"limit"`
	SinceID    *jsonInt64 `json:"sinceId"`
	UntilID    *jsonInt64 `json:"untilId"`
	ShowQuotes *bool      `json:"showQuotes"`
}

func (h *Handler) registerNotesReadCompatRoutes(router *gin.Engine) {
	for _, prefix := range []string{"", "/api", "/api/v1"} {
		router.POST(prefix+"/notes/global-timeline", h.optionalAuth(), h.notesGlobalTimelineCompat)
		router.POST(prefix+"/notes/local-timeline", h.optionalAuth(), h.notesLocalTimelineCompat)
		router.POST(prefix+"/notes/following", h.requireAuthScope("read"), h.notesFollowingCompat)
		router.POST(prefix+"/notes/featured", h.optionalAuth(), h.notesFeaturedCompat)
		router.POST(prefix+"/users/featured-notes", h.optionalAuth(), h.usersFeaturedNotesCompat)
		router.POST(prefix+"/notes/children", h.optionalAuth(), h.notesChildrenCompat)
		router.POST(prefix+"/notes/replies", h.optionalAuth(), h.notesRepliesCompat)
	}
}

func (h *Handler) notesGlobalTimelineCompat(c *gin.Context) {
	var request notesReadTimelineCompatRequest
	if !decodeSensitiveAccountCompatRequest(c, &request) || request.WithFiles || request.AllowPartial {
		writeFollowingCompatError(c, "Invalid param.", "INVALID_PARAM", compatInvalidParamID)
		return
	}
	h.writeCompatTimeline(c, request, "")
}

func (h *Handler) notesLocalTimelineCompat(c *gin.Context) {
	var request notesReadTimelineCompatRequest
	if !decodeSensitiveAccountCompatRequest(c, &request) || request.WithFiles || request.WithReplies || request.AllowPartial {
		writeFollowingCompatError(c, "Invalid param.", "INVALID_PARAM", compatInvalidParamID)
		return
	}
	if request.TimelineMode != "" && request.TimelineMode != "chronological" {
		writeFollowingCompatError(c, "Invalid param.", "INVALID_PARAM", compatInvalidParamID)
		return
	}
	h.writeCompatTimeline(c, request, "")
}

func (h *Handler) writeCompatTimeline(c *gin.Context, request notesReadTimelineCompatRequest, sortOrder string) {
	limit, ok := normalizeCompatNoteLimit(request.Limit)
	if !ok || !validCompatNoteWindow(request.SinceID, request.UntilID, request.SinceDate, request.UntilDate) {
		writeFollowingCompatError(c, "Invalid param.", "INVALID_PARAM", compatInvalidParamID)
		return
	}
	offset := int32(0)
	if request.Offset != nil {
		offset = *request.Offset
	}
	if offset < 0 || offset > 1000 {
		writeFollowingCompatError(c, "Invalid param.", "INVALID_PARAM", compatInvalidParamID)
		return
	}
	ctx, cancel := rpcContext(c)
	defer cancel()
	items, err := h.listCompatNotesWithOptions(c, ctx, limit, 0, 0, true, request.SinceID, request.UntilID, request.SinceDate, request.UntilDate, sortOrder, offset)
	if err != nil {
		writeRPCError(c, err)
		return
	}
	c.JSON(stdhttp.StatusOK, items)
}

func (h *Handler) notesFollowingCompat(c *gin.Context) {
	var request notesReadTimelineCompatRequest
	if !decodeSensitiveAccountCompatRequest(c, &request) || request.WithFiles || request.WithReplies || request.IncludeNonPublic || request.List == "followers" || request.List == "mutuals" {
		writeFollowingCompatError(c, "Invalid param.", "INVALID_PARAM", compatInvalidParamID)
		return
	}
	limit, ok := normalizeCompatNoteLimit(request.Limit)
	if !ok || !validCompatNoteWindow(request.SinceID, request.UntilID, request.SinceDate, request.UntilDate) {
		writeFollowingCompatError(c, "Invalid param.", "INVALID_PARAM", compatInvalidParamID)
		return
	}
	ctx, cancel := rpcContext(c)
	defer cancel()
	authorIDs, err := h.followingIDSet(ctx, currentUserID(c))
	if err != nil {
		writeRPCError(c, err)
		return
	}
	items := make([]misskeyClipNote, 0, limit)
	for authorID := range authorIDs {
		page, err := h.listCompatNotes(c, ctx, limit, authorID, 0, true, request.SinceID, request.UntilID, request.SinceDate, request.UntilDate)
		if err != nil {
			writeRPCError(c, err)
			return
		}
		items = append(items, page...)
	}
	sort.SliceStable(items, func(i, j int) bool { return items[i].CreatedAt > items[j].CreatedAt })
	if len(items) > int(limit) {
		items = items[:limit]
	}
	c.JSON(stdhttp.StatusOK, items)
}

func (h *Handler) notesFeaturedCompat(c *gin.Context) {
	var request notesReadFeaturedCompatRequest
	if !decodeSensitiveAccountCompatRequest(c, &request) || (request.ChannelID != nil && request.ChannelID.Int64() <= 0) {
		writeFollowingCompatError(c, "Invalid param.", "INVALID_PARAM", compatInvalidParamID)
		return
	}
	limit, ok := normalizeCompatNoteLimit(request.Limit)
	if !ok || !validCompatNoteWindow(nil, request.UntilID, nil, nil) {
		writeFollowingCompatError(c, "Invalid param.", "INVALID_PARAM", compatInvalidParamID)
		return
	}
	ctx, cancel := rpcContext(c)
	defer cancel()
	channelID := int64(0)
	if request.ChannelID != nil {
		channelID = request.ChannelID.Int64()
	}
	items, err := h.listCompatNotesWithOptions(c, ctx, limit, 0, channelID, true, nil, request.UntilID, nil, nil, "hot", 0)
	if err != nil {
		writeRPCError(c, err)
		return
	}
	c.JSON(stdhttp.StatusOK, items)
}

func (h *Handler) usersFeaturedNotesCompat(c *gin.Context) {
	var request usersFeaturedNotesCompatRequest
	if !decodeSensitiveAccountCompatRequest(c, &request) || request.UserID == nil || request.UserID.Int64() <= 0 {
		writeFollowingCompatError(c, "Invalid param.", "INVALID_PARAM", compatInvalidParamID)
		return
	}
	limit, ok := normalizeCompatNoteLimit(request.Limit)
	if !ok || !validCompatNoteWindow(nil, request.UntilID, nil, nil) {
		writeFollowingCompatError(c, "Invalid param.", "INVALID_PARAM", compatInvalidParamID)
		return
	}
	ctx, cancel := rpcContext(c)
	defer cancel()
	if h.clients == nil || h.clients.User == nil {
		writeRPCError(c, status.Error(codes.Unavailable, "user service unavailable"))
		return
	}
	userResponse, err := h.clients.User.GetUser(ctx, &userpb.UserIDRequest{Id: request.UserID.Int64()})
	if err != nil {
		writeUsersNotesRPCError(c, err)
		return
	}
	if userResponse.GetUser() == nil {
		writeFollowingCompatError(c, "No such user.", "NO_SUCH_USER", usersNotesNoSuchUserID)
		return
	}
	items, err := h.listCompatNotesWithOptions(c, ctx, limit, request.UserID.Int64(), 0, true, nil, request.UntilID, nil, nil, "hot", 0)
	if err != nil {
		writeRPCError(c, err)
		return
	}
	c.JSON(stdhttp.StatusOK, items)
}

func (h *Handler) notesChildrenCompat(c *gin.Context) {
	h.listNoteRepliesCompat(c, false)
}

func (h *Handler) notesRepliesCompat(c *gin.Context) {
	h.listNoteRepliesCompat(c, true)
}

func (h *Handler) listNoteRepliesCompat(c *gin.Context, replies bool) {
	var request notesChildrenCompatRequest
	if !decodeSensitiveAccountCompatRequest(c, &request) || request.NoteID == nil || request.NoteID.Int64() <= 0 {
		writeFollowingCompatError(c, "Invalid param.", "INVALID_PARAM", notesRepliesInvalidID)
		return
	}
	limit, ok := normalizeCompatNoteLimit(request.Limit)
	if !ok || !validCompatNoteWindow(request.SinceID, request.UntilID, nil, nil) {
		writeFollowingCompatError(c, "Invalid param.", "INVALID_PARAM", compatInvalidParamID)
		return
	}
	ctx, cancel := rpcContext(c)
	defer cancel()
	ref, ok := h.resolveMisskeyReactionRef(c, ctx, request.NoteID.Int64(), notesChildrenNoSuchNoteID)
	if !ok {
		return
	}
	if h.clients == nil || h.clients.Comment == nil || h.clients.User == nil {
		writeRPCError(c, status.Error(codes.Unavailable, "comment or user service unavailable"))
		return
	}
	comments, err := h.clients.Comment.ListComments(ctx, &commentpb.ListCommentsRequest{EntityType: ref.GetEntityType(), EntityId: ref.GetEntityId(), Page: 1, PageSize: limit})
	if err != nil {
		writeRPCError(c, err)
		return
	}
	items, err := h.commentNotesCompat(c, ctx, comments.GetItems(), request.SinceID, request.UntilID)
	if err != nil {
		writeRPCError(c, err)
		return
	}
	if replies {
		// The current domain stores note replies as top-level comments. Nested
		// comment replies remain available through /comments/:id/replies.
	}
	c.JSON(stdhttp.StatusOK, items)
}

func (h *Handler) commentNotesCompat(c *gin.Context, ctx context.Context, comments []*commentpb.CommentInfo, sinceID, untilID *jsonInt64) ([]misskeyClipNote, error) {
	userIDs := make([]int64, 0, len(comments))
	seen := make(map[int64]struct{}, len(comments))
	for _, comment := range comments {
		if comment == nil || comment.GetAuthorId() <= 0 {
			continue
		}
		if _, exists := seen[comment.GetAuthorId()]; !exists {
			seen[comment.GetAuthorId()] = struct{}{}
			userIDs = append(userIDs, comment.GetAuthorId())
		}
	}
	users, err := h.clients.User.ListUsers(ctx, &userpb.ListUsersRequest{Ids: userIDs, Page: 1, PageSize: int32(len(userIDs))})
	if err != nil {
		return nil, err
	}
	usersByID := make(map[int64]*userpb.UserInfo, len(users.GetItems()))
	for _, user := range users.GetItems() {
		if user != nil {
			h.sanitizeUserProfileTheme(ctx, user)
			usersByID[user.GetId()] = user
		}
	}
	items := make([]misskeyClipNote, 0, len(comments))
	for _, comment := range comments {
		if comment == nil || !compatNoteInWindow(comment.GetId(), comment.GetCreatedAt(), sinceID, untilID, nil, nil) {
			continue
		}
		user := usersByID[comment.GetAuthorId()]
		if user == nil {
			continue
		}
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
		items = append(items, misskeyClipNote{ID: id, ThreadID: strconv.FormatInt(threadID, 10), CreatedAt: formatUnixMilli(comment.GetCreatedAt()), Text: comment.GetContent(), UserID: strconv.FormatInt(comment.GetAuthorId(), 10), User: toMisskeyUserLite(user), ReplyID: replyID, Visibility: "public", Mentions: []string{}, VisibleUserIDs: []string{}, FileIDs: []string{}, Files: []any{}, Tags: []string{}, Emojis: map[string]string{}, ReactionEmojis: map[string]string{}, Reactions: map[string]int64{}, RepliesCount: comment.GetReplyCount(), ReactionCount: comment.GetLikeCount()})
		if int32(len(items)) >= 100 {
			break
		}
	}
	return items, nil
}
