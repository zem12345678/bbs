package http

import (
	"encoding/json"
	"net/http"
	"strings"
	"time"

	"api-gateway/pkg/http/response"

	"github.com/gin-gonic/gin"
)

const defaultAnnouncementLimit int32 = 10
const maxAnnouncementLimit int32 = 100

type announcementListRequest struct {
	Limit int32 `json:"limit"`
}

type announcementShowRequest struct {
	AnnouncementID     string `json:"announcement_id"`
	AnnouncementIDJSON string `json:"announcementId"`
}

type publicAnnouncement struct {
	ID                     string `json:"id"`
	Title                  string `json:"title"`
	Text                   string `json:"text"`
	ImageURL               string `json:"image_url,omitempty"`
	Icon                   string `json:"icon"`
	Display                string `json:"display"`
	NeedConfirmationToRead bool   `json:"need_confirmation_to_read"`
	Silence                bool   `json:"silence"`
	Confetti               bool   `json:"confetti"`
	ForYou                 bool   `json:"for_you"`
	IsRead                 bool   `json:"is_read"`
	Active                 bool   `json:"active"`
	CreatedAt              int64  `json:"created_at,omitempty"`
	UpdatedAt              int64  `json:"updated_at,omitempty"`
}

type announcementSettingItem struct {
	ID                     string `json:"id"`
	Title                  string `json:"title"`
	Text                   string `json:"text"`
	Content                string `json:"content"`
	ImageURL               string `json:"image_url"`
	ImageURLJSON           string `json:"imageUrl"`
	Icon                   string `json:"icon"`
	Display                string `json:"display"`
	NeedConfirmationToRead bool   `json:"need_confirmation_to_read"`
	NeedConfirmationJSON   bool   `json:"needConfirmationToRead"`
	Silence                bool   `json:"silence"`
	Confetti               bool   `json:"confetti"`
	ForYou                 bool   `json:"for_you"`
	ForYouJSON             bool   `json:"forYou"`
	Active                 *bool  `json:"active"`
	StartsAt               int64  `json:"starts_at"`
	StartsAtJSON           int64  `json:"startsAt"`
	EndsAt                 int64  `json:"ends_at"`
	EndsAtJSON             int64  `json:"endsAt"`
	CreatedAt              int64  `json:"created_at"`
	CreatedAtJSON          int64  `json:"createdAt"`
	UpdatedAt              int64  `json:"updated_at"`
	UpdatedAtJSON          int64  `json:"updatedAt"`
}

func (h *Handler) listAnnouncements(c *gin.Context) {
	limit, ok := announcementListLimit(c)
	if !ok {
		return
	}
	announcements, ok := h.publicAnnouncements(c)
	if !ok {
		return
	}
	items := make([]publicAnnouncement, 0, min(len(announcements), int(limit)))
	for _, item := range announcements {
		if !item.Active {
			continue
		}
		items = append(items, item)
		if int32(len(items)) >= limit {
			break
		}
	}
	response.Success(c, gin.H{"items": items, "total": len(items)})
}

func (h *Handler) getAnnouncement(c *gin.Context) {
	h.writeAnnouncement(c, strings.TrimSpace(c.Param("id")))
}

func (h *Handler) showAnnouncement(c *gin.Context) {
	var req announcementShowRequest
	if !bindJSON(c, &req) {
		return
	}
	id := strings.TrimSpace(req.AnnouncementID)
	if id == "" {
		id = strings.TrimSpace(req.AnnouncementIDJSON)
	}
	if id == "" {
		writeError(c, http.StatusBadRequest, "announcement_id is required", "bad_request")
		return
	}
	h.writeAnnouncement(c, id)
}

func (h *Handler) writeAnnouncement(c *gin.Context, id string) {
	announcements, ok := h.publicAnnouncements(c)
	if !ok {
		return
	}
	for _, item := range announcements {
		if item.ID == id && item.Active {
			response.Success(c, gin.H{"announcement": item})
			return
		}
	}
	writeError(c, http.StatusNotFound, "announcement not found", "not_found")
}

func (h *Handler) publicAnnouncements(c *gin.Context) ([]publicAnnouncement, bool) {
	settings, ok := h.loadPublicSiteSettings(c)
	if !ok {
		return nil, false
	}
	return parsePublicAnnouncements(settings["site_announcements"], time.Now().UnixMilli()), true
}

func announcementListLimit(c *gin.Context) (int32, bool) {
	limit := normalizeAnnouncementLimit(queryInt32(c, "limit", defaultAnnouncementLimit))
	if c.Request.Method != http.MethodPost || c.Request.Body == nil || c.Request.ContentLength == 0 {
		return limit, true
	}
	var req announcementListRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		writeError(c, http.StatusBadRequest, "invalid request body", "bad_request")
		return 0, false
	}
	if req.Limit > 0 {
		limit = normalizeAnnouncementLimit(req.Limit)
	}
	return limit, true
}

func parsePublicAnnouncements(raw string, now int64) []publicAnnouncement {
	var input []announcementSettingItem
	if err := json.Unmarshal([]byte(raw), &input); err != nil {
		return []publicAnnouncement{}
	}
	items := make([]publicAnnouncement, 0, len(input))
	seen := make(map[string]struct{}, len(input))
	for _, item := range input {
		announcement, ok := toPublicAnnouncement(item, now)
		if !ok {
			continue
		}
		if _, duplicate := seen[announcement.ID]; duplicate {
			continue
		}
		seen[announcement.ID] = struct{}{}
		items = append(items, announcement)
	}
	return items
}

func toPublicAnnouncement(item announcementSettingItem, now int64) (publicAnnouncement, bool) {
	id := strings.TrimSpace(item.ID)
	title := strings.TrimSpace(item.Title)
	text := strings.TrimSpace(item.Text)
	if text == "" {
		text = strings.TrimSpace(item.Content)
	}
	if id == "" || title == "" || text == "" {
		return publicAnnouncement{}, false
	}
	startsAt := firstInt64(item.StartsAt, item.StartsAtJSON)
	endsAt := firstInt64(item.EndsAt, item.EndsAtJSON)
	active := true
	if item.Active != nil {
		active = *item.Active
	}
	if startsAt > 0 && now < startsAt {
		active = false
	}
	if endsAt > 0 && now > endsAt {
		active = false
	}
	imageURL := strings.TrimSpace(firstString(item.ImageURL, item.ImageURLJSON))
	return publicAnnouncement{
		ID:                     id,
		Title:                  title,
		Text:                   text,
		ImageURL:               imageURL,
		Icon:                   normalizeAnnouncementIcon(item.Icon),
		Display:                normalizeAnnouncementDisplay(item.Display),
		NeedConfirmationToRead: item.NeedConfirmationToRead || item.NeedConfirmationJSON,
		Silence:                item.Silence,
		Confetti:               item.Confetti,
		ForYou:                 item.ForYou || item.ForYouJSON,
		IsRead:                 false,
		Active:                 active,
		CreatedAt:              firstInt64(item.CreatedAt, item.CreatedAtJSON),
		UpdatedAt:              firstInt64(item.UpdatedAt, item.UpdatedAtJSON),
	}, true
}

func normalizeAnnouncementLimit(limit int32) int32 {
	if limit <= 0 {
		return defaultAnnouncementLimit
	}
	if limit > maxAnnouncementLimit {
		return maxAnnouncementLimit
	}
	return limit
}

func normalizeAnnouncementIcon(value string) string {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "warning", "error", "success":
		return strings.ToLower(strings.TrimSpace(value))
	default:
		return "info"
	}
}

func normalizeAnnouncementDisplay(value string) string {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "dialog", "banner":
		return strings.ToLower(strings.TrimSpace(value))
	default:
		return "normal"
	}
}

func firstString(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return value
		}
	}
	return ""
}

func firstInt64(values ...int64) int64 {
	for _, value := range values {
		if value > 0 {
			return value
		}
	}
	return 0
}
