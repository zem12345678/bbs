package http

import (
	"context"
	stdhttp "net/http"
	"strconv"
	"strings"

	"api-gateway/api/proto/contentpb"
	"api-gateway/api/proto/userpb"

	"github.com/gin-gonic/gin"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

const notesShowNoSuchNoteID = "24fcbfc6-2e37-42b6-8388-c29b3861a08d"

type notesShowCompatRequest struct {
	NoteID *jsonInt64 `json:"noteId"`
}

func (h *Handler) registerNotesCompatRoutes(router *gin.Engine) {
	for _, prefix := range []string{"", "/api", "/api/v1"} {
		router.POST(prefix+"/notes/show", h.optionalAuth(), h.showNoteCompat)
	}
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
