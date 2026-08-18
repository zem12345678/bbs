package http

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	stdhttp "net/http"
	"sort"
	"strconv"
	"time"

	"api-gateway/api/proto/reactionpb"

	"github.com/gin-gonic/gin"
)

const favoriteExportPageSize = int32(100)

const favoriteExportTimestampLayout = "2006-01-02T15:04:05.000Z"

type favoriteExportRecord struct {
	ID        string                   `json:"id"`
	CreatedAt string                   `json:"createdAt"`
	Note      favoriteExportNoteRecord `json:"note"`
}

type favoriteExportNoteRecord struct {
	ID                 string                `json:"id"`
	Text               string                `json:"text"`
	CreatedAt          string                `json:"createdAt"`
	FileIDs            []string              `json:"fileIds"`
	ReplyID            *string               `json:"replyId"`
	RenoteID           *string               `json:"renoteId"`
	Poll               *clipExportPollRecord `json:"poll"`
	CW                 *string               `json:"cw"`
	Visibility         string                `json:"visibility"`
	VisibleUserIDs     []string              `json:"visibleUserIds"`
	LocalOnly          bool                  `json:"localOnly"`
	ReactionAcceptance *string               `json:"reactionAcceptance"`
	URI                *string               `json:"uri"`
	URL                *string               `json:"url"`
	User               clipExportUserRecord  `json:"user"`
}

func (h *Handler) exportFavorites(c *gin.Context) {
	if h == nil || h.clients == nil || h.clients.Reaction == nil || h.clients.Content == nil || h.clients.User == nil {
		writeError(c, stdhttp.StatusServiceUnavailable, "favorite export dependencies unavailable", "service_unavailable")
		return
	}
	h.deliverUserExport(c, userExportSpec{
		label: "favorite", filenamePrefix: "favorites", exportedEntity: "favorite",
		extension: ".json", contentType: "application/json",
		gate: h.favoriteExportGate, build: h.buildFavoriteExport,
	})
}

func (h *Handler) buildFavoriteExport(ctx context.Context, userID int64) ([]byte, error) {
	favorites, err := h.allFavoriteRelations(ctx, userID)
	if err != nil {
		return nil, err
	}
	users := make(map[int64]clipExportUserRecord)
	result := make([]favoriteExportRecord, 0, len(favorites))
	for _, favorite := range favorites {
		if favorite == nil || favorite.GetId() <= 0 || favorite.GetEntity() == nil || favorite.GetEntity().GetEntityId() <= 0 {
			return nil, errors.New("invalid favorite export record")
		}
		item := &reactionpb.CollectionItemInfo{Entity: favorite.GetEntity()}
		note, err := h.clipExportNote(ctx, item, users)
		if err != nil {
			return nil, err
		}
		createdAt, err := favoriteExportTimestampFromString(note.CreatedAt)
		if err != nil {
			return nil, err
		}
		noteCreatedAt := createdAt
		result = append(result, favoriteExportRecord{
			ID:        strconv.FormatInt(favorite.GetId(), 10),
			CreatedAt: favoriteExportTimestamp(favorite.GetCreatedAt()),
			Note: favoriteExportNoteRecord{
				ID: note.ID, Text: note.Text, CreatedAt: noteCreatedAt,
				FileIDs: note.FileIDs, ReplyID: note.ReplyID, RenoteID: note.RenoteID,
				Poll: note.Poll, CW: note.CW, Visibility: note.Visibility,
				VisibleUserIDs: note.VisibleUserIDs, LocalOnly: note.LocalOnly,
				ReactionAcceptance: note.ReactionAcceptance, URI: note.URI, URL: note.URL,
				User: note.User,
			},
		})
	}
	return json.Marshal(result)
}

func (h *Handler) allFavoriteRelations(ctx context.Context, userID int64) ([]*reactionpb.FavoriteInfo, error) {
	result := make([]*reactionpb.FavoriteInfo, 0)
	for _, entityType := range []string{"article", "topic"} {
		var afterID int64
		for {
			response, err := h.clients.Reaction.ListFavorites(ctx, &reactionpb.ListFavoritesRequest{
				UserId: userID, EntityType: entityType, Limit: favoriteExportPageSize,
				AfterId: afterID, AscendingById: true,
			})
			if err != nil {
				return nil, err
			}
			items := response.GetItems()
			if len(items) == 0 {
				break
			}
			for _, item := range items {
				if item == nil || item.GetId() <= afterID || item.GetEntity() == nil || item.GetEntity().GetEntityType() != entityType {
					return nil, errors.New("invalid favorite export cursor")
				}
				result = append(result, item)
				afterID = item.GetId()
			}
			if len(items) < int(favoriteExportPageSize) {
				break
			}
		}
	}
	sort.Slice(result, func(i, j int) bool { return result[i].GetId() < result[j].GetId() })
	return result, nil
}

func favoriteExportTimestamp(value int64) string {
	return time.UnixMilli(value).UTC().Format(favoriteExportTimestampLayout)
}

func favoriteExportTimestampFromString(value string) (string, error) {
	parsed, err := time.Parse(time.RFC3339Nano, value)
	if err != nil {
		return "", fmt.Errorf("invalid note timestamp: %w", err)
	}
	return parsed.UTC().Format(favoriteExportTimestampLayout), nil
}
