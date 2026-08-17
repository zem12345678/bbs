package http

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	stdhttp "net/http"
	"strconv"
	"strings"

	"api-gateway/api/proto/contentpb"
	"api-gateway/api/proto/reactionpb"
	"api-gateway/api/proto/userpb"

	"github.com/gin-gonic/gin"
)

const (
	clipExportPageSize = int32(100)
)

type clipExportRecord struct {
	ID            string                 `json:"id"`
	Name          string                 `json:"name"`
	Description   *string                `json:"description"`
	LastClippedAt *string                `json:"lastClippedAt,omitempty"`
	ClipNotes     []clipExportItemRecord `json:"clipNotes"`
}

type clipExportItemRecord struct {
	ID        string               `json:"id"`
	CreatedAt string               `json:"createdAt"`
	Note      clipExportNoteRecord `json:"note"`
}

type clipExportNoteRecord struct {
	ID                 string                `json:"id"`
	Text               string                `json:"text"`
	CreatedAt          string                `json:"createdAt"`
	FileIDs            []string              `json:"fileIds"`
	ReplyID            *string               `json:"replyId"`
	RenoteID           *string               `json:"renoteId"`
	Poll               *clipExportPollRecord `json:"poll,omitempty"`
	CW                 *string               `json:"cw"`
	Visibility         string                `json:"visibility"`
	VisibleUserIDs     []string              `json:"visibleUserIds"`
	LocalOnly          bool                  `json:"localOnly"`
	ReactionAcceptance *string               `json:"reactionAcceptance"`
	URI                *string               `json:"uri"`
	URL                *string               `json:"url"`
	User               clipExportUserRecord  `json:"user"`
}

type clipExportUserRecord struct {
	ID       string  `json:"id"`
	Name     *string `json:"name"`
	Username string  `json:"username"`
	Host     *string `json:"host"`
	URI      *string `json:"uri"`
}

type clipExportPollRecord struct {
	Multiple  bool     `json:"multiple"`
	Choices   []string `json:"choices"`
	Votes     []int64  `json:"votes"`
	ExpiresAt *string  `json:"expiresAt"`
}

func (h *Handler) exportClips(c *gin.Context) {
	if h == nil || h.clients == nil || h.clients.Reaction == nil || h.clients.Content == nil || h.clients.User == nil {
		writeError(c, stdhttp.StatusServiceUnavailable, "clip export dependencies unavailable", "service_unavailable")
		return
	}
	h.deliverUserExport(c, userExportSpec{
		label: "clip", filenamePrefix: "clips", exportedEntity: "clip",
		extension: ".json", contentType: "application/json",
		gate: h.clipExportGate, build: h.buildClipExport,
	})
}

func (h *Handler) beginClipExport(c *gin.Context, userID int64) (ClipExportPermit, bool) {
	return h.beginUserExport(c, userID, "clip", h.clipExportGate)
}

func (h *Handler) buildClipExport(ctx context.Context, userID int64) ([]byte, error) {
	collections, err := h.allClipCollections(ctx, userID)
	if err != nil {
		return nil, err
	}
	records := make([]clipExportRecord, 0, len(collections))
	users := make(map[int64]clipExportUserRecord)
	for _, collection := range collections {
		items, err := h.allClipItems(ctx, userID, collection.GetId())
		if err != nil {
			return nil, err
		}
		exportedItems := make([]clipExportItemRecord, 0, len(items))
		for _, item := range items {
			note, err := h.clipExportNote(ctx, item, users)
			if err != nil {
				return nil, err
			}
			exportedItems = append(exportedItems, clipExportItemRecord{ID: strconv.FormatInt(item.GetId(), 10), CreatedAt: formatUnixMilli(item.GetCreatedAt()), Note: note})
		}
		record := clipExportRecord{
			ID: strconv.FormatInt(collection.GetId(), 10), Name: collection.GetName(),
			Description: optionalMisskeyText(collection.GetDescription()), ClipNotes: exportedItems,
		}
		if collection.GetLastClippedAt() > 0 {
			value := formatUnixMilli(collection.GetLastClippedAt())
			record.LastClippedAt = &value
		}
		records = append(records, record)
	}
	return json.Marshal(records)
}

func (h *Handler) allClipCollections(ctx context.Context, userID int64) ([]*reactionpb.CollectionInfo, error) {
	var result []*reactionpb.CollectionInfo
	var afterID int64
	for {
		response, err := h.clients.Reaction.ListCollections(ctx, &reactionpb.ListCollectionsRequest{
			UserId: userID, Limit: clipExportPageSize, AfterId: afterID, AscendingById: true,
		})
		if err != nil {
			return nil, err
		}
		items := response.GetItems()
		if len(items) == 0 {
			break
		}
		result = append(result, items...)
		nextAfterID := items[len(items)-1].GetId()
		if nextAfterID <= afterID {
			return nil, errors.New("reaction collection cursor did not advance")
		}
		afterID = nextAfterID
		if len(items) < int(clipExportPageSize) {
			break
		}
	}
	return result, nil
}

func (h *Handler) allClipItems(ctx context.Context, userID, collectionID int64) ([]*reactionpb.CollectionItemInfo, error) {
	var result []*reactionpb.CollectionItemInfo
	var afterID int64
	for {
		response, err := h.clients.Reaction.ListCollectionItems(ctx, &reactionpb.ListCollectionItemsRequest{
			UserId: userID, CollectionId: collectionID, Limit: clipExportPageSize, AfterId: afterID, AscendingById: true,
		})
		if err != nil {
			return nil, err
		}
		items := response.GetItems()
		if len(items) == 0 {
			break
		}
		result = append(result, items...)
		nextAfterID := items[len(items)-1].GetId()
		if nextAfterID <= afterID {
			return nil, errors.New("reaction collection item cursor did not advance")
		}
		afterID = nextAfterID
		if len(items) < int(clipExportPageSize) {
			break
		}
	}
	return result, nil
}

func (h *Handler) clipExportNote(ctx context.Context, item *reactionpb.CollectionItemInfo, users map[int64]clipExportUserRecord) (clipExportNoteRecord, error) {
	if item == nil || item.GetEntity() == nil || item.GetEntity().GetEntityId() <= 0 {
		return clipExportNoteRecord{}, errors.New("invalid clip item")
	}
	entityID := item.GetEntity().GetEntityId()
	var text string
	var authorID, createdAt int64
	var poll *clipExportPollRecord
	switch item.GetEntity().GetEntityType() {
	case "article":
		response, err := h.clients.Content.GetArticle(ctx, &contentpb.GetArticleRequest{Key: &contentpb.GetArticleRequest_Id{Id: entityID}})
		if err != nil {
			return clipExportNoteRecord{}, err
		}
		article := response.GetArticle()
		if article == nil {
			return clipExportNoteRecord{}, fmt.Errorf("article %d not found", entityID)
		}
		text, authorID, createdAt = article.GetBody(), article.GetAuthorId(), article.GetCreatedAt()
		if strings.TrimSpace(text) == "" {
			text = article.GetTitle()
		}
	case "topic":
		response, err := h.clients.Content.GetTopic(ctx, &contentpb.GetTopicRequest{Key: &contentpb.GetTopicRequest_Id{Id: entityID}})
		if err != nil {
			return clipExportNoteRecord{}, err
		}
		topic := response.GetTopic()
		if topic == nil {
			return clipExportNoteRecord{}, fmt.Errorf("topic %d not found", entityID)
		}
		text, authorID, createdAt = topic.GetBody(), topic.GetAuthorId(), topic.GetCreatedAt()
		if strings.TrimSpace(text) == "" {
			text = topic.GetTitle()
		}
		poll = clipExportPoll(topic.GetPoll())
	default:
		return clipExportNoteRecord{}, fmt.Errorf("unsupported clip entity type %q", item.GetEntity().GetEntityType())
	}
	user, ok := users[authorID]
	if !ok {
		response, err := h.clients.User.GetUser(ctx, &userpb.UserIDRequest{Id: authorID})
		if err != nil {
			return clipExportNoteRecord{}, err
		}
		if response.GetUser() == nil {
			return clipExportNoteRecord{}, fmt.Errorf("clip note author %d not found", authorID)
		}
		user = clipExportUserRecord{ID: strconv.FormatInt(authorID, 10), Name: optionalMisskeyText(response.GetUser().GetNickname()), Username: response.GetUser().GetUsername()}
		users[authorID] = user
	}
	return clipExportNoteRecord{
		ID: strconv.FormatInt(entityID, 10), Text: text, CreatedAt: formatUnixMilli(createdAt),
		FileIDs: []string{}, Poll: poll, Visibility: "public", VisibleUserIDs: []string{}, User: user,
	}, nil
}

func clipExportPoll(value *contentpb.TopicPollInfo) *clipExportPollRecord {
	if value == nil {
		return nil
	}
	result := &clipExportPollRecord{Multiple: value.GetMultiple(), Choices: make([]string, 0, len(value.GetChoices())), Votes: make([]int64, 0, len(value.GetChoices()))}
	for _, choice := range value.GetChoices() {
		result.Choices = append(result.Choices, choice.GetText())
		result.Votes = append(result.Votes, choice.GetVotes())
	}
	if value.GetExpiresAt() > 0 {
		expiresAt := formatUnixMilli(value.GetExpiresAt())
		result.ExpiresAt = &expiresAt
	}
	return result
}
