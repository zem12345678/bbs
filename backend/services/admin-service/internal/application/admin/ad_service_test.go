package admin

import (
	"context"
	"errors"
	"testing"
	"time"

	domain "admin/internal/domain/admin"
)

func TestNormalizeAdCommandTrimsValidValues(t *testing.T) {
	command := validAdCommand()
	command.URL = "  https://example.com/landing  "
	command.Memo = "  campaign  "
	command.Place = "  vertical  "
	command.Priority = "  high  "
	command.ImageURL = "  https://cdn.example.com/ad.png  "

	got, err := normalizeAdCommand(command)
	if err != nil {
		t.Fatalf("normalizeAdCommand() error = %v", err)
	}
	if got.URL != "https://example.com/landing" || got.Memo != "campaign" || got.Place != "vertical" || got.Priority != "high" || got.ImageURL != "https://cdn.example.com/ad.png" {
		t.Fatalf("normalizeAdCommand() = %#v", got)
	}
}

func TestNormalizeAdCommandRejectsInvalidValues(t *testing.T) {
	tests := []struct {
		name   string
		mutate func(*domain.CreateAdCommand)
	}{
		{name: "unsafe url", mutate: func(command *domain.CreateAdCommand) { command.URL = "javascript:alert(1)" }},
		{name: "url credentials", mutate: func(command *domain.CreateAdCommand) { command.URL = "https://user:pass@example.com" }},
		{name: "unsafe image url", mutate: func(command *domain.CreateAdCommand) { command.ImageURL = "/ad.png" }},
		{name: "invalid place", mutate: func(command *domain.CreateAdCommand) { command.Place = "sidebar" }},
		{name: "invalid priority", mutate: func(command *domain.CreateAdCommand) { command.Priority = "urgent" }},
		{name: "zero ratio", mutate: func(command *domain.CreateAdCommand) { command.Ratio = 0 }},
		{name: "negative ratio", mutate: func(command *domain.CreateAdCommand) { command.Ratio = -1 }},
		{name: "empty weekday", mutate: func(command *domain.CreateAdCommand) { command.DayOfWeek = 0 }},
		{name: "weekday overflow", mutate: func(command *domain.CreateAdCommand) { command.DayOfWeek = 128 }},
		{name: "equal timestamps", mutate: func(command *domain.CreateAdCommand) { command.ExpiresAt = command.StartsAt }},
		{name: "reversed timestamps", mutate: func(command *domain.CreateAdCommand) { command.ExpiresAt = command.StartsAt.Add(-time.Millisecond) }},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			command := validAdCommand()
			tt.mutate(&command)
			if _, err := normalizeAdCommand(command); !errors.Is(err, domain.ErrInvalidAd) {
				t.Fatalf("normalizeAdCommand() error = %v, want ErrInvalidAd", err)
			}
		})
	}
}

func TestAdPublishingAtUsesHalfOpenWindowAndUTCWeekday(t *testing.T) {
	monday := time.Date(2026, time.August, 10, 2, 0, 0, 0, time.UTC)
	ad := domain.Ad{
		StartsAt:  monday,
		ExpiresAt: monday.Add(time.Hour),
		DayOfWeek: 1 << uint(time.Monday),
	}
	if !ad.PublishingAt(monday) {
		t.Fatal("ad should publish at starts_at")
	}
	if ad.PublishingAt(monday.Add(time.Hour)) {
		t.Fatal("ad must not publish at expires_at")
	}
	if ad.PublishingAt(monday.Add(24 * time.Hour)) {
		t.Fatal("ad must not publish on a weekday outside its mask")
	}
	localMonday := monday.In(time.FixedZone("UTC-8", -8*60*60))
	if !ad.PublishingAt(localMonday) {
		t.Fatal("weekday evaluation must use UTC")
	}
}

func TestHighestPriorityAds(t *testing.T) {
	items := []domain.Ad{
		{ID: 1, Priority: "low"},
		{ID: 2, Priority: "high"},
		{ID: 3, Priority: "middle"},
		{ID: 4, Priority: "high"},
	}
	got := highestPriorityAds(items)
	if len(got) != 2 || got[0].ID != 2 || got[1].ID != 4 {
		t.Fatalf("highestPriorityAds() = %#v, want high ads only", got)
	}

	got = highestPriorityAds([]domain.Ad{{ID: 5, Priority: "low"}, {ID: 6, Priority: "low"}})
	if len(got) != 2 || got[0].ID != 5 || got[1].ID != 6 {
		t.Fatalf("highestPriorityAds(low only) = %#v", got)
	}
}

func TestAdManagementMethodsAuthorizeExpectedActions(t *testing.T) {
	actor := domain.Actor{ID: 1, Username: "admin"}
	tests := []struct {
		name   string
		action domain.Action
		run    func(*Service) error
	}{
		{name: "list", action: domain.ActionListAds, run: func(service *Service) error {
			_, err := service.ListAds(context.Background(), actor, 10, 0, 0, nil)
			return err
		}},
		{name: "create", action: domain.ActionCreateAd, run: func(service *Service) error {
			_, err := service.CreateAd(context.Background(), actor, validAdCommand())
			return err
		}},
		{name: "update", action: domain.ActionUpdateAd, run: func(service *Service) error {
			_, err := service.UpdateAd(context.Background(), actor, domain.UpdateAdCommand{ID: 1})
			return err
		}},
		{name: "delete", action: domain.ActionDeleteAd, run: func(service *Service) error {
			return service.DeleteAd(context.Background(), actor, 1)
		}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			authorizer := &adAuthorizerStub{err: domain.ErrPermissionDenied}
			service := &Service{auth: authorizer, ops: &adOperationStoreStub{}}
			if err := tt.run(service); !errors.Is(err, domain.ErrPermissionDenied) {
				t.Fatalf("method error = %v, want ErrPermissionDenied", err)
			}
			if len(authorizer.actions) != 1 || authorizer.actions[0] != tt.action {
				t.Fatalf("authorized actions = %v, want [%s]", authorizer.actions, tt.action)
			}
		})
	}
}

func TestListActiveAdsIsAnonymousAndSelectsHighestPriority(t *testing.T) {
	authorizer := &adAuthorizerStub{err: domain.ErrPermissionDenied}
	store := &adOperationStoreStub{active: []domain.Ad{{ID: 1, Priority: "low"}, {ID: 2, Priority: "middle"}}}
	service := &Service{auth: authorizer, ops: store}

	items, err := service.ListActiveAds(context.Background())
	if err != nil {
		t.Fatalf("ListActiveAds() error = %v", err)
	}
	if len(authorizer.actions) != 0 {
		t.Fatalf("ListActiveAds() authorized actions = %v, want none", authorizer.actions)
	}
	if len(items) != 1 || items[0].ID != 2 {
		t.Fatalf("ListActiveAds() = %#v, want middle priority group", items)
	}
}

func TestUpdateAdMergesOptionalFields(t *testing.T) {
	memo := "  updated campaign  "
	command := validAdCommand()
	store := &adOperationStoreStub{existing: domain.Ad{
		ID:        7,
		URL:       command.URL,
		Memo:      command.Memo,
		Place:     command.Place,
		Priority:  command.Priority,
		Ratio:     command.Ratio,
		StartsAt:  command.StartsAt,
		ExpiresAt: command.ExpiresAt,
		ImageURL:  command.ImageURL,
		DayOfWeek: command.DayOfWeek,
	}}
	service := &Service{auth: &adAuthorizerStub{}, ops: store}

	_, err := service.UpdateAd(context.Background(), domain.Actor{ID: 1, Username: "admin"}, domain.UpdateAdCommand{ID: 7, Memo: &memo})
	if err != nil {
		t.Fatalf("UpdateAd() error = %v", err)
	}
	if store.updated == nil {
		t.Fatal("UpdateAd() did not call the store")
	}
	if store.updated.Memo != "updated campaign" || store.updated.URL != command.URL || store.updated.ImageURL != command.ImageURL || store.updated.Ratio != command.Ratio {
		t.Fatalf("merged update = %#v", *store.updated)
	}
}

func validAdCommand() domain.CreateAdCommand {
	return domain.CreateAdCommand{
		URL:       "https://example.com/landing",
		Memo:      "campaign",
		Place:     "vertical",
		Priority:  "high",
		Ratio:     1,
		StartsAt:  time.Date(2026, time.August, 10, 0, 0, 0, 0, time.UTC),
		ExpiresAt: time.Date(2026, time.August, 11, 0, 0, 0, 0, time.UTC),
		ImageURL:  "https://cdn.example.com/ad.png",
		DayOfWeek: 127,
	}
}

type adAuthorizerStub struct {
	actions []domain.Action
	err     error
}

func (a *adAuthorizerStub) Authorize(_ context.Context, _ domain.Actor, action domain.Action) error {
	a.actions = append(a.actions, action)
	return a.err
}

func (a *adAuthorizerStub) Reload(context.Context) error {
	return nil
}

type adOperationStoreStub struct {
	OperationStore
	active   []domain.Ad
	existing domain.Ad
	updated  *domain.CreateAdCommand
}

func (s *adOperationStoreStub) ListAds(context.Context, int32, int64, int64, *bool, time.Time) (domain.AdList, error) {
	return domain.AdList{}, nil
}

func (s *adOperationStoreStub) ListActiveAds(context.Context, time.Time) ([]domain.Ad, error) {
	return s.active, nil
}

func (s *adOperationStoreStub) GetAd(context.Context, int64) (domain.Ad, error) {
	if s.existing.ID > 0 {
		return s.existing, nil
	}
	command := validAdCommand()
	return domain.Ad{ID: 1, URL: command.URL, Memo: command.Memo, Place: command.Place, Priority: command.Priority, Ratio: command.Ratio, StartsAt: command.StartsAt, ExpiresAt: command.ExpiresAt, ImageURL: command.ImageURL, DayOfWeek: command.DayOfWeek}, nil
}

func (s *adOperationStoreStub) CreateAd(_ context.Context, command domain.CreateAdCommand) (domain.Ad, error) {
	return domain.Ad{ID: 1, URL: command.URL}, nil
}

func (s *adOperationStoreStub) UpdateAd(_ context.Context, id int64, command domain.CreateAdCommand) (domain.Ad, error) {
	s.updated = &command
	return domain.Ad{ID: id, URL: command.URL}, nil
}

func (s *adOperationStoreStub) DeleteAd(context.Context, int64) error {
	return nil
}
