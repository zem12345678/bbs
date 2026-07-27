package credit

import (
	"context"
	"fmt"
	"strings"

	"file-service/api/proto/creditpb"
	app "file-service/internal/application/file"
	"file-service/internal/clients/etcdresolver"
	domain "file-service/internal/domain/file"

	"github.com/spf13/viper"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type Client struct {
	client creditpb.CreditServiceClient
	close  func() error
}

const (
	etcdResolverScheme      = "file-etcd"
	internalAuthMetadataKey = "x-bbs-internal-token"
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

func NewClient(v *viper.Viper) (*Client, error) {
	token := strings.TrimSpace(v.GetString("upstreams.creditInternalAuthToken"))
	if token == "" {
		return nil, fmt.Errorf("credit internal auth token required")
	}
	service := normalizeServiceName(v.GetString("upstreams.credit"))
	conn, err := etcdresolver.Dial(
		v.GetStringSlice("grpc.client.etcdAddr"),
		etcdResolverScheme,
		service,
		"credit",
		grpc.WithPerRPCCredentials(internalAuthCredentials{token: token}),
	)
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
