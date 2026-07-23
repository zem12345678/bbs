package mall

import (
	"context"
	"strings"

	"content-service/api/proto/mallpb"
	topiccommand "content-service/internal/application/topic/command"
	iocgrpc "content-service/internal/ioc/grpc"

	"github.com/spf13/viper"
)

const (
	digitalEntitlementGrantType = "membership"
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
	if userID <= 0 {
		return false, nil
	}
	resp, err := c.client.ListActiveEntitlementUserIDs(ctx, &mallpb.ListActiveEntitlementUserIDsRequest{
		UserIds:   []int64{userID},
		GrantType: digitalEntitlementGrantType,
	})
	if err != nil {
		return false, err
	}
	for _, activeUserID := range resp.GetUserIds() {
		if activeUserID == userID {
			return true, nil
		}
	}
	return false, nil
}

var _ topiccommand.MembershipEntitlementReader = (*Client)(nil)
