package clients

import (
	"context"
	"fmt"
	"strings"

	"api-gateway/api/proto/adminpb"
	iocgrpc "api-gateway/internal/ioc/grpc"

	"google.golang.org/grpc"
)

const adminInternalAuthMetadataKey = "x-bbs-internal-token"

type adminInternalAuthCredentials struct {
	token  string
	secure bool
}

func (c adminInternalAuthCredentials) GetRequestMetadata(context.Context, ...string) (map[string]string, error) {
	return map[string]string{adminInternalAuthMetadataKey: c.token}, nil
}

func (c adminInternalAuthCredentials) RequireTransportSecurity() bool {
	return c.secure
}

func (c *Clients) initAdmin(grpcClient *iocgrpc.Client, o Options) error {
	token := strings.TrimSpace(o.AdminInternalAuthToken)
	if token == "" {
		return fmt.Errorf("admin internal auth token required")
	}
	conn, err := c.dial(grpcClient, o.Admin, "admin",
		iocgrpc.WithSecureConnection(o.AdminInternalAuthSecure),
		iocgrpc.WithGrpcDialOptions(grpc.WithPerRPCCredentials(adminInternalAuthCredentials{token: token, secure: o.AdminInternalAuthSecure})),
	)
	if err != nil {
		return err
	}
	c.Admin = adminpb.NewAdminServiceClient(conn)
	return nil
}
