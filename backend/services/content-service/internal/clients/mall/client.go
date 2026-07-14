package mall

import (
	"context"
	"strings"
	"time"

	"content-service/api/proto/mallpb"
	topiccommand "content-service/internal/application/topic/command"
	iocgrpc "content-service/internal/ioc/grpc"

	"github.com/spf13/viper"
)

const (
	digitalEntitlementStatusActive = "ACTIVE"
	digitalEntitlementGrantType    = "membership"
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

func (c *Client) HasActiveMembership(ctx context.Context, userID int64) (bool, error) {
	resp, err := c.client.ListUserDigitalEntitlements(ctx, &mallpb.ListUserDigitalEntitlementsRequest{
		UserId:    userID,
		Status:    digitalEntitlementStatusActive,
		Limit:     1,
		Offset:    0,
		GrantType: digitalEntitlementGrantType,
	})
	if err != nil {
		return false, err
	}
	now := time.Now()
	for _, entitlement := range resp.GetItems() {
		if !digitalEntitlementIsActive(entitlement, now) {
			continue
		}
		if strings.ToLower(strings.TrimSpace(entitlement.GetGrantType())) != digitalEntitlementGrantType {
			continue
		}
		return true, nil
	}
	return false, nil
}

func digitalEntitlementIsActive(entitlement *mallpb.DigitalEntitlement, now time.Time) bool {
	if entitlement == nil || entitlement.GetRevokedAt() > 0 {
		return false
	}
	statusText := strings.ToUpper(strings.TrimSpace(entitlement.GetStatus()))
	if statusText != "" && statusText != digitalEntitlementStatusActive {
		return false
	}
	expiresAt := entitlement.GetExpiresAt()
	return expiresAt <= 0 || expiresAt > now.UnixMilli()
}

var _ topiccommand.MembershipEntitlementReader = (*Client)(nil)
