package mall

import (
	"context"
	"strings"
	"time"

	"user-service/api/proto/mallpb"
	"user-service/internal/application/user/command"
	iocgrpc "user-service/internal/ioc/grpc"

	"github.com/spf13/viper"
)

const (
	digitalEntitlementStatusActive = "ACTIVE"
	digitalEntitlementGrantType    = "theme"
	digitalEntitlementLookupLimit  = 20
)

type Client struct {
	client mallpb.MallServiceClient
}

func NewClient(grpcClient *iocgrpc.Client, v *viper.Viper) (*Client, error) {
	service := serviceName(v.GetString("upstreams.mall"))
	if service == "" {
		service = "bbs-mall-service"
	}
	conn, err := grpcClient.Dial(service, false)
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
	now := time.Now()
	offset := int32(0)
	for {
		resp, err := c.client.ListUserDigitalEntitlements(ctx, &mallpb.ListUserDigitalEntitlementsRequest{
			UserId:    userID,
			Status:    digitalEntitlementStatusActive,
			Limit:     digitalEntitlementLookupLimit,
			Offset:    offset,
			GrantType: digitalEntitlementGrantType,
			GrantKey:  theme,
		})
		if err != nil {
			return false, err
		}
		for _, entitlement := range resp.GetItems() {
			if !digitalEntitlementIsActive(entitlement, now) {
				continue
			}
			if strings.ToLower(strings.TrimSpace(entitlement.GetGrantType())) != digitalEntitlementGrantType {
				continue
			}
			if strings.ToLower(strings.TrimSpace(entitlement.GetGrantKey())) != theme {
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
