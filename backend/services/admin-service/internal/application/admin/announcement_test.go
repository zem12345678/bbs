package admin

import (
	"context"
	"errors"
	"testing"
	"time"

	domain "admin/internal/domain/admin"
)

func TestNormalizeCreateAnnouncementCommand(t *testing.T) {
	got, err := normalizeCreateAnnouncementCommand(domain.CreateAnnouncementCommand{
		Title: "  发布通知  ", Text: "  正文  ", ImageURL: " https://cdn.example.com/a.png ",
		Icon: " WARNING ", Display: " BANNER ", ForRoles: []string{" member ", "member", ""},
		StartsAt: 1000, EndsAt: 2000,
	})
	if err != nil {
		t.Fatalf("normalizeCreateAnnouncementCommand() error = %v", err)
	}
	if got.Title != "发布通知" || got.Text != "正文" || got.ImageURL != "https://cdn.example.com/a.png" || got.Icon != "warning" || got.Display != "banner" {
		t.Fatalf("normalized announcement = %#v", got)
	}
	if len(got.ForRoles) != 1 || got.ForRoles[0] != "member" {
		t.Fatalf("normalized roles = %v", got.ForRoles)
	}
}

func TestNormalizeCreateAnnouncementCommandRejectsInvalidValues(t *testing.T) {
	tests := []struct {
		name   string
		mutate func(*domain.CreateAnnouncementCommand)
	}{
		{name: "missing title", mutate: func(command *domain.CreateAnnouncementCommand) { command.Title = "" }},
		{name: "missing text", mutate: func(command *domain.CreateAnnouncementCommand) { command.Text = "" }},
		{name: "unsafe image", mutate: func(command *domain.CreateAnnouncementCommand) { command.ImageURL = "javascript:alert(1)" }},
		{name: "url credentials", mutate: func(command *domain.CreateAnnouncementCommand) {
			command.ImageURL = "https://user:pass@example.com/a.png"
		}},
		{name: "invalid icon", mutate: func(command *domain.CreateAnnouncementCommand) { command.Icon = "urgent" }},
		{name: "invalid display", mutate: func(command *domain.CreateAnnouncementCommand) { command.Display = "toast" }},
		{name: "invalid schedule", mutate: func(command *domain.CreateAnnouncementCommand) { command.StartsAt = 2000; command.EndsAt = 1000 }},
		{name: "negative user", mutate: func(command *domain.CreateAnnouncementCommand) { command.UserID = -1 }},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			command := validAnnouncementCommand()
			tt.mutate(&command)
			if _, err := normalizeCreateAnnouncementCommand(command); !errors.Is(err, domain.ErrInvalidAnnouncement) {
				t.Fatalf("normalizeCreateAnnouncementCommand() error = %v, want ErrInvalidAnnouncement", err)
			}
		})
	}
}

func TestAnnouncementManagementAuthorizesDedicatedActions(t *testing.T) {
	actor := domain.Actor{ID: 1, Username: "admin"}
	tests := []struct {
		name   string
		action domain.Action
		run    func(*Service) error
	}{
		{name: "list", action: domain.ActionListAnnouncements, run: func(service *Service) error {
			_, err := service.ListAnnouncements(context.Background(), actor, domain.AnnouncementListFilter{Limit: 10, Status: "all"})
			return err
		}},
		{name: "create", action: domain.ActionCreateAnnouncement, run: func(service *Service) error {
			_, err := service.CreateAnnouncement(context.Background(), actor, validAnnouncementCommand())
			return err
		}},
		{name: "update", action: domain.ActionUpdateAnnouncement, run: func(service *Service) error {
			_, err := service.UpdateAnnouncement(context.Background(), actor, domain.UpdateAnnouncementCommand{ID: "announcement"})
			return err
		}},
		{name: "delete", action: domain.ActionDeleteAnnouncement, run: func(service *Service) error {
			return service.DeleteAnnouncement(context.Background(), actor, "announcement")
		}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			authorizer := &announcementAuthorizerStub{err: domain.ErrPermissionDenied}
			service := &Service{auth: authorizer, ops: &announcementStoreStub{}}
			if err := tt.run(service); !errors.Is(err, domain.ErrPermissionDenied) {
				t.Fatalf("method error = %v, want ErrPermissionDenied", err)
			}
			if len(authorizer.actions) != 1 || authorizer.actions[0] != tt.action {
				t.Fatalf("authorized actions = %v, want [%s]", authorizer.actions, tt.action)
			}
		})
	}
}

func TestPublicAnnouncementsAndReadDoNotRequireAdminAuthorization(t *testing.T) {
	authorizer := &announcementAuthorizerStub{err: domain.ErrPermissionDenied}
	store := &announcementStoreStub{}
	service := &Service{auth: authorizer, ops: store}

	if _, err := service.ListPublicAnnouncements(context.Background(), 7, 1000, domain.PublicAnnouncementListFilter{Limit: 20}); err != nil {
		t.Fatalf("ListPublicAnnouncements() error = %v", err)
	}
	if err := service.MarkAnnouncementRead(context.Background(), 7, "announcement"); err != nil {
		t.Fatalf("MarkAnnouncementRead() error = %v", err)
	}
	if len(authorizer.actions) != 0 {
		t.Fatalf("public methods authorized actions = %v, want none", authorizer.actions)
	}
	if store.readUserID != 7 || store.readAnnouncementID != "announcement" {
		t.Fatalf("read call = user %d announcement %q", store.readUserID, store.readAnnouncementID)
	}
}

func TestGenericSettingUpdateCannotOverwriteManagedAnnouncements(t *testing.T) {
	service := &Service{auth: &announcementAuthorizerStub{}, ops: &announcementStoreStub{}}
	_, err := service.UpdateSetting(context.Background(), domain.Actor{ID: 1, Username: "admin"}, domain.UpsertSettingCommand{
		Key: "site_announcements", Value: "[]", Group: "site", ValueType: "json", Status: 2,
	})
	if !errors.Is(err, domain.ErrAnnouncementsManaged) {
		t.Fatalf("UpdateSetting(site_announcements) error = %v, want ErrAnnouncementsManaged", err)
	}
}

func validAnnouncementCommand() domain.CreateAnnouncementCommand {
	return domain.CreateAnnouncementCommand{
		Title: "发布通知", Text: "正文", Icon: "info", Display: "normal", Active: true,
		StartsAt: time.Now().Add(-time.Hour).UnixMilli(), EndsAt: time.Now().Add(time.Hour).UnixMilli(),
	}
}

type announcementAuthorizerStub struct {
	actions []domain.Action
	err     error
}

func (a *announcementAuthorizerStub) Authorize(_ context.Context, _ domain.Actor, action domain.Action) error {
	a.actions = append(a.actions, action)
	return a.err
}

func (a *announcementAuthorizerStub) Reload(context.Context) error { return nil }

type announcementStoreStub struct {
	OperationStore
	readUserID         int64
	readAnnouncementID string
}

func (s *announcementStoreStub) ListAnnouncements(context.Context, domain.AnnouncementListFilter) (domain.AnnouncementList, error) {
	return domain.AnnouncementList{}, nil
}

func (s *announcementStoreStub) ListPublicAnnouncements(context.Context, int64, int64, domain.PublicAnnouncementListFilter, int64) (domain.AnnouncementList, error) {
	return domain.AnnouncementList{}, nil
}

func (s *announcementStoreStub) GetPublicAnnouncement(context.Context, int64, int64, string, int64) (domain.Announcement, error) {
	return domain.Announcement{}, nil
}

func (s *announcementStoreStub) CreateAnnouncement(_ context.Context, id string, _ domain.CreateAnnouncementCommand, _ int64) (domain.Announcement, error) {
	return domain.Announcement{ID: id}, nil
}

func (s *announcementStoreStub) UpdateAnnouncement(_ context.Context, command domain.UpdateAnnouncementCommand, _ int64) (domain.Announcement, error) {
	return domain.Announcement{ID: command.ID}, nil
}

func (s *announcementStoreStub) DeleteAnnouncement(context.Context, string) error { return nil }

func (s *announcementStoreStub) MarkAnnouncementRead(_ context.Context, userID int64, announcementID string, _ int64) error {
	s.readUserID = userID
	s.readAnnouncementID = announcementID
	return nil
}
