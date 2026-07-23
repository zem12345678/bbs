package mall

import (
	"context"
	"strings"

	"file-service/api/proto/mallpb"
	app "file-service/internal/application/file"
	"file-service/internal/clients/etcdresolver"

	"github.com/spf13/viper"
)

const (
	digitalEntitlementGrantType = "membership"
	etcdResolverScheme          = "file-mall-etcd"
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
	if userID <= 0 {
		return false, nil
	}
	response, err := c.client.ListActiveEntitlementUserIDs(ctx, &mallpb.ListActiveEntitlementUserIDsRequest{
		UserIds:   []int64{userID},
		GrantType: digitalEntitlementGrantType,
	})
	if err != nil {
		return false, err
	}
	for _, activeUserID := range response.GetUserIds() {
		if activeUserID == userID {
			return true, nil
		}
	}
	return false, nil
}

func normalizeServiceName(value string) string {
	value = strings.TrimSpace(value)
	if value == "" || value == "mall-service" {
		return "bbs-mall-service"
	}
	return value
}

var _ app.MembershipEntitlementReader = (*Client)(nil)
