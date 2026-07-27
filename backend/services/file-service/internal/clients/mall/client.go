package mall

import (
	"context"
	"fmt"
	"strings"

	"file-service/api/proto/mallpb"
	app "file-service/internal/application/file"
	"file-service/internal/clients/etcdresolver"

	"github.com/spf13/viper"
	"google.golang.org/grpc"
)

const (
	digitalEntitlementGrantType = "membership"
	etcdResolverScheme          = "file-mall-etcd"
	internalAuthMetadataKey     = "x-bbs-internal-token"
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
	close  func() error
}

func NewClient(v *viper.Viper) (*Client, error) {
	token := strings.TrimSpace(v.GetString("upstreams.mallInternalAuthToken"))
	if token == "" {
		return nil, fmt.Errorf("mall internal auth token required")
	}
	conn, err := etcdresolver.Dial(
		v.GetStringSlice("grpc.client.etcdAddr"),
		etcdResolverScheme,
		normalizeServiceName(v.GetString("upstreams.mall")),
		"mall",
		grpc.WithPerRPCCredentials(internalAuthCredentials{token: token}),
	)
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
