package credit

import (
	"context"
	"errors"
	"strings"

	"mall-service/api/proto/creditpb"
	app "mall-service/internal/application/mall"
	domain "mall-service/internal/domain/mall"
	iocgrpc "mall-service/internal/ioc/grpc"

	"github.com/spf13/viper"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type Client struct {
	client creditpb.CreditServiceClient
}

func NewClient(grpcClient *iocgrpc.Client, v *viper.Viper) (*Client, error) {
	service := strings.TrimSpace(v.GetString("upstreams.credit"))
	if service == "" {
		service = "credit-service"
	}
	conn, err := grpcClient.Dial(service, false)
	if err != nil {
		return nil, err
	}
	return &Client{client: creditpb.NewCreditServiceClient(conn)}, nil
}

func (c *Client) DebitCredits(ctx context.Context, command app.CreditDebitCommand) error {
	_, err := c.client.DebitCredits(ctx, &creditpb.DebitCreditsRequest{
		UserId:        command.UserID,
		Amount:        command.Amount,
		Reason:        command.Reason,
		Description:   command.Description,
		SourceEventId: command.SourceEventID,
		SourceType:    command.SourceType,
		SourceId:      command.SourceID,
	})
	if err == nil {
		return nil
	}
	if errors.Is(err, domain.ErrInsufficientCredits) {
		return err
	}
	if status.Code(err) == codes.FailedPrecondition {
		return domain.ErrInsufficientCredits
	}
	return err
}

func (c *Client) AdjustCredits(ctx context.Context, command app.CreditAdjustCommand) error {
	_, err := c.client.AdjustCredits(ctx, &creditpb.AdjustCreditsRequest{
		UserId:        command.UserID,
		Delta:         command.Delta,
		Reason:        command.Reason,
		Description:   command.Description,
		SourceEventId: command.SourceEventID,
		SourceType:    command.SourceType,
		SourceId:      command.SourceID,
	})
	return err
}

var _ app.CreditCharger = (*Client)(nil)
