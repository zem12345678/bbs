package http

import (
	"encoding/json"
	stdhttp "net/http"
	"strconv"
	"strings"

	"api-gateway/api/proto/adminpb"

	"github.com/gin-gonic/gin"
)

type adminAnnouncementListRequest struct {
	Limit   int32     `json:"limit"`
	SinceID string    `json:"sinceId"`
	UntilID string    `json:"untilId"`
	UserID  jsonInt64 `json:"userId"`
	Status  string    `json:"status"`
}

type adminAnnouncementCreateRequest struct {
	Title                  string          `json:"title"`
	Text                   string          `json:"text"`
	ImageURL               json.RawMessage `json:"imageUrl"`
	Icon                   string          `json:"icon"`
	Display                string          `json:"display"`
	ForExistingUsers       bool            `json:"forExistingUsers"`
	ForRoles               []string        `json:"forRoles"`
	Silence                bool            `json:"silence"`
	NeedConfirmationToRead bool            `json:"needConfirmationToRead"`
	Confetti               bool            `json:"confetti"`
	UserID                 jsonInt64       `json:"userId"`
	Active                 *bool           `json:"isActive"`
	StartsAt               *int64          `json:"startsAt"`
	EndsAt                 *int64          `json:"endsAt"`
}

type announcementStringPatch struct {
	Set   bool
	Value *string
}

func (patch *announcementStringPatch) UnmarshalJSON(data []byte) error {
	patch.Set = true
	if string(data) == "null" {
		patch.Value = nil
		return nil
	}
	var value string
	if err := json.Unmarshal(data, &value); err != nil {
		return err
	}
	patch.Value = &value
	return nil
}

type adminAnnouncementUpdateRequest struct {
	ID                     string                  `json:"id"`
	Title                  announcementStringPatch `json:"title"`
	Text                   announcementStringPatch `json:"text"`
	ImageURL               announcementStringPatch `json:"imageUrl"`
	Icon                   announcementStringPatch `json:"icon"`
	Display                announcementStringPatch `json:"display"`
	ForExistingUsers       *bool                   `json:"forExistingUsers"`
	ForRoles               *[]string               `json:"forRoles"`
	Silence                *bool                   `json:"silence"`
	NeedConfirmationToRead *bool                   `json:"needConfirmationToRead"`
	Confetti               *bool                   `json:"confetti"`
	Active                 *bool                   `json:"isActive"`
	StartsAt               *int64                  `json:"startsAt"`
	EndsAt                 *int64                  `json:"endsAt"`
}

type adminAnnouncementDeleteRequest struct {
	ID string `json:"id"`
}

type readAnnouncementRequest struct {
	AnnouncementID string `json:"announcementId"`
	LegacyID       string `json:"announcement_id"`
}

func (h *Handler) readAnnouncement(c *gin.Context) {
	var req readAnnouncementRequest
	if !bindJSON(c, &req) {
		return
	}
	id := strings.TrimSpace(req.AnnouncementID)
	if id == "" {
		id = strings.TrimSpace(req.LegacyID)
	}
	if id == "" {
		writeError(c, stdhttp.StatusBadRequest, "announcementId is required", "bad_request")
		return
	}
	ctx, cancel := rpcContext(c)
	defer cancel()
	if _, err := h.clients.Admin.MarkAnnouncementRead(ctx, &adminpb.ReadAnnouncementRequest{UserId: currentUserID(c), AnnouncementId: id}); err != nil {
		writeRPCError(c, err)
		return
	}
	c.Status(stdhttp.StatusNoContent)
}

func (h *Handler) listAdminAnnouncements(c *gin.Context) {
	var req adminAnnouncementListRequest
	if !bindJSON(c, &req) {
		return
	}
	if req.Limit == 0 {
		req.Limit = 10
	}
	if req.Limit < 1 || req.Limit > 100 || req.UserID.Int64() < 0 {
		writeError(c, stdhttp.StatusBadRequest, "invalid request body", "bad_request")
		return
	}
	ctx, cancel := rpcContext(c)
	defer cancel()
	resp, err := h.clients.Admin.ListAnnouncements(ctx, &adminpb.ListAnnouncementsRequest{
		Actor: currentActor(c), Limit: req.Limit, SinceId: strings.TrimSpace(req.SinceID), UntilId: strings.TrimSpace(req.UntilID), UserId: req.UserID.Int64(), Status: strings.TrimSpace(req.Status),
	})
	if err != nil {
		writeRPCError(c, err)
		return
	}
	items := make([]adminAnnouncementPayload, 0, len(resp.GetItems()))
	for _, item := range resp.GetItems() {
		items = append(items, adminAnnouncementFromProto(item, true))
	}
	c.JSON(stdhttp.StatusOK, items)
}

func (h *Handler) createAdminAnnouncement(c *gin.Context) {
	var req adminAnnouncementCreateRequest
	if !bindJSON(c, &req) {
		return
	}
	if strings.TrimSpace(req.Title) == "" || strings.TrimSpace(req.Text) == "" || req.ImageURL == nil {
		writeError(c, stdhttp.StatusBadRequest, "title, text and imageUrl are required", "bad_request")
		return
	}
	imageURL, err := nullableAnnouncementString(req.ImageURL)
	if err != nil {
		writeError(c, stdhttp.StatusBadRequest, "imageUrl must be a string or null", "bad_request")
		return
	}
	active := true
	if req.Active != nil {
		active = *req.Active
	}
	ctx, cancel := rpcContext(c)
	defer cancel()
	resp, err := h.clients.Admin.CreateAnnouncement(ctx, &adminpb.CreateAnnouncementRequest{
		Actor: currentActor(c), Title: req.Title, Text: req.Text, ImageUrl: imageURL, Icon: req.Icon, Display: req.Display,
		ForExistingUsers: req.ForExistingUsers, ForRoles: req.ForRoles, Silence: req.Silence,
		NeedConfirmationToRead: req.NeedConfirmationToRead, Confetti: req.Confetti, UserId: req.UserID.Int64(), Active: active,
		StartsAt: optionalInt64(req.StartsAt), EndsAt: optionalInt64(req.EndsAt),
	})
	if err != nil {
		writeRPCError(c, err)
		return
	}
	if resp.GetAnnouncement() == nil {
		writeError(c, stdhttp.StatusInternalServerError, "admin service returned an empty announcement", "internal_error")
		return
	}
	c.JSON(stdhttp.StatusOK, adminAnnouncementFromProto(resp.GetAnnouncement(), true))
}

func (h *Handler) updateAdminAnnouncement(c *gin.Context) {
	var req adminAnnouncementUpdateRequest
	if !bindJSON(c, &req) {
		return
	}
	if strings.TrimSpace(req.ID) == "" {
		writeError(c, stdhttp.StatusBadRequest, "id is required", "bad_request")
		return
	}
	protobuf := &adminpb.UpdateAnnouncementRequest{Actor: currentActor(c), Id: strings.TrimSpace(req.ID), ForExistingUsers: req.ForExistingUsers, Silence: req.Silence, NeedConfirmationToRead: req.NeedConfirmationToRead, Confetti: req.Confetti, Active: req.Active, StartsAt: req.StartsAt, EndsAt: req.EndsAt}
	if req.Title.Set {
		protobuf.Title = req.Title.Value
	}
	if req.Text.Set {
		protobuf.Text = req.Text.Value
	}
	if req.ImageURL.Set {
		value := ""
		if req.ImageURL.Value != nil {
			value = *req.ImageURL.Value
		}
		protobuf.ImageUrl = &value
	}
	if req.Icon.Set {
		protobuf.Icon = req.Icon.Value
	}
	if req.Display.Set {
		protobuf.Display = req.Display.Value
	}
	if req.ForRoles != nil {
		protobuf.ForRoles = &adminpb.StringListValue{Values: *req.ForRoles}
	}
	ctx, cancel := rpcContext(c)
	defer cancel()
	if _, err := h.clients.Admin.UpdateAnnouncement(ctx, protobuf); err != nil {
		writeRPCError(c, err)
		return
	}
	c.Status(stdhttp.StatusNoContent)
}

func (h *Handler) deleteAdminAnnouncement(c *gin.Context) {
	var req adminAnnouncementDeleteRequest
	if !bindJSON(c, &req) {
		return
	}
	if strings.TrimSpace(req.ID) == "" {
		writeError(c, stdhttp.StatusBadRequest, "id is required", "bad_request")
		return
	}
	ctx, cancel := rpcContext(c)
	defer cancel()
	if _, err := h.clients.Admin.DeleteAnnouncement(ctx, &adminpb.AnnouncementIDRequest{Actor: currentActor(c), Id: strings.TrimSpace(req.ID)}); err != nil {
		writeRPCError(c, err)
		return
	}
	c.Status(stdhttp.StatusNoContent)
}

func nullableAnnouncementString(raw json.RawMessage) (string, error) {
	if string(raw) == "null" {
		return "", nil
	}
	var value string
	if err := json.Unmarshal(raw, &value); err != nil {
		return "", err
	}
	return strings.TrimSpace(value), nil
}

func optionalInt64(value *int64) int64 {
	if value == nil {
		return 0
	}
	return *value
}

type adminAnnouncementPayload struct {
	ID                     string   `json:"id"`
	CreatedAt              string   `json:"createdAt"`
	UpdatedAt              *string  `json:"updatedAt"`
	Title                  string   `json:"title"`
	Text                   string   `json:"text"`
	ImageURL               *string  `json:"imageUrl"`
	Icon                   string   `json:"icon"`
	Display                string   `json:"display"`
	ForExistingUsers       bool     `json:"forExistingUsers"`
	ForRoles               []string `json:"forRoles"`
	Silence                bool     `json:"silence"`
	NeedConfirmationToRead bool     `json:"needConfirmationToRead"`
	Confetti               bool     `json:"confetti"`
	UserID                 *string  `json:"userId"`
	IsActive               bool     `json:"isActive"`
	StartsAt               *string  `json:"startsAt,omitempty"`
	EndsAt                 *string  `json:"endsAt,omitempty"`
	Reads                  int64    `json:"reads"`
	ForYou                 bool     `json:"forYou,omitempty"`
	IsRead                 *bool    `json:"isRead,omitempty"`
}

func adminAnnouncementFromProto(item *adminpb.AnnouncementInfo, includeRead bool) adminAnnouncementPayload {
	createdAt := formatUnixMilli(item.GetCreatedAt())
	updatedAt := formatUnixMilliPointer(item.GetUpdatedAt())
	result := adminAnnouncementPayload{
		ID: item.GetId(), CreatedAt: createdAt, UpdatedAt: updatedAt, Title: item.GetTitle(), Text: item.GetText(),
		Icon: item.GetIcon(), Display: item.GetDisplay(), ForExistingUsers: item.GetForExistingUsers(), ForRoles: item.GetForRoles(),
		Silence: item.GetSilence(), NeedConfirmationToRead: item.GetNeedConfirmationToRead(), Confetti: item.GetConfetti(), IsActive: item.GetActive(),
		Reads: item.GetReads(), ForYou: item.GetForYou(),
	}
	if item.GetImageUrl() != "" {
		imageURL := item.GetImageUrl()
		result.ImageURL = &imageURL
	}
	if item.GetUserId() > 0 {
		value := jsonInt64(item.GetUserId())
		text := strconv.FormatInt(value.Int64(), 10)
		result.UserID = &text
	}
	if item.GetStartsAt() > 0 {
		value := formatUnixMilli(item.GetStartsAt())
		result.StartsAt = &value
	}
	if item.GetEndsAt() > 0 {
		value := formatUnixMilli(item.GetEndsAt())
		result.EndsAt = &value
	}
	if includeRead {
		value := item.GetIsRead()
		result.IsRead = &value
	}
	return result
}

func formatUnixMilliPointer(value int64) *string {
	if value <= 0 {
		return nil
	}
	formatted := formatUnixMilli(value)
	return &formatted
}
