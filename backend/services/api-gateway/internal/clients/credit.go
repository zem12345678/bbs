package clients

import (
	"context"
	"fmt"
	"strings"

	"api-gateway/api/proto/creditpb"
	iocgrpc "api-gateway/internal/ioc/grpc"

	"google.golang.org/grpc"
)

const creditInternalAuthMetadataKey = "x-bbs-internal-token"

type creditInternalAuthCredentials struct {
	token  string
	secure bool
}

func (c creditInternalAuthCredentials) GetRequestMetadata(context.Context, ...string) (map[string]string, error) {
	return map[string]string{creditInternalAuthMetadataKey: c.token}, nil
}

func (c creditInternalAuthCredentials) RequireTransportSecurity() bool {
	return c.secure
}

func (c *Clients) initCredit(grpcClient *iocgrpc.Client, o Options) error {
	token := strings.TrimSpace(o.CreditInternalAuthToken)
	if token == "" {
		return fmt.Errorf("credit internal auth token required")
	}
	conn, err := c.dial(grpcClient, o.Credit, "credit",
		iocgrpc.WithSecureConnection(o.CreditInternalAuthSecure),
		iocgrpc.WithGrpcDialOptions(grpc.WithPerRPCCredentials(creditInternalAuthCredentials{token: token, secure: o.CreditInternalAuthSecure})),
	)
	if err != nil {
		return err
	}
	c.Credit = creditpb.NewCreditServiceClient(conn)
	return nil
}
