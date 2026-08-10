package admin

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"net/url"
	"strings"
	"time"

	domain "admin/internal/domain/admin"
)

const (
	maxAnnouncementTitleLength = 256
	maxAnnouncementTextLength  = 20000
)

func (s *Service) ListAnnouncements(ctx context.Context, actor domain.Actor, filter domain.AnnouncementListFilter) (domain.AnnouncementList, error) {
	if err := actor.Validate(); err != nil {
		return domain.AnnouncementList{}, err
	}
	if err := s.auth.Authorize(ctx, actor, domain.ActionListAnnouncements); err != nil {
		return domain.AnnouncementList{}, err
	}
	filter.Status = strings.ToLower(strings.TrimSpace(filter.Status))
	if filter.Status == "" {
		filter.Status = "active"
	}
	if filter.Status != "all" && filter.Status != "active" && filter.Status != "archived" {
		return domain.AnnouncementList{}, domain.ErrInvalidStatus
	}
	if filter.Limit <= 0 || filter.Limit > 100 || filter.UserID < 0 {
		return domain.AnnouncementList{}, domain.ErrInvalidAnnouncement
	}
	return s.ops.ListAnnouncements(ctx, filter)
}

func (s *Service) ListPublicAnnouncements(ctx context.Context, userID int64, userCreatedAt int64, filter domain.PublicAnnouncementListFilter) (domain.AnnouncementList, error) {
	if userID < 0 || userCreatedAt < 0 {
		return domain.AnnouncementList{}, domain.ErrInvalidUserID
	}
	if filter.Limit <= 0 || filter.Limit > 100 {
		return domain.AnnouncementList{}, domain.ErrInvalidAnnouncement
	}
	return s.ops.ListPublicAnnouncements(ctx, userID, userCreatedAt, filter, time.Now().UnixMilli())
}

func (s *Service) GetPublicAnnouncement(ctx context.Context, userID int64, userCreatedAt int64, id string) (domain.Announcement, error) {
	id = strings.TrimSpace(id)
	if userID < 0 || userCreatedAt < 0 {
		return domain.Announcement{}, domain.ErrInvalidUserID
	}
	if id == "" {
		return domain.Announcement{}, domain.ErrInvalidAnnouncementID
	}
	return s.ops.GetPublicAnnouncement(ctx, userID, userCreatedAt, id, time.Now().UnixMilli())
}

func (s *Service) CreateAnnouncement(ctx context.Context, actor domain.Actor, command domain.CreateAnnouncementCommand) (domain.Announcement, error) {
	if err := actor.Validate(); err != nil {
		return domain.Announcement{}, err
	}
	if err := s.auth.Authorize(ctx, actor, domain.ActionCreateAnnouncement); err != nil {
		return domain.Announcement{}, err
	}
	command, err := normalizeCreateAnnouncementCommand(command)
	if err != nil {
		return domain.Announcement{}, err
	}
	id, err := newAnnouncementID()
	if err != nil {
		return domain.Announcement{}, err
	}
	return s.ops.CreateAnnouncement(ctx, id, command, time.Now().UnixMilli())
}

func (s *Service) UpdateAnnouncement(ctx context.Context, actor domain.Actor, command domain.UpdateAnnouncementCommand) (domain.Announcement, error) {
	if err := actor.Validate(); err != nil {
		return domain.Announcement{}, err
	}
	command.ID = strings.TrimSpace(command.ID)
	if command.ID == "" {
		return domain.Announcement{}, domain.ErrInvalidAnnouncementID
	}
	if err := s.auth.Authorize(ctx, actor, domain.ActionUpdateAnnouncement); err != nil {
		return domain.Announcement{}, err
	}
	command, err := normalizeUpdateAnnouncementCommand(command)
	if err != nil {
		return domain.Announcement{}, err
	}
	return s.ops.UpdateAnnouncement(ctx, command, time.Now().UnixMilli())
}

func (s *Service) DeleteAnnouncement(ctx context.Context, actor domain.Actor, id string) error {
	if err := actor.Validate(); err != nil {
		return err
	}
	id = strings.TrimSpace(id)
	if id == "" {
		return domain.ErrInvalidAnnouncementID
	}
	if err := s.auth.Authorize(ctx, actor, domain.ActionDeleteAnnouncement); err != nil {
		return err
	}
	return s.ops.DeleteAnnouncement(ctx, id)
}

func (s *Service) MarkAnnouncementRead(ctx context.Context, userID int64, announcementID string) error {
	announcementID = strings.TrimSpace(announcementID)
	if userID <= 0 {
		return domain.ErrInvalidUserID
	}
	if announcementID == "" {
		return domain.ErrInvalidAnnouncementID
	}
	return s.ops.MarkAnnouncementRead(ctx, userID, announcementID, time.Now().UnixMilli())
}

func normalizeCreateAnnouncementCommand(command domain.CreateAnnouncementCommand) (domain.CreateAnnouncementCommand, error) {
	command.Title = strings.TrimSpace(command.Title)
	command.Text = strings.TrimSpace(command.Text)
	command.ImageURL = strings.TrimSpace(command.ImageURL)
	icon := strings.ToLower(strings.TrimSpace(command.Icon))
	if icon == "" {
		icon = "info"
	}
	if icon != "info" && icon != "warning" && icon != "error" && icon != "success" {
		return domain.CreateAnnouncementCommand{}, domain.ErrInvalidAnnouncement
	}
	command.Icon = icon
	display := strings.ToLower(strings.TrimSpace(command.Display))
	if display == "" {
		display = "normal"
	}
	if display != "normal" && display != "banner" && display != "dialog" {
		return domain.CreateAnnouncementCommand{}, domain.ErrInvalidAnnouncement
	}
	command.Display = display
	command.ForRoles = normalizeAnnouncementRoles(command.ForRoles)
	if !validAnnouncementContent(command.Title, command.Text) || !validAnnouncementURL(command.ImageURL) || command.UserID < 0 || !validAnnouncementSchedule(command.StartsAt, command.EndsAt) {
		return domain.CreateAnnouncementCommand{}, domain.ErrInvalidAnnouncement
	}
	return command, nil
}

func normalizeUpdateAnnouncementCommand(command domain.UpdateAnnouncementCommand) (domain.UpdateAnnouncementCommand, error) {
	if command.Title != nil {
		value := strings.TrimSpace(*command.Title)
		if value == "" || len([]rune(value)) > maxAnnouncementTitleLength {
			return domain.UpdateAnnouncementCommand{}, domain.ErrInvalidAnnouncement
		}
		command.Title = &value
	}
	if command.Text != nil {
		value := strings.TrimSpace(*command.Text)
		if value == "" || len([]rune(value)) > maxAnnouncementTextLength {
			return domain.UpdateAnnouncementCommand{}, domain.ErrInvalidAnnouncement
		}
		command.Text = &value
	}
	if command.ImageURL != nil {
		value := strings.TrimSpace(*command.ImageURL)
		if !validAnnouncementURL(value) {
			return domain.UpdateAnnouncementCommand{}, domain.ErrInvalidAnnouncement
		}
		command.ImageURL = &value
	}
	if command.Icon != nil {
		value := strings.ToLower(strings.TrimSpace(*command.Icon))
		if value != "info" && value != "warning" && value != "error" && value != "success" {
			return domain.UpdateAnnouncementCommand{}, domain.ErrInvalidAnnouncement
		}
		command.Icon = &value
	}
	if command.Display != nil {
		value := strings.ToLower(strings.TrimSpace(*command.Display))
		if value != "normal" && value != "banner" && value != "dialog" {
			return domain.UpdateAnnouncementCommand{}, domain.ErrInvalidAnnouncement
		}
		command.Display = &value
	}
	if command.ForRolesSet {
		command.ForRoles = normalizeAnnouncementRoles(command.ForRoles)
	}
	if command.StartsAt != nil && *command.StartsAt < 0 || command.EndsAt != nil && *command.EndsAt < 0 {
		return domain.UpdateAnnouncementCommand{}, domain.ErrInvalidAnnouncement
	}
	return command, nil
}

func validAnnouncementContent(title string, text string) bool {
	return title != "" && text != "" && len([]rune(title)) <= maxAnnouncementTitleLength && len([]rune(text)) <= maxAnnouncementTextLength
}

func validAnnouncementURL(value string) bool {
	if value == "" {
		return true
	}
	parsed, err := url.ParseRequestURI(value)
	return err == nil && (parsed.Scheme == "http" || parsed.Scheme == "https") && parsed.Host != "" && parsed.User == nil
}

func validAnnouncementSchedule(startsAt int64, endsAt int64) bool {
	return startsAt >= 0 && endsAt >= 0 && (startsAt == 0 || endsAt == 0 || endsAt > startsAt)
}

func normalizeAnnouncementRoles(values []string) []string {
	result := make([]string, 0, len(values))
	seen := make(map[string]struct{}, len(values))
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" {
			continue
		}
		if _, ok := seen[value]; ok {
			continue
		}
		seen[value] = struct{}{}
		result = append(result, value)
	}
	return result
}

func newAnnouncementID() (string, error) {
	bytes := make([]byte, 12)
	if _, err := rand.Read(bytes); err != nil {
		return "", err
	}
	return hex.EncodeToString(bytes), nil
}
