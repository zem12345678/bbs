package mall

import (
	"context"
	"strings"
	"time"

	"file-service/api/proto/mallpb"
	app "file-service/internal/application/file"
	"file-service/internal/clients/etcdresolver"

	"github.com/spf13/viper"
)

const (
	digitalEntitlementStatusActive       = "ACTIVE"
	digitalEntitlementGrantType          = "membership"
	digitalEntitlementLookupLimit  int32 = 20
	etcdResolverScheme                   = "file-mall-etcd"
)

type Client struct {
	client mallpb.MallServiceClient
	close  func() error
}

func NewClient(v *viper.Viper) (*Client, error) {
	conn, err := etcdresolver.Dial(v.GetStringSlice("grpc.client.etcdAddr"), etcdResolverScheme, normalizeServiceName(v.GetString("upstreams.mall")), "mall")
	if err != nil {
		return nil, err
	}
	return &Client{client: mallpb.NewMallServiceClient(conn), close: conn.Close}, nil
}

func (c *Client) Close() error {
	if c == nil || c.close == nil {
		return nil
	}
	return c.close()
}

func (c *Client) HasActiveMembership(ctx context.Context, userID int64) (bool, error) {
	now := time.Now()
	offset := int32(0)
	for {
		response, err := c.client.ListUserDigitalEntitlements(ctx, &mallpb.ListUserDigitalEntitlementsRequest{
			UserId:    userID,
			Status:    digitalEntitlementStatusActive,
			Limit:     digitalEntitlementLookupLimit,
			Offset:    offset,
			GrantType: digitalEntitlementGrantType,
		})
		if err != nil {
			return false, err
		}
		for _, entitlement := range response.GetItems() {
			if !digitalEntitlementIsActive(entitlement, now) {
				continue
			}
			if strings.ToLower(strings.TrimSpace(entitlement.GetGrantType())) != digitalEntitlementGrantType {
				continue
			}
			if strings.TrimSpace(entitlement.GetGrantKey()) == "" || entitlement.GetExpiresAt() <= now.UnixMilli() {
				continue
			}
			return true, nil
		}
		if int32(len(response.GetItems())) < digitalEntitlementLookupLimit {
			return false, nil
		}
		offset += digitalEntitlementLookupLimit
	}
}

func digitalEntitlementIsActive(entitlement *mallpb.DigitalEntitlement, now time.Time) bool {
	if entitlement == nil || entitlement.GetRevokedAt() > 0 {
		return false
	}
	if strings.ToUpper(strings.TrimSpace(entitlement.GetStatus())) != digitalEntitlementStatusActive {
		return false
	}
	expiresAt := entitlement.GetExpiresAt()
	return expiresAt <= 0 || expiresAt > now.UnixMilli()
}

func normalizeServiceName(value string) string {
	value = strings.TrimSpace(value)
	if value == "" || value == "mall-service" {
		return "bbs-mall-service"
	}
	return value
}

var _ app.MembershipEntitlementReader = (*Client)(nil)
