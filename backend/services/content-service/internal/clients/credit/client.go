package credit

import (
	"context"
	"strings"

	"content-service/api/proto/creditpb"
	topiccommand "content-service/internal/application/topic/command"
	iocgrpc "content-service/internal/ioc/grpc"

	"github.com/spf13/viper"
)

type Client struct {
	client creditpb.CreditServiceClient
}

func NewClient(grpcClient *iocgrpc.Client, v *viper.Viper) (*Client, error) {
	service := serviceName(v.GetString("upstreams.credit"))
	if service == "" {
		service = "bbs-credit-service"
	}
	conn, err := grpcClient.Dial(service, false)
	if err != nil {
		return nil, err
	}
	return &Client{client: creditpb.NewCreditServiceClient(conn)}, nil
}

func serviceName(value string) string {
	value = strings.TrimSpace(value)
	if value == "credit-service" {
		return "bbs-credit-service"
	}
	return value
}

func (c *Client) HasEnoughCredit(ctx context.Context, userID, amount int64) (bool, error) {
	if amount <= 0 {
		return true, nil
	}
	resp, err := c.client.GetBalance(ctx, &creditpb.GetBalanceRequest{UserId: userID})
	if err != nil {
		return false, err
	}
	return resp.GetBalance().GetTotal() >= amount, nil
}

var _ topiccommand.BountyCreditReader = (*Client)(nil)
