package http

import (
	"context"
	stdhttp "net/http"
	"strconv"
	"strings"
	"unicode/utf8"

	"api-gateway/api/proto/contentpb"
	"api-gateway/api/proto/reactionpb"
	"api-gateway/api/proto/userpb"

	"github.com/gin-gonic/gin"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

const (
	compatInvalidParamID            = "3d81ceae-475f-4600-b2a8-2bc116157532"
	notesReactionCreateNoSuchNoteID = "033d0620-5bfe-4027-965d-980b0c85a3ea"
	notesReactionAlreadyReactedID   = "71efcf98-86d6-4e2b-b2ad-9d032369366b"
	notesReactionBlockedID          = "20ef5475-9f38-4e4c-bd33-de6d979498ec"
	notesReactionRenoteID           = "eaccdc08-ddef-43fe-908f-d108faad57f5"
	notesReactionDeleteNoSuchNoteID = "764d9fce-f9f2-4a0e-92b1-6ceac9a7ad37"
	notesReactionNotReactedID       = "92f4426d-4196-4125-aa5b-02943e2ec8fc"
	usersReactionsNotPublicID       = "673a7dd2-6924-1093-e0c0-e68456ceae5c"
	usersReactionsRemoteUserID      = "6b95fa98-8cf9-2350-e284-f0ffdb54a805"
	usersReactionsNoSuchUserID      = usersNotesNoSuchUserID
	usersReportNoSuchUserID         = "1acefcb5-0959-43fd-9685-b48305736cb5"
	usersReportSelfID               = "1e13149e-b1e8-43cf-902e-c01dbfcb202f"
	usersReportAdminID              = "35e166f5-05fb-4f87-a2d5-adb42676d48f"
)

type notesReactionCreateCompatRequest struct {
	NoteID   *jsonInt64 `json:"noteId"`
	Reaction string     `json:"reaction"`
}

type notesReactionDeleteCompatRequest struct {
	NoteID *jsonInt64 `json:"noteId"`
}

type usersReactionsCompatRequest struct {
	UserID    *jsonInt64 `json:"userId"`
	Limit     *int32     `json:"limit"`
	SinceID   *jsonInt64 `json:"sinceId"`
	UntilID   *jsonInt64 `json:"untilId"`
	SinceDate *int64     `json:"sinceDate"`
	UntilDate *int64     `json:"untilDate"`
}

type usersReportAbuseCompatRequest struct {
	UserID  *jsonInt64 `json:"userId"`
	Comment string     `json:"comment"`
}

type misskeyNoteReactionCompat struct {
	ID        string          `json:"id"`
	CreatedAt string          `json:"createdAt"`
	User      misskeyUserLite `json:"user"`
	Type      string          `json:"type"`
	Note      misskeyClipNote `json:"note"`
}

func (h *Handler) registerReactionsCompatRoutes(router *gin.Engine) {
	for _, prefix := range []string{"", "/api", "/api/v1"} {
		router.POST(prefix+"/notes/reactions/create", h.requireAuthScope("write"), h.createNoteReactionCompat)
		router.POST(prefix+"/notes/reactions/delete", h.requireAuthScope("write"), h.deleteNoteReactionCompat)
		router.POST(prefix+"/users/reactions", h.optionalAuth(), h.listUserReactionsCompat)
		router.POST(prefix+"/users/report-abuse", h.requireAuthScope("write"), h.reportUserAbuseCompat)
	}
}

func (h *Handler) createNoteReactionCompat(c *gin.Context) {
	var request notesReactionCreateCompatRequest
	if !decodeSensitiveAccountCompatRequest(c, &request) || request.NoteID == nil || request.NoteID.Int64() <= 0 || !validCompatReaction(request.Reaction) {
		writeFollowingCompatError(c, "Invalid param.", "INVALID_PARAM", compatInvalidParamID)
		return
	}
	ctx, cancel := rpcContext(c)
	defer cancel()
	ref, ok := h.resolveMisskeyReactionRef(c, ctx, request.NoteID.Int64(), notesReactionCreateNoSuchNoteID)
	if !ok {
		return
	}
	if h.clients == nil || h.clients.Reaction == nil {
		writeRPCError(c, status.Error(codes.Unavailable, "reaction service unavailable"))
		return
	}
	_, err := h.clients.Reaction.CreateReaction(ctx, &reactionpb.CreateReactionRequest{
		Entity: ref, UserId: currentUserID(c), Reaction: strings.TrimSpace(request.Reaction),
	})
	if err != nil {
		switch status.Code(err) {
		case codes.AlreadyExists:
			writeFollowingCompatError(c, "You are already reacting to that note.", "ALREADY_REACTED", notesReactionAlreadyReactedID)
		case codes.NotFound:
			writeFollowingCompatError(c, "No such note.", "NO_SUCH_NOTE", notesReactionCreateNoSuchNoteID)
		case codes.InvalidArgument:
			writeFollowingCompatError(c, "Invalid param.", "INVALID_PARAM", compatInvalidParamID)
		default:
			writeRPCError(c, err)
		}
		return
	}
	c.Status(stdhttp.StatusNoContent)
}

func (h *Handler) deleteNoteReactionCompat(c *gin.Context) {
	var request notesReactionDeleteCompatRequest
	if !decodeSensitiveAccountCompatRequest(c, &request) || request.NoteID == nil || request.NoteID.Int64() <= 0 {
		writeFollowingCompatError(c, "Invalid param.", "INVALID_PARAM", compatInvalidParamID)
		return
	}
	ctx, cancel := rpcContext(c)
	defer cancel()
	ref, ok := h.resolveMisskeyReactionRef(c, ctx, request.NoteID.Int64(), notesReactionDeleteNoSuchNoteID)
	if !ok {
		return
	}
	if h.clients == nil || h.clients.Reaction == nil {
		writeRPCError(c, status.Error(codes.Unavailable, "reaction service unavailable"))
		return
	}
	_, err := h.clients.Reaction.DeleteReaction(ctx, &reactionpb.DeleteReactionRequest{Entity: ref, UserId: currentUserID(c)})
	if err != nil {
		switch status.Code(err) {
		case codes.NotFound:
			writeFollowingCompatError(c, "You are not reacting to that note.", "NOT_REACTED", notesReactionNotReactedID)
		case codes.InvalidArgument:
			writeFollowingCompatError(c, "Invalid param.", "INVALID_PARAM", compatInvalidParamID)
		default:
			writeRPCError(c, err)
		}
		return
	}
	c.Status(stdhttp.StatusNoContent)
}

func validCompatReaction(value string) bool {
	value = strings.TrimSpace(value)
	return value != "" && utf8.ValidString(value) && utf8.RuneCountInString(value) <= 128
}

func (h *Handler) resolveMisskeyReactionRef(c *gin.Context, ctx context.Context, id int64, noSuchID string) (*reactionpb.EntityRef, bool) {
	if h.clients == nil || h.clients.Content == nil {
		writeRPCError(c, status.Error(codes.Unavailable, "content service unavailable"))
		return nil, false
	}
	articleResponse, articleErr := h.clients.Content.GetArticle(ctx, &contentpb.GetArticleRequest{Key: &contentpb.GetArticleRequest_Id{Id: id}})
	topicResponse, topicErr := h.clients.Content.GetTopic(ctx, &contentpb.GetTopicRequest{Key: &contentpb.GetTopicRequest_Id{Id: id}, ViewerUserId: currentUserID(c)})
	article := articleResponse.GetArticle()
	topic := topicResponse.GetTopic()
	articlePublished := articleErr == nil && article != nil && article.GetStatus() == contentStatusPublished
	topicPublished := topicErr == nil && topic != nil && topic.GetStatus() == contentStatusPublished
	if articlePublished && topicPublished {
		writeError(c, stdhttp.StatusConflict, "note id matches both an article and a topic", "ambiguous_note_id")
		return nil, false
	}
	if articlePublished {
		return &reactionpb.EntityRef{EntityType: "article", EntityId: id}, true
	}
	if topicPublished {
		return &reactionpb.EntityRef{EntityType: "topic", EntityId: id}, true
	}
	if articleErr != nil && status.Code(articleErr) != codes.NotFound {
		writeRPCError(c, articleErr)
		return nil, false
	}
	if topicErr != nil && status.Code(topicErr) != codes.NotFound {
		writeRPCError(c, topicErr)
		return nil, false
	}
	writeFollowingCompatError(c, "No such note.", "NO_SUCH_NOTE", noSuchID)
	return nil, false
}

func (h *Handler) listUserReactionsCompat(c *gin.Context) {
	var request usersReactionsCompatRequest
	if !decodeSensitiveAccountCompatRequest(c, &request) || request.UserID == nil || request.UserID.Int64() <= 0 {
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
	if h.clients == nil || h.clients.User == nil || h.clients.Reaction == nil {
		writeRPCError(c, status.Error(codes.Unavailable, "user or reaction service unavailable"))
		return
	}
	userResponse, err := h.clients.User.GetUser(ctx, &userpb.UserIDRequest{Id: request.UserID.Int64()})
	if err != nil {
		if status.Code(err) == codes.NotFound {
			writeFollowingCompatError(c, "No such user.", "NO_SUCH_USER", usersReactionsNoSuchUserID)
			return
		}
		writeRPCError(c, err)
		return
	}
	user := userResponse.GetUser()
	if user == nil {
		writeFollowingCompatError(c, "No such user.", "NO_SUCH_USER", usersReactionsNoSuchUserID)
		return
	}
	h.sanitizeUserProfileTheme(ctx, user)
	response, err := h.clients.Reaction.ListReactions(ctx, &reactionpb.ListReactionsRequest{
		UserId: request.UserID.Int64(), Limit: limit,
		SinceId: compatCursorValue(request.SinceID), UntilId: compatCursorValue(request.UntilID),
		SinceDate: compatDateValue(request.SinceDate), UntilDate: compatDateValue(request.UntilDate),
	})
	if err != nil {
		writeRPCError(c, err)
		return
	}
	items := make([]misskeyNoteReactionCompat, 0, len(response.GetItems()))
	packedUser := toMisskeyUserLite(user)
	for _, reaction := range response.GetItems() {
		if reaction == nil || (reaction.GetEntity().GetEntityType() != "topic" && reaction.GetEntity().GetEntityType() != "article") {
			continue
		}
		note, ok := h.packCompatReactionNote(c, ctx, reaction.GetEntity())
		if !ok {
			if c.IsAborted() {
				return
			}
			continue
		}
		items = append(items, misskeyNoteReactionCompat{
			ID: strconv.FormatInt(reaction.GetId(), 10), CreatedAt: formatUnixMilli(reaction.GetCreatedAt()),
			User: packedUser, Type: reaction.GetReaction(), Note: note,
		})
	}
	c.JSON(stdhttp.StatusOK, items)
}

func compatCursorValue(value *jsonInt64) int64 {
	if value == nil {
		return 0
	}
	return value.Int64()
}

func compatDateValue(value *int64) int64 {
	if value == nil {
		return 0
	}
	return *value
}

func (h *Handler) packCompatReactionNote(c *gin.Context, ctx context.Context, ref *reactionpb.EntityRef) (misskeyClipNote, bool) {
	if ref == nil || h.clients == nil || h.clients.Content == nil {
		return misskeyClipNote{}, false
	}
	switch ref.GetEntityType() {
	case "topic":
		response, err := h.clients.Content.GetTopic(ctx, &contentpb.GetTopicRequest{Key: &contentpb.GetTopicRequest_Id{Id: ref.GetEntityId()}, ViewerUserId: currentUserID(c)})
		if err != nil || response.GetTopic() == nil || response.GetTopic().GetStatus() != contentStatusPublished {
			return misskeyClipNote{}, false
		}
		return h.misskeyNoteFromTopic(c, ctx, response.GetTopic())
	case "article":
		response, err := h.clients.Content.GetArticle(ctx, &contentpb.GetArticleRequest{Key: &contentpb.GetArticleRequest_Id{Id: ref.GetEntityId()}})
		if err != nil || response.GetArticle() == nil || response.GetArticle().GetStatus() != contentStatusPublished {
			return misskeyClipNote{}, false
		}
		return h.misskeyNoteFromArticle(c, ctx, response.GetArticle())
	default:
		return misskeyClipNote{}, false
	}
}

func (h *Handler) reportUserAbuseCompat(c *gin.Context) {
	var request usersReportAbuseCompatRequest
	if !decodeSensitiveAccountCompatRequest(c, &request) || request.UserID == nil || request.UserID.Int64() <= 0 || strings.TrimSpace(request.Comment) == "" || utf8.RuneCountInString(request.Comment) > 2048 {
		writeFollowingCompatError(c, "Invalid param.", "INVALID_PARAM", compatInvalidParamID)
		return
	}
	ctx, cancel := rpcContext(c)
	defer cancel()
	if h.clients == nil || h.clients.User == nil || h.clients.Reaction == nil {
		writeRPCError(c, status.Error(codes.Unavailable, "user or reaction service unavailable"))
		return
	}
	targetResponse, err := h.clients.User.GetUser(ctx, &userpb.UserIDRequest{Id: request.UserID.Int64()})
	if err != nil {
		if status.Code(err) == codes.NotFound {
			writeFollowingCompatError(c, "No such user.", "NO_SUCH_USER", usersReportNoSuchUserID)
			return
		}
		writeRPCError(c, err)
		return
	}
	target := targetResponse.GetUser()
	if target == nil {
		writeFollowingCompatError(c, "No such user.", "NO_SUCH_USER", usersReportNoSuchUserID)
		return
	}
	if target.GetId() == currentUserID(c) {
		writeFollowingCompatError(c, "Cannot report yourself.", "CANNOT_REPORT_YOURSELF", usersReportSelfID)
		return
	}
	if isCompatAdministrator(target) {
		writeFollowingCompatError(c, "Cannot report the admin.", "CANNOT_REPORT_THE_ADMIN", usersReportAdminID)
		return
	}
	_, err = h.clients.Reaction.SubmitReport(ctx, &reactionpb.SubmitReportRequest{
		Entity: &reactionpb.EntityRef{EntityType: "user", EntityId: target.GetId()}, ReporterId: currentUserID(c),
		Reason: "abuse", Description: strings.TrimSpace(request.Comment),
	})
	if err != nil {
		writeRPCError(c, err)
		return
	}
	c.Status(stdhttp.StatusNoContent)
}

func isCompatAdministrator(user *userpb.UserInfo) bool {
	if user == nil {
		return false
	}
	switch strings.ToLower(strings.TrimSpace(user.GetAccountState())) {
	case "admin", "administrator":
		return true
	}
	switch strings.ToLower(strings.TrimSpace(user.GetUsername())) {
	case "admin", "administrator":
		return true
	default:
		return false
	}
}
