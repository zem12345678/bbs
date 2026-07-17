package credit

import (
	"context"
	"strings"

	"file-service/api/proto/creditpb"
	app "file-service/internal/application/file"
	domain "file-service/internal/domain/file"

	"github.com/spf13/viper"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type Client struct {
	client creditpb.CreditServiceClient
	close  func() error
}

func NewClient(v *viper.Viper) (*Client, error) {
	service := normalizeServiceName(v.GetString("upstreams.credit"))
	conn, err := dialEtcd(v.GetStringSlice("grpc.client.etcdAddr"), service)
	if err != nil {
		return nil, err
	}
	return &Client{client: creditpb.NewCreditServiceClient(conn), close: conn.Close}, nil
}

func (c *Client) Close() error {
	if c == nil || c.close == nil {
		return nil
	}
	return c.close()
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
	grpcStatus := status.Convert(err)
	if grpcStatus.Code() == codes.FailedPrecondition && strings.Contains(grpcStatus.Message(), "余额不足") {
		return domain.ErrInsufficientCredits
	}
	if grpcStatus.Code() == codes.Unavailable || grpcStatus.Code() == codes.DeadlineExceeded {
		return domain.ErrCreditServiceUnavailable
	}
	return err
}

func normalizeServiceName(value string) string {
	value = strings.TrimSpace(value)
	if value == "" || value == "credit-service" {
		return "bbs-credit-service"
	}
	return value
}

var _ app.CreditCharger = (*Client)(nil)
