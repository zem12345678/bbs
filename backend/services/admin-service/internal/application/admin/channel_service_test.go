package admin

import (
	"context"
	"errors"
	"testing"

	domain "admin/internal/domain/admin"
)

func TestChannelGovernanceUsesExpectedPermissionsAndGatewayCalls(t *testing.T) {
	tests := []struct {
		name       string
		wantAction domain.Action
		invoke     func(*Service) error
		verify     func(*channelContentGateway) bool
	}{
		{
			name:       "list",
			wantAction: domain.ActionListChannels,
			invoke: func(service *Service) error {
				_, err := service.ListChannels(t.Context(), channelTestActor(), "go", 7, 2, 25, 10)
				return err
			},
			verify: func(gateway *channelContentGateway) bool {
				return gateway.listQuery == "go" && gateway.listCategoryID == 7 && gateway.listArchivedStatus == 2 && gateway.listLimit == 25 && gateway.listOffset == 10
			},
		},
		{
			name:       "feature",
			wantAction: domain.ActionFeatureChannel,
			invoke: func(service *Service) error {
				_, err := service.SetChannelFeatured(t.Context(), channelTestActor(), 11, true)
				return err
			},
			verify: func(gateway *channelContentGateway) bool {
				return gateway.featuredID == 11 && gateway.featured
			},
		},
		{
			name:       "unfeature",
			wantAction: domain.ActionFeatureChannel,
			invoke: func(service *Service) error {
				_, err := service.SetChannelFeatured(t.Context(), channelTestActor(), 12, false)
				return err
			},
			verify: func(gateway *channelContentGateway) bool {
				return gateway.featuredID == 12 && !gateway.featured
			},
		},
		{
			name:       "archive",
			wantAction: domain.ActionArchiveChannel,
			invoke: func(service *Service) error {
				_, err := service.SetChannelArchived(t.Context(), channelTestActor(), 13, true)
				return err
			},
			verify: func(gateway *channelContentGateway) bool {
				return gateway.archivedID == 13 && gateway.archived
			},
		},
		{
			name:       "restore",
			wantAction: domain.ActionRestoreChannel,
			invoke: func(service *Service) error {
				_, err := service.SetChannelArchived(t.Context(), channelTestActor(), 14, false)
				return err
			},
			verify: func(gateway *channelContentGateway) bool {
				return gateway.archivedID == 14 && !gateway.archived
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			auth := &channelAuthorizer{}
			gateway := &channelContentGateway{}
			service := &Service{auth: auth, content: gateway}
			if err := tt.invoke(service); err != nil {
				t.Fatalf("invoke error = %v", err)
			}
			if auth.action != tt.wantAction {
				t.Fatalf("authorized action = %q, want %q", auth.action, tt.wantAction)
			}
			if !tt.verify(gateway) {
				t.Fatalf("gateway call = %#v", gateway)
			}
		})
	}
}

func TestChannelGovernanceStopsBeforeGatewayWhenAuthorizationFails(t *testing.T) {
	wantErr := domain.ErrPermissionDenied
	gateway := &channelContentGateway{}
	service := &Service{auth: &channelAuthorizer{err: wantErr}, content: gateway}

	_, err := service.SetChannelFeatured(t.Context(), channelTestActor(), 11, true)
	if !errors.Is(err, wantErr) {
		t.Fatalf("SetChannelFeatured() error = %v, want permission denied", err)
	}
	if gateway.featuredID != 0 {
		t.Fatalf("featured gateway called with id %d", gateway.featuredID)
	}
}

func TestChannelGovernanceRejectsInvalidIDBeforeAuthorization(t *testing.T) {
	auth := &channelAuthorizer{}
	service := &Service{auth: auth, content: &channelContentGateway{}}

	_, err := service.SetChannelArchived(t.Context(), channelTestActor(), 0, true)
	if !errors.Is(err, domain.ErrInvalidChannelID) {
		t.Fatalf("SetChannelArchived() error = %v, want invalid channel id", err)
	}
	if auth.action != "" {
		t.Fatalf("authorized action = %q, want none", auth.action)
	}
}

func channelTestActor() domain.Actor {
	return domain.Actor{ID: 1, Username: "moderator"}
}

type channelAuthorizer struct {
	action domain.Action
	err    error
}

func (a *channelAuthorizer) Authorize(_ context.Context, _ domain.Actor, action domain.Action) error {
	a.action = action
	return a.err
}

func (*channelAuthorizer) Reload(context.Context) error { return nil }

type channelContentGateway struct {
	ContentGateway
	listQuery          string
	listCategoryID     int64
	listArchivedStatus int32
	listLimit          int32
	listOffset         int32
	featuredID         int64
	featured           bool
	archivedID         int64
	archived           bool
}

func (g *channelContentGateway) ListChannels(_ context.Context, query string, categoryID int64, archivedStatus int32, limit int32, offset int32) (domain.ChannelList, error) {
	g.listQuery = query
	g.listCategoryID = categoryID
	g.listArchivedStatus = archivedStatus
	g.listLimit = limit
	g.listOffset = offset
	return domain.ChannelList{}, nil
}

func (g *channelContentGateway) SetChannelFeatured(_ context.Context, id int64, featured bool) (domain.Channel, error) {
	g.featuredID = id
	g.featured = featured
	return domain.Channel{ID: id, IsFeatured: featured}, nil
}

func (g *channelContentGateway) SetChannelArchived(_ context.Context, id int64, archived bool) (domain.Channel, error) {
	g.archivedID = id
	g.archived = archived
	return domain.Channel{ID: id, IsArchived: archived}, nil
}
