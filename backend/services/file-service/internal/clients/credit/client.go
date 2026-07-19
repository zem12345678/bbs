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

func (c *Client) TransferCredits(ctx context.Context, command app.CreditTransferCommand) error {
	_, err := c.client.TransferCredits(ctx, &creditpb.TransferCreditsRequest{
		PayerUserId:       command.PayerUserID,
		PayeeUserId:       command.PayeeUserID,
		Amount:            command.Amount,
		DebitReason:       command.DebitReason,
		DebitDescription:  command.DebitDescription,
		CreditReason:      command.CreditReason,
		CreditDescription: command.CreditDescription,
		SourceEventId:     command.SourceEventID,
		SourceType:        command.SourceType,
		SourceId:          command.SourceID,
	})
	return creditError(err)
}

func creditError(err error) error {
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
