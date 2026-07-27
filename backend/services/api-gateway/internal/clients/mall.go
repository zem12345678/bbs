package clients

import (
	"context"
	"fmt"
	"strings"

	"api-gateway/api/proto/mallpb"
	iocgrpc "api-gateway/internal/ioc/grpc"

	"google.golang.org/grpc"
)

const mallInternalAuthMetadataKey = "x-bbs-internal-token"

type mallInternalAuthCredentials struct {
	token  string
	secure bool
}

func (c mallInternalAuthCredentials) GetRequestMetadata(context.Context, ...string) (map[string]string, error) {
	return map[string]string{mallInternalAuthMetadataKey: c.token}, nil
}

func (c mallInternalAuthCredentials) RequireTransportSecurity() bool {
	return c.secure
}

func (c *Clients) initMall(grpcClient *iocgrpc.Client, o Options) error {
	token := strings.TrimSpace(o.MallInternalAuthToken)
	if token == "" {
		return fmt.Errorf("mall internal auth token required")
	}
	conn, err := c.dial(grpcClient, o.Mall, "mall",
		iocgrpc.WithSecureConnection(o.MallInternalAuthSecure),
		iocgrpc.WithGrpcDialOptions(grpc.WithPerRPCCredentials(mallInternalAuthCredentials{token: token, secure: o.MallInternalAuthSecure})),
	)
	if err != nil {
		return err
	}
	c.Mall = mallpb.NewMallServiceClient(conn)
	return nil
}
