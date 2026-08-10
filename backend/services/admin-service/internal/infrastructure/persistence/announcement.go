package persistence

import (
	"context"
	"encoding/json"
	"errors"
	"strconv"
	"strings"
	"time"

	domain "admin/internal/domain/admin"
	"admin/internal/infrastructure/persistence/po"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

const announcementSettingKey = "site_announcements"
const maxActiveDialogAnnouncements = 5

type announcementStorageInput struct {
	ID                     string          `json:"id"`
	Title                  string          `json:"title"`
	Text                   string          `json:"text"`
	Content                string          `json:"content"`
	ImageURL               string          `json:"image_url"`
	ImageURLJSON           string          `json:"imageUrl"`
	Icon                   string          `json:"icon"`
	Display                string          `json:"display"`
	ForExistingUsers       bool            `json:"for_existing_users"`
	ForExistingUsersJSON   bool            `json:"forExistingUsers"`
	ForRoles               []string        `json:"for_roles"`
	ForRolesJSON           []string        `json:"forRoles"`
	Silence                bool            `json:"silence"`
	NeedConfirmationToRead bool            `json:"need_confirmation_to_read"`
	NeedConfirmationJSON   bool            `json:"needConfirmationToRead"`
	Confetti               bool            `json:"confetti"`
	UserID                 json.RawMessage `json:"user_id"`
	UserIDJSON             json.RawMessage `json:"userId"`
	Active                 *bool           `json:"active"`
	IsActive               *bool           `json:"isActive"`
	StartsAt               int64           `json:"starts_at"`
	StartsAtJSON           int64           `json:"startsAt"`
	EndsAt                 int64           `json:"ends_at"`
	EndsAtJSON             int64           `json:"endsAt"`
	CreatedAt              int64           `json:"created_at"`
	CreatedAtJSON          int64           `json:"createdAt"`
	UpdatedAt              int64           `json:"updated_at"`
	UpdatedAtJSON          int64           `json:"updatedAt"`
}

type announcementStorageOutput struct {
	ID                     string   `json:"id"`
	Title                  string   `json:"title"`
	Text                   string   `json:"text"`
	ImageURL               string   `json:"image_url,omitempty"`
	Icon                   string   `json:"icon"`
	Display                string   `json:"display"`
	ForExistingUsers       bool     `json:"for_existing_users"`
	ForRoles               []string `json:"for_roles"`
	Silence                bool     `json:"silence"`
	NeedConfirmationToRead bool     `json:"need_confirmation_to_read"`
	Confetti               bool     `json:"confetti"`
	UserID                 int64    `json:"user_id,omitempty"`
	Active                 bool     `json:"active"`
	StartsAt               int64    `json:"starts_at,omitempty"`
	EndsAt                 int64    `json:"ends_at,omitempty"`
	CreatedAt              int64    `json:"created_at"`
	UpdatedAt              int64    `json:"updated_at"`
}

func (r *Repository) ListAnnouncements(ctx context.Context, filter domain.AnnouncementListFilter) (domain.AnnouncementList, error) {
	items, err := r.loadAnnouncements(ctx, r.db.WithContext(ctx), false)
	if err != nil {
		return domain.AnnouncementList{}, err
	}
	filtered := make([]domain.Announcement, 0, len(items))
	for _, item := range items {
		if filter.Status == "active" && !item.Active || filter.Status == "archived" && item.Active {
			continue
		}
		if filter.UserID > 0 && item.UserID != filter.UserID {
			continue
		}
		filtered = append(filtered, item)
	}
	total := int64(len(filtered))
	filtered = announcementCursorSlice(filtered, filter.SinceID, filter.UntilID)
	if len(filtered) > int(filter.Limit) {
		filtered = filtered[:filter.Limit]
	}
	if err := r.attachAnnouncementReadState(ctx, filtered, 0); err != nil {
		return domain.AnnouncementList{}, err
	}
	return domain.AnnouncementList{Items: filtered, Total: total}, nil
}

func (r *Repository) ListPublicAnnouncements(ctx context.Context, userID int64, userCreatedAt int64, filter domain.PublicAnnouncementListFilter, now int64) (domain.AnnouncementList, error) {
	items, err := r.loadAnnouncements(ctx, r.db.WithContext(ctx), false)
	if err != nil {
		return domain.AnnouncementList{}, err
	}
	visible := make([]domain.Announcement, 0, len(items))
	for _, item := range items {
		if !publicAnnouncementVisible(item, userID, filter.Active, now) {
			continue
		}
		item.ForYou = item.UserID > 0 && item.UserID == userID
		visible = append(visible, item)
	}
	visible = announcementCursorSlice(visible, filter.SinceID, filter.UntilID)
	if len(visible) > int(filter.Limit) {
		visible = visible[:filter.Limit]
	}
	if err := r.attachAnnouncementReadState(ctx, visible, userID); err != nil {
		return domain.AnnouncementList{}, err
	}
	if userID > 0 && userCreatedAt > 0 {
		for index := range visible {
			if visible[index].ForExistingUsers && visible[index].CreatedAt > 0 && userCreatedAt > visible[index].CreatedAt {
				visible[index].IsRead = true
			}
		}
	}
	return domain.AnnouncementList{Items: visible, Total: int64(len(visible))}, nil
}

func (r *Repository) GetPublicAnnouncement(ctx context.Context, userID int64, userCreatedAt int64, id string, now int64) (domain.Announcement, error) {
	items, err := r.loadAnnouncements(ctx, r.db.WithContext(ctx), false)
	if err != nil {
		return domain.Announcement{}, err
	}
	index := announcementIndex(items, id)
	active := true
	if index < 0 || !publicAnnouncementVisible(items[index], userID, &active, now) {
		return domain.Announcement{}, domain.ErrInvalidAnnouncementID
	}
	result := []domain.Announcement{items[index]}
	result[0].ForYou = result[0].UserID > 0 && result[0].UserID == userID
	if err := r.attachAnnouncementReadState(ctx, result, userID); err != nil {
		return domain.Announcement{}, err
	}
	if userID > 0 && userCreatedAt > 0 && result[0].ForExistingUsers && result[0].CreatedAt > 0 && userCreatedAt > result[0].CreatedAt {
		result[0].IsRead = true
	}
	return result[0], nil
}

func publicAnnouncementVisible(item domain.Announcement, userID int64, active *bool, now int64) bool {
	currentlyActive := item.Active && (item.StartsAt == 0 || now >= item.StartsAt) && (item.EndsAt == 0 || now <= item.EndsAt)
	if active == nil && !currentlyActive || active != nil && currentlyActive != *active {
		return false
	}
	if item.UserID > 0 && item.UserID != userID {
		return false
	}
	// End-user roles are not yet represented in BBS. Targeted role announcements
	// stay hidden rather than leaking to a broader audience.
	return len(item.ForRoles) == 0
}

func (r *Repository) CreateAnnouncement(ctx context.Context, id string, command domain.CreateAnnouncementCommand, now int64) (domain.Announcement, error) {
	var created domain.Announcement
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		items, err := r.loadAnnouncements(ctx, tx, true)
		if err != nil {
			return err
		}
		for _, item := range items {
			if item.ID == id {
				return domain.ErrInvalidAnnouncementID
			}
		}
		created = domain.Announcement{
			ID: id, Title: command.Title, Text: command.Text, ImageURL: command.ImageURL,
			Icon: command.Icon, Display: command.Display, ForExistingUsers: command.ForExistingUsers,
			ForRoles: append([]string(nil), command.ForRoles...), Silence: command.Silence,
			NeedConfirmationToRead: command.NeedConfirmationToRead, Confetti: command.Confetti,
			UserID: command.UserID, Active: command.Active, StartsAt: command.StartsAt,
			EndsAt: command.EndsAt, CreatedAt: now, UpdatedAt: now,
		}
		if created.Active && created.Display == "dialog" && activeDialogAnnouncementCount(items, "") >= maxActiveDialogAnnouncements {
			return domain.ErrAnnouncementDialogLimit
		}
		items = append([]domain.Announcement{created}, items...)
		return r.saveAnnouncements(ctx, tx, items, now)
	})
	return created, err
}

func (r *Repository) UpdateAnnouncement(ctx context.Context, command domain.UpdateAnnouncementCommand, now int64) (domain.Announcement, error) {
	var updated domain.Announcement
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		items, err := r.loadAnnouncements(ctx, tx, true)
		if err != nil {
			return err
		}
		index := announcementIndex(items, command.ID)
		if index < 0 {
			return domain.ErrInvalidAnnouncementID
		}
		updated = items[index]
		applyAnnouncementUpdate(&updated, command)
		if strings.TrimSpace(updated.Title) == "" || strings.TrimSpace(updated.Text) == "" || !validStoredAnnouncementSchedule(updated.StartsAt, updated.EndsAt) {
			return domain.ErrInvalidAnnouncement
		}
		if updated.Active && updated.Display == "dialog" && activeDialogAnnouncementCount(items, updated.ID) >= maxActiveDialogAnnouncements {
			return domain.ErrAnnouncementDialogLimit
		}
		updated.UpdatedAt = now
		items[index] = updated
		return r.saveAnnouncements(ctx, tx, items, now)
	})
	return updated, err
}

func (r *Repository) DeleteAnnouncement(ctx context.Context, id string) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		items, err := r.loadAnnouncements(ctx, tx, true)
		if err != nil {
			return err
		}
		index := announcementIndex(items, id)
		if index < 0 {
			return domain.ErrInvalidAnnouncementID
		}
		items = append(items[:index], items[index+1:]...)
		if err := r.saveAnnouncements(ctx, tx, items, time.Now().UnixMilli()); err != nil {
			return err
		}
		return tx.WithContext(ctx).Where("announcement_id = ?", id).Delete(&po.AnnouncementRead{}).Error
	})
}

func (r *Repository) MarkAnnouncementRead(ctx context.Context, userID int64, announcementID string, now int64) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		items, err := r.loadAnnouncements(ctx, tx, true)
		if err != nil {
			return err
		}
		index := announcementIndex(items, announcementID)
		if index < 0 {
			return nil
		}
		read := po.AnnouncementRead{
			UserID: userID, AnnouncementID: announcementID,
			AnnouncementUpdatedAt: items[index].UpdatedAt, ReadAt: time.UnixMilli(now),
		}
		if err := tx.WithContext(ctx).Clauses(clause.OnConflict{
			Columns: []clause.Column{{Name: "user_id"}, {Name: "announcement_id"}},
			DoUpdates: clause.Assignments(map[string]any{
				"announcement_updated_at": read.AnnouncementUpdatedAt,
				"read_at":                 read.ReadAt,
			}),
		}).Create(&read).Error; err != nil {
			return err
		}
		if items[index].UserID == userID && items[index].Active {
			items[index].Active = false
			items[index].UpdatedAt = now
			return r.saveAnnouncements(ctx, tx, items, now)
		}
		return nil
	})
}

func (r *Repository) loadAnnouncements(ctx context.Context, db *gorm.DB, lock bool) ([]domain.Announcement, error) {
	query := db.WithContext(ctx)
	if lock {
		query = query.Clauses(clause.Locking{Strength: "UPDATE"})
	}
	var setting po.SiteSetting
	if err := query.Where("key = ?", announcementSettingKey).First(&setting).Error; err != nil {
		if !errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, err
		}
		if !lock {
			return []domain.Announcement{}, nil
		}
		setting = po.SiteSetting{
			Key: announcementSettingKey, Value: "[]", Group: "site", ValueType: "json",
			Description: "C 端公开公告 JSON；由公告管理功能维护。", Status: 2,
		}
		if err := db.WithContext(ctx).Create(&setting).Error; err != nil {
			return nil, err
		}
	}
	return decodeAnnouncements(setting.Value)
}

func (r *Repository) saveAnnouncements(ctx context.Context, tx *gorm.DB, items []domain.Announcement, now int64) error {
	stored := make([]announcementStorageOutput, 0, len(items))
	for _, item := range items {
		stored = append(stored, announcementStorageOutput{
			ID: item.ID, Title: item.Title, Text: item.Text, ImageURL: item.ImageURL,
			Icon: item.Icon, Display: item.Display, ForExistingUsers: item.ForExistingUsers,
			ForRoles: item.ForRoles, Silence: item.Silence, NeedConfirmationToRead: item.NeedConfirmationToRead,
			Confetti: item.Confetti, UserID: item.UserID, Active: item.Active,
			StartsAt: item.StartsAt, EndsAt: item.EndsAt, CreatedAt: item.CreatedAt, UpdatedAt: item.UpdatedAt,
		})
	}
	value, err := json.Marshal(stored)
	if err != nil {
		return err
	}
	result := tx.WithContext(ctx).Model(&po.SiteSetting{}).Where("key = ?", announcementSettingKey).Updates(map[string]any{
		"value": string(value), "value_type": "json", "setting_group": "site", "status": 2,
		"updated_at": time.UnixMilli(now),
	})
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return domain.ErrInvalidSettingID
	}
	return nil
}

func decodeAnnouncements(raw string) ([]domain.Announcement, error) {
	if strings.TrimSpace(raw) == "" {
		raw = "[]"
	}
	var input []announcementStorageInput
	if err := json.Unmarshal([]byte(raw), &input); err != nil {
		return nil, domain.ErrInvalidAnnouncement
	}
	items := make([]domain.Announcement, 0, len(input))
	seen := make(map[string]struct{}, len(input))
	for _, stored := range input {
		item, err := decodeAnnouncement(stored)
		if err != nil {
			return nil, err
		}
		if _, ok := seen[item.ID]; ok {
			return nil, domain.ErrInvalidAnnouncementID
		}
		seen[item.ID] = struct{}{}
		items = append(items, item)
	}
	return items, nil
}

func decodeAnnouncement(stored announcementStorageInput) (domain.Announcement, error) {
	item := domain.Announcement{
		ID: strings.TrimSpace(stored.ID), Title: strings.TrimSpace(stored.Title),
		Text:     strings.TrimSpace(firstStoredString(stored.Text, stored.Content)),
		ImageURL: strings.TrimSpace(firstStoredString(stored.ImageURL, stored.ImageURLJSON)),
		Icon:     normalizeStoredAnnouncementIcon(stored.Icon), Display: normalizeStoredAnnouncementDisplay(stored.Display),
		ForExistingUsers: stored.ForExistingUsers || stored.ForExistingUsersJSON,
		ForRoles:         append([]string(nil), firstStoredStrings(stored.ForRoles, stored.ForRolesJSON)...),
		Silence:          stored.Silence, NeedConfirmationToRead: stored.NeedConfirmationToRead || stored.NeedConfirmationJSON,
		Confetti: stored.Confetti, Active: true,
		StartsAt: firstStoredInt64(stored.StartsAt, stored.StartsAtJSON), EndsAt: firstStoredInt64(stored.EndsAt, stored.EndsAtJSON),
		CreatedAt: firstStoredInt64(stored.CreatedAt, stored.CreatedAtJSON), UpdatedAt: firstStoredInt64(stored.UpdatedAt, stored.UpdatedAtJSON),
	}
	if stored.Active != nil {
		item.Active = *stored.Active
	} else if stored.IsActive != nil {
		item.Active = *stored.IsActive
	}
	item.UserID = firstStoredJSONInt64(stored.UserID, stored.UserIDJSON)
	if item.ID == "" || item.Title == "" || item.Text == "" || item.UserID < 0 || !validStoredAnnouncementSchedule(item.StartsAt, item.EndsAt) {
		return domain.Announcement{}, domain.ErrInvalidAnnouncement
	}
	return item, nil
}

func (r *Repository) attachAnnouncementReadState(ctx context.Context, items []domain.Announcement, userID int64) error {
	if len(items) == 0 {
		return nil
	}
	ids := make([]string, 0, len(items))
	for _, item := range items {
		ids = append(ids, item.ID)
	}
	type readCount struct {
		AnnouncementID string
		Count          int64
	}
	var counts []readCount
	if err := r.db.WithContext(ctx).Model(&po.AnnouncementRead{}).
		Select("announcement_id, COUNT(*) AS count").Where("announcement_id IN ?", ids).
		Group("announcement_id").Scan(&counts).Error; err != nil {
		return err
	}
	countByID := make(map[string]int64, len(counts))
	for _, count := range counts {
		countByID[count.AnnouncementID] = count.Count
	}
	readByID := map[string]int64{}
	if userID > 0 {
		var reads []po.AnnouncementRead
		if err := r.db.WithContext(ctx).Where("user_id = ? AND announcement_id IN ?", userID, ids).Find(&reads).Error; err != nil {
			return err
		}
		for _, read := range reads {
			readByID[read.AnnouncementID] = read.AnnouncementUpdatedAt
		}
	}
	for index := range items {
		items[index].Reads = countByID[items[index].ID]
		_, items[index].IsRead = readByID[items[index].ID]
	}
	return nil
}

func applyAnnouncementUpdate(item *domain.Announcement, command domain.UpdateAnnouncementCommand) {
	if command.Title != nil {
		item.Title = *command.Title
	}
	if command.Text != nil {
		item.Text = *command.Text
	}
	if command.ImageURL != nil {
		item.ImageURL = *command.ImageURL
	}
	if command.Icon != nil {
		item.Icon = *command.Icon
	}
	if command.Display != nil {
		item.Display = *command.Display
	}
	if command.ForExistingUsers != nil {
		item.ForExistingUsers = *command.ForExistingUsers
	}
	if command.ForRolesSet {
		item.ForRoles = append([]string(nil), command.ForRoles...)
	}
	if command.Silence != nil {
		item.Silence = *command.Silence
	}
	if command.NeedConfirmationToRead != nil {
		item.NeedConfirmationToRead = *command.NeedConfirmationToRead
	}
	if command.Confetti != nil {
		item.Confetti = *command.Confetti
	}
	if command.Active != nil {
		item.Active = *command.Active
	}
	if command.StartsAt != nil {
		item.StartsAt = *command.StartsAt
	}
	if command.EndsAt != nil {
		item.EndsAt = *command.EndsAt
	}
}

func announcementCursorSlice(items []domain.Announcement, sinceID string, untilID string) []domain.Announcement {
	start, end := 0, len(items)
	if untilID = strings.TrimSpace(untilID); untilID != "" {
		if index := announcementIndex(items, untilID); index >= 0 {
			start = index + 1
		}
	}
	if sinceID = strings.TrimSpace(sinceID); sinceID != "" {
		if index := announcementIndex(items, sinceID); index >= 0 {
			end = index
		}
	}
	if start > end {
		return []domain.Announcement{}
	}
	return items[start:end]
}

func announcementIndex(items []domain.Announcement, id string) int {
	for index := range items {
		if items[index].ID == id {
			return index
		}
	}
	return -1
}

func activeDialogAnnouncementCount(items []domain.Announcement, excludeID string) int {
	count := 0
	for _, item := range items {
		if item.ID != excludeID && item.Active && item.Display == "dialog" {
			count++
		}
	}
	return count
}

func validStoredAnnouncementSchedule(startsAt int64, endsAt int64) bool {
	return startsAt >= 0 && endsAt >= 0 && (startsAt == 0 || endsAt == 0 || endsAt > startsAt)
}

func firstStoredString(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return value
		}
	}
	return ""
}

func firstStoredStrings(values ...[]string) []string {
	for _, value := range values {
		if len(value) > 0 {
			return value
		}
	}
	return []string{}
}

func firstStoredInt64(values ...int64) int64 {
	for _, value := range values {
		if value > 0 {
			return value
		}
	}
	return 0
}

func firstStoredJSONInt64(values ...json.RawMessage) int64 {
	for _, raw := range values {
		if len(raw) == 0 || string(raw) == "null" {
			continue
		}
		var number int64
		if json.Unmarshal(raw, &number) == nil {
			return number
		}
		var text string
		if json.Unmarshal(raw, &text) == nil {
			number, _ = strconv.ParseInt(strings.TrimSpace(text), 10, 64)
			return number
		}
	}
	return 0
}

func normalizeStoredAnnouncementIcon(value string) string {
	value = strings.ToLower(strings.TrimSpace(value))
	if value == "warning" || value == "error" || value == "success" {
		return value
	}
	return "info"
}

func normalizeStoredAnnouncementDisplay(value string) string {
	value = strings.ToLower(strings.TrimSpace(value))
	if value == "banner" || value == "dialog" {
		return value
	}
	return "normal"
}
