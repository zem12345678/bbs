package http

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	stdhttp "net/http"
	"sort"
	"strconv"
	"strings"
	"time"

	"api-gateway/api/proto/contentpb"
	"api-gateway/api/proto/filepb"

	"github.com/gin-gonic/gin"
)

const (
	noteExportPageSize        = int32(100)
	noteExportTimestampLayout = "2006-01-02T15:04:05.000Z"
)

type noteExportRecord struct {
	ID                 string                 `json:"id"`
	Text               string                 `json:"text"`
	CreatedAt          string                 `json:"createdAt"`
	FileIDs            []string               `json:"fileIds"`
	Files              []noteExportFileRecord `json:"files"`
	ReplyID            *string                `json:"replyId"`
	RenoteID           *string                `json:"renoteId"`
	Poll               *clipExportPollRecord  `json:"poll"`
	CW                 *string                `json:"cw"`
	Visibility         string                 `json:"visibility"`
	VisibleUserIDs     []string               `json:"visibleUserIds"`
	LocalOnly          bool                   `json:"localOnly"`
	ReactionAcceptance *string                `json:"reactionAcceptance"`
}

type noteExportFileRecord struct {
	ID           string         `json:"id"`
	CreatedAt    string         `json:"createdAt"`
	Name         string         `json:"name"`
	Type         string         `json:"type"`
	MD5          string         `json:"md5"`
	Size         int64          `json:"size"`
	IsSensitive  bool           `json:"isSensitive"`
	Blurhash     *string        `json:"blurhash"`
	Properties   map[string]any `json:"properties"`
	URL          string         `json:"url"`
	ThumbnailURL *string        `json:"thumbnailUrl"`
	Comment      *string        `json:"comment"`
	FolderID     *string        `json:"folderId"`
	Folder       any            `json:"folder"`
	UserID       *string        `json:"userId"`
	User         any            `json:"user"`
}

type noteExportSource struct {
	kind      string
	id        int64
	text      string
	createdAt int64
	poll      *clipExportPollRecord
	files     []noteExportFileRecord
}

func (h *Handler) exportNotes(c *gin.Context) {
	if h == nil || h.clients == nil || h.clients.Content == nil || h.clients.File == nil {
		writeError(c, stdhttp.StatusServiceUnavailable, "note export dependencies unavailable", "service_unavailable")
		return
	}
	h.deliverUserExport(c, userExportSpec{
		label: "note", filenamePrefix: "notes", exportedEntity: "note",
		extension: ".json", contentType: "application/json",
		gate: h.noteExportGate, build: h.buildNoteExport,
	})
}

func (h *Handler) buildNoteExport(ctx context.Context, userID int64) ([]byte, error) {
	sources, err := h.allNoteExportSources(ctx, userID)
	if err != nil {
		return nil, err
	}
	records := make([]noteExportRecord, 0, len(sources))
	for _, source := range sources {
		fileIDs := make([]string, 0, len(source.files))
		for _, file := range source.files {
			fileIDs = append(fileIDs, file.ID)
		}
		records = append(records, noteExportRecord{
			ID: strconv.FormatInt(source.id, 10), Text: source.text,
			CreatedAt: noteExportTimestamp(source.createdAt), FileIDs: fileIDs,
			Files: source.files, Poll: source.poll, Visibility: "public",
			VisibleUserIDs: []string{},
		})
	}
	return json.Marshal(records)
}

func (h *Handler) allNoteExportSources(ctx context.Context, userID int64) ([]noteExportSource, error) {
	articles, err := h.allNoteExportArticles(ctx, userID)
	if err != nil {
		return nil, err
	}
	topics, err := h.allNoteExportTopics(ctx, userID)
	if err != nil {
		return nil, err
	}
	result := append(articles, topics...)
	sort.Slice(result, func(i, j int) bool {
		if result[i].id == result[j].id {
			return result[i].kind < result[j].kind
		}
		return result[i].id < result[j].id
	})
	return result, nil
}

func (h *Handler) allNoteExportArticles(ctx context.Context, userID int64) ([]noteExportSource, error) {
	result := make([]noteExportSource, 0)
	var afterID int64
	for {
		response, err := h.clients.Content.ListArticles(ctx, &contentpb.ListArticlesRequest{
			AuthorId: userID, Limit: noteExportPageSize, AfterId: afterID, AscendingById: true,
		})
		if err != nil {
			return nil, err
		}
		items := response.GetItems()
		if len(items) == 0 {
			break
		}
		for _, article := range items {
			if article == nil || article.GetId() <= afterID || article.GetAuthorId() != userID {
				return nil, errors.New("invalid article export cursor")
			}
			text := article.GetBody()
			if strings.TrimSpace(text) == "" {
				text = article.GetTitle()
			}
			result = append(result, noteExportSource{kind: "article", id: article.GetId(), text: text, createdAt: article.GetCreatedAt(), files: []noteExportFileRecord{}})
			afterID = article.GetId()
		}
		if len(items) < int(noteExportPageSize) {
			break
		}
	}
	return result, nil
}

func (h *Handler) allNoteExportTopics(ctx context.Context, userID int64) ([]noteExportSource, error) {
	result := make([]noteExportSource, 0)
	var afterID int64
	for {
		response, err := h.clients.Content.ListTopics(ctx, &contentpb.ListTopicsRequest{
			AuthorId: userID, Limit: noteExportPageSize, AfterId: afterID, AscendingById: true,
		})
		if err != nil {
			return nil, err
		}
		items := response.GetItems()
		if len(items) == 0 {
			break
		}
		for _, listed := range items {
			if listed == nil || listed.GetId() <= afterID || listed.GetAuthorId() != userID {
				return nil, errors.New("invalid topic export cursor")
			}
			detail, err := h.clients.Content.GetTopic(ctx, &contentpb.GetTopicRequest{
				Key: &contentpb.GetTopicRequest_Id{Id: listed.GetId()}, ViewerUserId: userID,
			})
			if err != nil {
				return nil, err
			}
			topic := detail.GetTopic()
			if topic == nil || topic.GetId() != listed.GetId() || topic.GetAuthorId() != userID {
				return nil, fmt.Errorf("topic %d unavailable for export", listed.GetId())
			}
			files, err := h.noteExportTopicFiles(ctx, userID, topic.GetId())
			if err != nil {
				return nil, err
			}
			text := topic.GetBody()
			if strings.TrimSpace(text) == "" {
				text = topic.GetTitle()
			}
			result = append(result, noteExportSource{
				kind: "topic", id: topic.GetId(), text: text, createdAt: topic.GetCreatedAt(),
				poll: noteExportPoll(topic.GetPoll()), files: files,
			})
			afterID = listed.GetId()
		}
		if len(items) < int(noteExportPageSize) {
			break
		}
	}
	return result, nil
}

func (h *Handler) noteExportTopicFiles(ctx context.Context, userID, topicID int64) ([]noteExportFileRecord, error) {
	response, err := h.clients.File.ListOwnedTopicAttachments(ctx, &filepb.ListOwnedTopicAttachmentsRequest{TopicId: topicID, OwnerId: userID})
	if err != nil {
		return nil, err
	}
	items := response.GetItems()
	result := make([]noteExportFileRecord, 0, len(items))
	for _, item := range items {
		if item == nil || item.GetId() <= 0 || item.GetTopicId() != topicID || item.GetOwnerId() != userID {
			return nil, errors.New("invalid note export attachment")
		}
		if h.publicBaseURL == "" {
			return nil, errors.New("public base URL required for note attachment export")
		}
		result = append(result, noteExportFileRecord{
			ID: strconv.FormatInt(item.GetId(), 10), CreatedAt: noteExportTimestamp(item.GetCreatedAt()),
			Name: item.GetOriginalName(), Type: item.GetContentType(), Size: item.GetSizeBytes(),
			Properties: map[string]any{}, URL: h.publicBaseURL + "/api/v1/attachments/" + strconv.FormatInt(item.GetId(), 10) + "/download",
		})
	}
	return result, nil
}

func noteExportPoll(value *contentpb.TopicPollInfo) *clipExportPollRecord {
	if value == nil {
		return nil
	}
	result := &clipExportPollRecord{Multiple: value.GetMultiple(), Choices: make([]string, 0, len(value.GetChoices())), Votes: make([]int64, 0, len(value.GetChoices()))}
	for _, choice := range value.GetChoices() {
		result.Choices = append(result.Choices, choice.GetText())
		result.Votes = append(result.Votes, choice.GetVotes())
	}
	if value.GetExpiresAt() > 0 {
		expiresAt := noteExportTimestamp(value.GetExpiresAt())
		result.ExpiresAt = &expiresAt
	}
	return result
}

func noteExportTimestamp(value int64) string {
	return time.UnixMilli(value).UTC().Format(noteExportTimestampLayout)
}
