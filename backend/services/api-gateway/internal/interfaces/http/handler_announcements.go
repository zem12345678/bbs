package http

import (
	"encoding/json"
	"net/http"
	"strings"
	"time"

	"api-gateway/api/proto/adminpb"
	"api-gateway/api/proto/userpb"
	"api-gateway/pkg/http/response"

	"github.com/gin-gonic/gin"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

const defaultAnnouncementLimit int32 = 10
const maxAnnouncementLimit int32 = 100

type announcementListRequest struct {
	Limit   int32  `json:"limit"`
	SinceID string `json:"sinceId"`
	UntilID string `json:"untilId"`
	Active  *bool  `json:"isActive"`
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
	var req announcementListRequest
	if value, exists := c.Get("announcement_list_request"); exists {
		req, _ = value.(announcementListRequest)
	}
	active := true
	if req.Active != nil {
		active = *req.Active
	}
	sinceID, untilID := req.SinceID, req.UntilID
	if c.Request.Method != http.MethodPost {
		sinceID, untilID = c.Query("sinceId"), c.Query("untilId")
	}
	items, typed := h.typedPublicAnnouncements(c, limit, sinceID, untilID, &active)
	if typed {
		if c.IsAborted() {
			return
		}
		if c.Request.Method == http.MethodPost {
			c.JSON(http.StatusOK, publicAnnouncementPayloads(items, currentUserID(c) > 0))
		} else {
			legacyItems := make([]publicAnnouncement, 0, len(items))
			for _, item := range items {
				legacyItems = append(legacyItems, publicAnnouncementFromProto(item))
			}
			response.Success(c, gin.H{"items": legacyItems, "total": len(legacyItems)})
		}
		return
	}
	legacy, ok := h.publicAnnouncements(c)
	if !ok {
		return
	}
	visible := make([]publicAnnouncement, 0, min(len(legacy), int(limit)))
	for _, item := range legacy {
		if item.Active != active {
			continue
		}
		visible = append(visible, item)
		if int32(len(visible)) >= limit {
			break
		}
	}
	if c.Request.Method == http.MethodPost {
		c.JSON(http.StatusOK, visible)
	} else {
		response.Success(c, gin.H{"items": visible, "total": len(visible)})
	}
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
	if item, typed := h.typedPublicAnnouncement(c, id); typed {
		if c.Request.Method == http.MethodPost {
			c.JSON(http.StatusOK, publicAnnouncementPayloadFromProto(item, currentUserID(c) > 0))
		} else {
			response.Success(c, gin.H{"announcement": publicAnnouncementFromProto(item)})
		}
		return
	}
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

func (h *Handler) typedPublicAnnouncements(c *gin.Context, limit int32, sinceID string, untilID string, active *bool) ([]*adminpb.AnnouncementInfo, bool) {
	if h.clients == nil || h.clients.Admin == nil {
		return nil, false
	}
	userID, userCreatedAt := h.announcementViewer(c)
	ctx, cancel := rpcContext(c)
	defer cancel()
	resp, err := h.clients.Admin.ListPublicAnnouncements(ctx, &adminpb.ListPublicAnnouncementsRequest{UserId: userID, UserCreatedAt: userCreatedAt, Limit: limit, SinceId: strings.TrimSpace(sinceID), UntilId: strings.TrimSpace(untilID), Active: active})
	if err != nil {
		if status.Code(err) == codes.Unimplemented {
			return nil, false
		}
		writeRPCError(c, err)
		return nil, true
	}
	return resp.GetItems(), true
}

func (h *Handler) typedPublicAnnouncement(c *gin.Context, id string) (*adminpb.AnnouncementInfo, bool) {
	if h.clients == nil || h.clients.Admin == nil {
		return nil, false
	}
	userID, userCreatedAt := h.announcementViewer(c)
	ctx, cancel := rpcContext(c)
	defer cancel()
	resp, err := h.clients.Admin.GetPublicAnnouncement(ctx, &adminpb.GetPublicAnnouncementRequest{UserId: userID, UserCreatedAt: userCreatedAt, Id: strings.TrimSpace(id)})
	if err != nil {
		if status.Code(err) == codes.Unimplemented {
			return nil, false
		}
		writeRPCError(c, err)
		return nil, true
	}
	return resp.GetAnnouncement(), true
}

func (h *Handler) announcementViewer(c *gin.Context) (int64, int64) {
	userID := currentUserID(c)
	if userID <= 0 {
		return 0, 0
	}
	if h.clients == nil || h.clients.User == nil {
		return userID, 0
	}
	ctx, cancel := rpcContext(c)
	defer cancel()
	resp, err := h.clients.User.GetUser(ctx, &userpb.UserIDRequest{Id: userID})
	if err != nil || resp.GetUser() == nil {
		return userID, 0
	}
	return userID, resp.GetUser().GetCreatedAt()
}

type publicAnnouncementPayload struct {
	ID                     string  `json:"id"`
	CreatedAt              string  `json:"createdAt"`
	UpdatedAt              *string `json:"updatedAt"`
	Title                  string  `json:"title"`
	Text                   string  `json:"text"`
	ImageURL               *string `json:"imageUrl"`
	Icon                   string  `json:"icon"`
	Display                string  `json:"display"`
	NeedConfirmationToRead bool    `json:"needConfirmationToRead"`
	Silence                bool    `json:"silence"`
	Confetti               bool    `json:"confetti"`
	ForYou                 bool    `json:"forYou"`
	IsRead                 *bool   `json:"isRead,omitempty"`
	IsActive               bool    `json:"isActive"`
}

func publicAnnouncementPayloads(items []*adminpb.AnnouncementInfo, includeRead bool) []publicAnnouncementPayload {
	result := make([]publicAnnouncementPayload, 0, len(items))
	for _, item := range items {
		result = append(result, publicAnnouncementPayloadFromProto(item, includeRead))
	}
	return result
}

func publicAnnouncementPayloadFromProto(item *adminpb.AnnouncementInfo, includeRead bool) publicAnnouncementPayload {
	result := publicAnnouncementPayload{ID: item.GetId(), CreatedAt: formatUnixMilli(item.GetCreatedAt()), UpdatedAt: formatUnixMilliPointer(item.GetUpdatedAt()), Title: item.GetTitle(), Text: item.GetText(), Icon: item.GetIcon(), Display: item.GetDisplay(), NeedConfirmationToRead: item.GetNeedConfirmationToRead(), Silence: item.GetSilence(), Confetti: item.GetConfetti(), ForYou: item.GetForYou(), IsActive: item.GetActive()}
	if item.GetImageUrl() != "" {
		imageURL := item.GetImageUrl()
		result.ImageURL = &imageURL
	}
	if includeRead {
		value := item.GetIsRead()
		result.IsRead = &value
	}
	return result
}

func publicAnnouncementFromProto(item *adminpb.AnnouncementInfo) publicAnnouncement {
	return publicAnnouncement{ID: item.GetId(), Title: item.GetTitle(), Text: item.GetText(), ImageURL: item.GetImageUrl(), Icon: item.GetIcon(), Display: item.GetDisplay(), NeedConfirmationToRead: item.GetNeedConfirmationToRead(), Silence: item.GetSilence(), Confetti: item.GetConfetti(), ForYou: item.GetForYou(), IsRead: item.GetIsRead(), Active: item.GetActive(), CreatedAt: item.GetCreatedAt(), UpdatedAt: item.GetUpdatedAt()}
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
	c.Set("announcement_list_request", req)
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
