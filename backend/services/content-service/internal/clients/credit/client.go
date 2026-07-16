package credit

import (
	"context"
	"fmt"
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

func (c *Client) ReserveQABounty(ctx context.Context, userID, topicID, amount int64, title string) (bool, error) {
	if amount <= 0 {
		return true, nil
	}
	_, err := c.client.ReserveCredits(ctx, &creditpb.ReserveCreditsRequest{
		UserId:        userID,
		Amount:        amount,
		Reason:        "qa_bounty_reserved",
		Description:   qaBountyReservationDescription(topicID, title),
		SourceEventId: fmt.Sprintf("content.qa.bounty:%d", topicID),
		SourceType:    "topic",
		SourceId:      topicID,
	})
	if err != nil {
		return false, err
	}
	return true, nil
}

var _ topiccommand.BountyCreditReader = (*Client)(nil)

func qaBountyReservationDescription(topicID int64, title string) string {
	title = strings.TrimSpace(title)
	if title == "" {
		return fmt.Sprintf("问答悬赏冻结：话题 #%d", topicID)
	}
	return fmt.Sprintf("问答悬赏冻结：话题《%s》", title)
}
