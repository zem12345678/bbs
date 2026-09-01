package http

import (
	"context"
	stdhttp "net/http"
	"sort"
	"strconv"
	"strings"
	"time"

	"api-gateway/api/proto/contentpb"
	"api-gateway/api/proto/userpb"

	"github.com/gin-gonic/gin"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

const notesShowNoSuchNoteID = "24fcbfc6-2e37-42b6-8388-c29b3861a08d"
const notesDeleteNoSuchNoteID = "490be23f-8c1f-4796-819f-94cb4f9d1630"
const notesDeleteAccessDeniedID = "fe8d7103-0ea8-4ec3-814d-f8b401dc69e9"
const usersNotesNoSuchUserID = "27e494ba-2ac2-48e8-893b-10d4d8c2387b"

type notesCreateCompatRequest struct {
	Visibility         string                 `json:"visibility"`
	VisibleUserIDs     []jsonInt64            `json:"visibleUserIds"`
	CW                 *string                `json:"cw"`
	LocalOnly          bool                   `json:"localOnly"`
	ReactionAcceptance *string                `json:"reactionAcceptance"`
	NoExtractMentions  bool                   `json:"noExtractMentions"`
	NoExtractHashtags  bool                   `json:"noExtractHashtags"`
	NoExtractEmojis    bool                   `json:"noExtractEmojis"`
	ReplyID            *jsonInt64             `json:"replyId"`
	RenoteID           *jsonInt64             `json:"renoteId"`
	ChannelID          *jsonInt64             `json:"channelId"`
	Text               *string                `json:"text"`
	FileIDs            []jsonInt64            `json:"fileIds"`
	MediaIDs           []jsonInt64            `json:"mediaIds"`
	Poll               *notesCreateCompatPoll `json:"poll"`
}

type notesCreateCompatPoll struct {
	Choices      []string `json:"choices"`
	Multiple     bool     `json:"multiple"`
	ExpiresAt    *int64   `json:"expiresAt"`
	ExpiredAfter *int64   `json:"expiredAfter"`
}

type notesTimelineCompatRequest struct {
	Limit                   *int32     `json:"limit"`
	SinceID                 *jsonInt64 `json:"sinceId"`
	UntilID                 *jsonInt64 `json:"untilId"`
	SinceDate               *int64     `json:"sinceDate"`
	UntilDate               *int64     `json:"untilDate"`
	AllowPartial            bool       `json:"allowPartial"`
	WithFiles               bool       `json:"withFiles"`
	WithRenotes             *bool      `json:"withRenotes"`
	WithBots                *bool      `json:"withBots"`
	IncludeFollowedChannels *bool      `json:"includeFollowedChannels"`
}

type usersNotesCompatRequest struct {
	UserID           *jsonInt64 `json:"userId"`
	WithReplies      bool       `json:"withReplies"`
	WithQuotes       *bool      `json:"withQuotes"`
	WithRenotes      *bool      `json:"withRenotes"`
	WithBots         *bool      `json:"withBots"`
	WithNonPublic    *bool      `json:"withNonPublic"`
	WithChannelNotes bool       `json:"withChannelNotes"`
	ChannelID        *jsonInt64 `json:"channelId"`
	Limit            *int32     `json:"limit"`
	SinceID          *jsonInt64 `json:"sinceId"`
	UntilID          *jsonInt64 `json:"untilId"`
	SinceDate        *int64     `json:"sinceDate"`
	UntilDate        *int64     `json:"untilDate"`
	AllowPartial     bool       `json:"allowPartial"`
	WithFiles        bool       `json:"withFiles"`
}

type notesShowCompatRequest struct {
	NoteID *jsonInt64 `json:"noteId"`
}

func (h *Handler) registerNotesCompatRoutes(router *gin.Engine) {
	for _, prefix := range []string{"", "/api", "/api/v1"} {
		router.POST(prefix+"/notes/create", h.requireAuthScope("write"), h.createNoteCompat)
		router.POST(prefix+"/notes/delete", h.requireAuthScope("write"), h.deleteNoteCompat)
		router.POST(prefix+"/notes/timeline", h.requireAuthScope("read"), h.notesTimelineCompat)
		router.POST(prefix+"/notes/show", h.optionalAuth(), h.showNoteCompat)
		router.POST(prefix+"/users/notes", h.optionalAuth(), h.usersNotesCompat)
	}
}

func (h *Handler) notesTimelineCompat(c *gin.Context) {
	var request notesTimelineCompatRequest
	if !decodeSensitiveAccountCompatRequest(c, &request) {
		writeFollowingCompatError(c, "Invalid param.", "INVALID_PARAM", "")
		return
	}
	limit, ok := normalizeCompatNoteLimit(request.Limit)
	if !ok || !validCompatNoteWindow(request.SinceID, request.UntilID, request.SinceDate, request.UntilDate) || request.AllowPartial || request.WithFiles {
		writeFollowingCompatError(c, "Invalid param.", "INVALID_PARAM", "")
		return
	}
	includeChannels := request.IncludeFollowedChannels == nil || *request.IncludeFollowedChannels
	ctx, cancel := rpcContext(c)
	defer cancel()
	items, err := h.listCompatNotes(c, ctx, limit, 0, 0, includeChannels, request.SinceID, request.UntilID, request.SinceDate, request.UntilDate)
	if err != nil {
		writeRPCError(c, err)
		return
	}
	if c.IsAborted() {
		return
	}
	c.JSON(stdhttp.StatusOK, items)
}

func (h *Handler) usersNotesCompat(c *gin.Context) {
	var request usersNotesCompatRequest
	if !decodeSensitiveAccountCompatRequest(c, &request) || request.UserID == nil || request.UserID.Int64() <= 0 {
		writeFollowingCompatError(c, "Invalid param.", "INVALID_PARAM", "")
		return
	}
	if request.WithReplies && request.WithFiles {
		writeFollowingCompatError(c, "Specifying both withReplies and withFiles is not supported", "BOTH_WITH_REPLIES_AND_WITH_FILES", "91c8cb9f-36ed-46e7-9ca2-7df96ed6e222")
		return
	}
	limit, ok := normalizeCompatNoteLimit(request.Limit)
	if !ok || !validCompatNoteWindow(request.SinceID, request.UntilID, request.SinceDate, request.UntilDate) || request.AllowPartial || request.WithFiles || request.WithReplies {
		writeFollowingCompatError(c, "Invalid param.", "INVALID_PARAM", "")
		return
	}
	if request.ChannelID != nil && request.ChannelID.Int64() <= 0 {
		writeFollowingCompatError(c, "Invalid param.", "INVALID_PARAM", "")
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
	includeChannels := request.WithChannelNotes
	channelID := int64(0)
	if request.ChannelID != nil {
		channelID = request.ChannelID.Int64()
		includeChannels = true
	}
	items, err := h.listCompatNotes(c, ctx, limit, request.UserID.Int64(), channelID, includeChannels, request.SinceID, request.UntilID, request.SinceDate, request.UntilDate)
	if err != nil {
		writeRPCError(c, err)
		return
	}
	if c.IsAborted() {
		return
	}
	c.JSON(stdhttp.StatusOK, items)
}

func normalizeCompatNoteLimit(value *int32) (int32, bool) {
	limit := int32(10)
	if value != nil {
		limit = *value
	}
	return limit, limit >= 1 && limit <= 100
}

func validCompatNoteWindow(sinceID, untilID *jsonInt64, sinceDate, untilDate *int64) bool {
	if (sinceID != nil && sinceID.Int64() <= 0) || (untilID != nil && untilID.Int64() <= 0) {
		return false
	}
	if (sinceDate != nil && *sinceDate <= 0) || (untilDate != nil && *untilDate <= 0) {
		return false
	}
	return true
}

func (h *Handler) listCompatNotes(c *gin.Context, ctx context.Context, limit int32, authorID, channelID int64, includeChannels bool, sinceID, untilID *jsonInt64, sinceDate, untilDate *int64) ([]misskeyClipNote, error) {
	if h.clients == nil || h.clients.Content == nil || h.clients.User == nil {
		return nil, status.Error(codes.Unavailable, "content or user service unavailable")
	}
	const pageSize int32 = 100
	topics := make([]*contentpb.TopicInfo, 0, pageSize)
	for offset := int32(0); ; offset += pageSize {
		response, err := h.clients.Content.ListTopics(ctx, &contentpb.ListTopicsRequest{Status: contentStatusPublished, Type: "tweet", AuthorId: authorID, Limit: pageSize, Offset: offset})
		if err != nil {
			return nil, err
		}
		items := response.GetItems()
		for _, topic := range items {
			if topic == nil || topic.GetType() != "tweet" || topic.GetStatus() != contentStatusPublished || (!includeChannels && topic.GetChannelId() > 0) || (channelID > 0 && topic.GetChannelId() != channelID) {
				continue
			}
			topics = append(topics, topic)
		}
		if int32(len(topics)) >= limit || len(items) == 0 || int64(offset+pageSize) >= response.GetTotal() || int32(len(items)) < pageSize {
			break
		}
	}
	sort.SliceStable(topics, func(i, j int) bool {
		if topics[i].GetCreatedAt() == topics[j].GetCreatedAt() {
			return topics[i].GetId() > topics[j].GetId()
		}
		return topics[i].GetCreatedAt() > topics[j].GetCreatedAt()
	})
	items := make([]misskeyClipNote, 0, limit)
	for _, topic := range topics {
		if !compatNoteInWindow(topic.GetId(), topic.GetCreatedAt(), sinceID, untilID, sinceDate, untilDate) {
			continue
		}
		note, ok := h.misskeyNoteFromTopic(c, ctx, topic)
		if !ok {
			if c.IsAborted() {
				return nil, nil
			}
			continue
		}
		items = append(items, note)
		if int32(len(items)) >= limit {
			break
		}
	}
	return items, nil
}

func compatNoteInWindow(id, createdAt int64, sinceID, untilID *jsonInt64, sinceDate, untilDate *int64) bool {
	if (sinceID != nil && id <= sinceID.Int64()) || (untilID != nil && id >= untilID.Int64()) {
		return false
	}
	if (sinceDate != nil && createdAt <= *sinceDate) || (untilDate != nil && createdAt >= *untilDate) {
		return false
	}
	return true
}

func writeUsersNotesRPCError(c *gin.Context, err error) {
	if status.Code(err) == codes.NotFound {
		writeFollowingCompatError(c, "No such user.", "NO_SUCH_USER", usersNotesNoSuchUserID)
		return
	}
	writeRPCError(c, err)
}

func (h *Handler) createNoteCompat(c *gin.Context) {
	var request notesCreateCompatRequest
	if !decodeSensitiveAccountCompatRequest(c, &request) {
		writeFollowingCompatError(c, "Invalid param.", "INVALID_PARAM", "")
		return
	}
	if !validBasicNoteCreateRequest(&request) {
		writeFollowingCompatError(c, "Invalid param.", "INVALID_PARAM", "")
		return
	}
	ctx, cancel := rpcContext(c)
	defer cancel()
	if !h.ensureCurrentUserCanPost(c, ctx) {
		return
	}
	text := strings.TrimSpace(*request.Text)
	resp, err := h.clients.Content.CreateTopic(ctx, &contentpb.CreateTopicRequest{
		Slug:     compatNoteSlug(currentUserID(c)),
		Type:     "tweet",
		Body:     text,
		Tags:     extractCompatNoteTags(text),
		AuthorId: currentUserID(c),
	})
	if err != nil {
		writeRPCError(c, err)
		return
	}
	if resp.GetTopic() == nil {
		writeError(c, stdhttp.StatusBadGateway, "content service returned an empty note", "upstream_error")
		return
	}
	resp, err = h.clients.Content.PublishTopic(ctx, &contentpb.TopicIDRequest{Id: resp.GetTopic().GetId()})
	if err != nil {
		writeRPCError(c, err)
		return
	}
	topic := resp.GetTopic()
	if topic == nil {
		writeError(c, stdhttp.StatusBadGateway, "content service returned an empty note", "upstream_error")
		return
	}
	note, ok := h.misskeyNoteFromTopic(c, ctx, topic)
	if !ok {
		return
	}
	c.JSON(stdhttp.StatusOK, gin.H{"createdNote": note})
}

func validBasicNoteCreateRequest(request *notesCreateCompatRequest) bool {
	if request == nil || request.Text == nil || strings.TrimSpace(*request.Text) == "" {
		return false
	}
	if request.Visibility != "" && strings.ToLower(strings.TrimSpace(request.Visibility)) != "public" {
		return false
	}
	if request.CW != nil || request.LocalOnly || request.ReactionAcceptance != nil || request.NoExtractMentions || request.NoExtractHashtags || request.NoExtractEmojis || request.ReplyID != nil || request.RenoteID != nil || request.ChannelID != nil || len(request.VisibleUserIDs) > 0 || len(request.FileIDs) > 0 || len(request.MediaIDs) > 0 || request.Poll != nil {
		return false
	}
	return true
}

func compatNoteSlug(userID int64) string {
	return "note-" + strconv.FormatInt(userID, 10) + "-" + strconv.FormatInt(time.Now().UnixNano(), 10)
}

func extractCompatNoteTags(text string) []string {
	seen := make(map[string]struct{})
	var tags []string
	for _, word := range strings.Fields(text) {
		if !strings.HasPrefix(word, "#") || len(word) == 1 {
			continue
		}
		tag := strings.Trim(word[1:], ".,!?;:()[]{}")
		if tag == "" {
			continue
		}
		if _, ok := seen[tag]; ok {
			continue
		}
		seen[tag] = struct{}{}
		tags = append(tags, tag)
	}
	return tags
}

func (h *Handler) deleteNoteCompat(c *gin.Context) {
	var request notesShowCompatRequest
	if !decodeSensitiveAccountCompatRequest(c, &request) || request.NoteID == nil || request.NoteID.Int64() <= 0 {
		writeFollowingCompatError(c, "Invalid param.", "INVALID_PARAM", "")
		return
	}
	ctx, cancel := rpcContext(c)
	defer cancel()
	id := request.NoteID.Int64()
	topicResponse, topicErr := h.clients.Content.GetTopic(ctx, &contentpb.GetTopicRequest{Key: &contentpb.GetTopicRequest_Id{Id: id}, ViewerUserId: currentUserID(c)})
	articleResponse, articleErr := h.clients.Content.GetArticle(ctx, &contentpb.GetArticleRequest{Key: &contentpb.GetArticleRequest_Id{Id: id}})
	topic := topicResponse.GetTopic()
	article := articleResponse.GetArticle()
	if topicErr != nil && status.Code(topicErr) != codes.NotFound {
		writeRPCError(c, topicErr)
		return
	}
	if articleErr != nil && status.Code(articleErr) != codes.NotFound {
		writeRPCError(c, articleErr)
		return
	}
	if topic != nil && article != nil {
		writeError(c, stdhttp.StatusConflict, "note id matches both an article and a topic", "ambiguous_note_id")
		return
	}
	if topic != nil {
		if topic.GetAuthorId() != currentUserID(c) {
			writeFollowingCompatError(c, "Access denied.", "ACCESS_DENIED", notesDeleteAccessDeniedID)
			return
		}
		if topic.GetType() != "tweet" || topic.GetStatus() != contentStatusPublished {
			writeFollowingCompatError(c, "No such note.", "NO_SUCH_NOTE", notesDeleteNoSuchNoteID)
			return
		}
		if _, err := h.clients.Content.ArchiveTopic(ctx, &contentpb.TopicIDRequest{Id: id}); err != nil {
			writeRPCError(c, err)
			return
		}
		c.Status(stdhttp.StatusNoContent)
		return
	}
	if article != nil {
		if article.GetAuthorId() != currentUserID(c) {
			writeFollowingCompatError(c, "Access denied.", "ACCESS_DENIED", notesDeleteAccessDeniedID)
			return
		}
		if article.GetStatus() != contentStatusPublished {
			writeFollowingCompatError(c, "No such note.", "NO_SUCH_NOTE", notesDeleteNoSuchNoteID)
			return
		}
		if _, err := h.clients.Content.ArchiveArticle(ctx, &contentpb.ArticleIDRequest{Id: id}); err != nil {
			writeRPCError(c, err)
			return
		}
		c.Status(stdhttp.StatusNoContent)
		return
	}
	writeFollowingCompatError(c, "No such note.", "NO_SUCH_NOTE", notesDeleteNoSuchNoteID)
}

func (h *Handler) showNoteCompat(c *gin.Context) {
	var request notesShowCompatRequest
	if !decodeSensitiveAccountCompatRequest(c, &request) || request.NoteID == nil || request.NoteID.Int64() <= 0 {
		writeFollowingCompatError(c, "Invalid param.", "INVALID_PARAM", "")
		return
	}
	ctx, cancel := rpcContext(c)
	defer cancel()
	note, ok := h.resolveMisskeyNote(c, ctx, request.NoteID.Int64())
	if !ok {
		return
	}
	c.JSON(stdhttp.StatusOK, note)
}

func (h *Handler) resolveMisskeyNote(c *gin.Context, ctx context.Context, id int64) (misskeyClipNote, bool) {
	articleResponse, articleErr := h.clients.Content.GetArticle(ctx, &contentpb.GetArticleRequest{Key: &contentpb.GetArticleRequest_Id{Id: id}})
	topicResponse, topicErr := h.clients.Content.GetTopic(ctx, &contentpb.GetTopicRequest{Key: &contentpb.GetTopicRequest_Id{Id: id}, ViewerUserId: currentUserID(c)})
	article := articleResponse.GetArticle()
	topic := topicResponse.GetTopic()
	articlePublished := articleErr == nil && article != nil && article.GetStatus() == contentStatusPublished
	topicPublished := topicErr == nil && topic != nil && topic.GetStatus() == contentStatusPublished
	if articlePublished && topicPublished {
		writeError(c, stdhttp.StatusConflict, "note id matches both an article and a topic", "ambiguous_note_id")
		return misskeyClipNote{}, false
	}
	if articlePublished {
		return h.misskeyNoteFromArticle(c, ctx, article)
	}
	if topicPublished {
		return h.misskeyNoteFromTopic(c, ctx, topic)
	}
	if articleErr != nil && status.Code(articleErr) != codes.NotFound {
		writeRPCError(c, articleErr)
		return misskeyClipNote{}, false
	}
	if topicErr != nil && status.Code(topicErr) != codes.NotFound {
		writeRPCError(c, topicErr)
		return misskeyClipNote{}, false
	}
	writeFollowingCompatError(c, "No such note.", "NO_SUCH_NOTE", notesShowNoSuchNoteID)
	return misskeyClipNote{}, false
}

func (h *Handler) misskeyNoteFromArticle(c *gin.Context, ctx context.Context, article *contentpb.ArticleInfo) (misskeyClipNote, bool) {
	text := strings.TrimSpace(article.GetBody())
	if text == "" {
		text = strings.TrimSpace(article.GetSummary())
	}
	if text == "" {
		text = article.GetTitle()
	}
	return h.misskeyNoteFromFields(c, ctx, article.GetId(), article.GetCreatedAt(), text, article.GetTags(), article.GetAuthorId())
}

func (h *Handler) misskeyNoteFromTopic(c *gin.Context, ctx context.Context, topic *contentpb.TopicInfo) (misskeyClipNote, bool) {
	text := strings.TrimSpace(topic.GetBody())
	if text == "" {
		text = topic.GetTitle()
	}
	return h.misskeyNoteFromFields(c, ctx, topic.GetId(), topic.GetCreatedAt(), text, topic.GetTags(), topic.GetAuthorId())
}

func (h *Handler) misskeyNoteFromFields(c *gin.Context, ctx context.Context, id, createdAt int64, text string, tags []string, authorID int64) (misskeyClipNote, bool) {
	userResponse, err := h.clients.User.GetUser(ctx, &userpb.UserIDRequest{Id: authorID})
	if err != nil {
		writeRPCError(c, err)
		return misskeyClipNote{}, false
	}
	author := userResponse.GetUser()
	if author == nil {
		writeError(c, stdhttp.StatusBadGateway, "note author not found", "upstream_error")
		return misskeyClipNote{}, false
	}
	h.sanitizeUserProfileTheme(ctx, author)
	value := strconv.FormatInt(id, 10)
	return misskeyClipNote{
		ID: value, ThreadID: value, CreatedAt: formatUnixMilli(createdAt), Text: text,
		UserID: strconv.FormatInt(authorID, 10), User: toMisskeyUserLite(author), Visibility: "public",
		Mentions: []string{}, VisibleUserIDs: []string{}, FileIDs: []string{}, Files: []any{}, Tags: tags,
		Emojis: map[string]string{}, ReactionEmojis: map[string]string{}, Reactions: map[string]int64{},
	}, true
}
