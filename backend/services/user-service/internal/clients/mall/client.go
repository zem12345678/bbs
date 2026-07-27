package mall

import (
	"context"
	"fmt"
	"strings"
	"time"

	"user-service/api/proto/mallpb"
	"user-service/internal/application/user/command"
	iocgrpc "user-service/internal/ioc/grpc"

	"github.com/spf13/viper"
	"google.golang.org/grpc"
)

const (
	digitalEntitlementStatusActive         = "ACTIVE"
	digitalEntitlementGrantTypeMembership  = "membership"
	digitalEntitlementGrantTypeTheme       = "theme"
	digitalEntitlementLookupLimit          = 20
	digitalEntitlementBatchUserLookupLimit = 100
	internalAuthMetadataKey                = "x-bbs-internal-token"
)

type internalAuthCredentials struct {
	token string
}

func (c internalAuthCredentials) GetRequestMetadata(context.Context, ...string) (map[string]string, error) {
	return map[string]string{internalAuthMetadataKey: c.token}, nil
}

func (internalAuthCredentials) RequireTransportSecurity() bool {
	return false
}

type Client struct {
	client mallpb.MallServiceClient
}

func NewClient(grpcClient *iocgrpc.Client, v *viper.Viper) (*Client, error) {
	service := serviceName(v.GetString("upstreams.mall"))
	if service == "" {
		service = "bbs-mall-service"
	}
	token := strings.TrimSpace(v.GetString("upstreams.mallInternalAuthToken"))
	if token == "" {
		return nil, fmt.Errorf("mall internal auth token required")
	}
	conn, err := grpcClient.Dial(service, false,
		iocgrpc.WithGrpcDialOptions(grpc.WithPerRPCCredentials(internalAuthCredentials{token: token})),
	)
	if err != nil {
		return nil, err
	}
	return &Client{client: mallpb.NewMallServiceClient(conn)}, nil
}

func serviceName(value string) string {
	value = strings.TrimSpace(value)
	if value == "mall-service" {
		return "bbs-mall-service"
	}
	return value
}

func (c *Client) HasActiveProfileTheme(ctx context.Context, userID int64, theme string) (bool, error) {
	theme = strings.ToLower(strings.TrimSpace(theme))
	if theme == "" {
		return false, nil
	}
	return c.hasActiveDigitalEntitlement(ctx, userID, digitalEntitlementGrantTypeTheme, theme, false)
}

func (c *Client) HasActiveMembership(ctx context.Context, userID int64) (bool, error) {
	return c.hasActiveDigitalEntitlement(ctx, userID, digitalEntitlementGrantTypeMembership, "", true)
}

func (c *Client) ListActiveProfileThemeUserIDs(ctx context.Context, userIDs []int64, theme string) (map[int64]bool, error) {
	theme = strings.ToLower(strings.TrimSpace(theme))
	if theme == "" {
		return map[int64]bool{}, nil
	}
	return c.listActiveEntitlementUserIDs(ctx, userIDs, digitalEntitlementGrantTypeTheme, theme)
}

func (c *Client) ListActiveMembershipUserIDs(ctx context.Context, userIDs []int64) (map[int64]bool, error) {
	return c.listActiveEntitlementUserIDs(ctx, userIDs, digitalEntitlementGrantTypeMembership, "")
}

func (c *Client) listActiveEntitlementUserIDs(ctx context.Context, userIDs []int64, grantType string, grantKey string) (map[int64]bool, error) {
	active := make(map[int64]bool)
	requested := make(map[int64]bool, len(userIDs))
	orderedUserIDs := make([]int64, 0, len(userIDs))
	for _, userID := range userIDs {
		if userID <= 0 || requested[userID] {
			continue
		}
		requested[userID] = true
		orderedUserIDs = append(orderedUserIDs, userID)
	}
	for start := 0; start < len(orderedUserIDs); start += digitalEntitlementBatchUserLookupLimit {
		end := start + digitalEntitlementBatchUserLookupLimit
		if end > len(orderedUserIDs) {
			end = len(orderedUserIDs)
		}
		resp, err := c.client.ListActiveEntitlementUserIDs(ctx, &mallpb.ListActiveEntitlementUserIDsRequest{
			UserIds:   orderedUserIDs[start:end],
			GrantType: grantType,
			GrantKey:  grantKey,
		})
		if err != nil {
			return nil, err
		}
		for _, userID := range resp.GetUserIds() {
			if requested[userID] {
				active[userID] = true
			}
		}
	}
	return active, nil
}

func (c *Client) hasActiveDigitalEntitlement(ctx context.Context, userID int64, grantType string, grantKey string, requireFutureExpiry bool) (bool, error) {
	grantType = strings.ToLower(strings.TrimSpace(grantType))
	grantKey = strings.ToLower(strings.TrimSpace(grantKey))
	if grantType == "" {
		return false, nil
	}
	now := time.Now()
	offset := int32(0)
	for {
		resp, err := c.client.ListUserDigitalEntitlements(ctx, &mallpb.ListUserDigitalEntitlementsRequest{
			UserId:    userID,
			Status:    digitalEntitlementStatusActive,
			Limit:     digitalEntitlementLookupLimit,
			Offset:    offset,
			GrantType: grantType,
			GrantKey:  grantKey,
		})
		if err != nil {
			return false, err
		}
		for _, entitlement := range resp.GetItems() {
			if !digitalEntitlementIsActive(entitlement, now) {
				continue
			}
			if strings.ToLower(strings.TrimSpace(entitlement.GetGrantType())) != grantType {
				continue
			}
			entitlementGrantKey := strings.ToLower(strings.TrimSpace(entitlement.GetGrantKey()))
			if entitlementGrantKey == "" {
				continue
			}
			if grantKey != "" && entitlementGrantKey != grantKey {
				continue
			}
			if requireFutureExpiry && entitlement.GetExpiresAt() <= now.UnixMilli() {
				continue
			}
			return true, nil
		}
		if int32(len(resp.GetItems())) < digitalEntitlementLookupLimit {
			break
		}
		offset += digitalEntitlementLookupLimit
	}
	return false, nil
}

func digitalEntitlementIsActive(entitlement *mallpb.DigitalEntitlement, now time.Time) bool {
	if entitlement == nil || entitlement.GetRevokedAt() > 0 {
		return false
	}
	statusText := strings.ToUpper(strings.TrimSpace(entitlement.GetStatus()))
	if statusText != digitalEntitlementStatusActive {
		return false
	}
	expiresAt := entitlement.GetExpiresAt()
	return expiresAt <= 0 || expiresAt > now.UnixMilli()
}

var _ command.ProfileThemeEntitlementReader = (*Client)(nil)
