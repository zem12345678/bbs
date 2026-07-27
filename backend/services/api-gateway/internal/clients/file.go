package clients

import (
	"context"
	"fmt"
	"strings"

	"api-gateway/api/proto/filepb"
	iocgrpc "api-gateway/internal/ioc/grpc"

	"google.golang.org/grpc"
)

const fileInternalAuthMetadataKey = "x-bbs-internal-token"

type fileInternalAuthCredentials struct {
	token  string
	secure bool
}

func (c fileInternalAuthCredentials) GetRequestMetadata(context.Context, ...string) (map[string]string, error) {
	return map[string]string{fileInternalAuthMetadataKey: c.token}, nil
}

func (c fileInternalAuthCredentials) RequireTransportSecurity() bool {
	return c.secure
}

func (c *Clients) initFile(grpcClient *iocgrpc.Client, o Options) error {
	token := strings.TrimSpace(o.FileInternalAuthToken)
	if token == "" {
		return fmt.Errorf("file internal auth token required")
	}
	conn, err := c.dial(grpcClient, o.File, "file",
		iocgrpc.WithSecureConnection(o.FileInternalAuthSecure),
		iocgrpc.WithGrpcDialOptions(grpc.WithPerRPCCredentials(fileInternalAuthCredentials{token: token, secure: o.FileInternalAuthSecure})),
	)
	if err != nil {
		return err
	}
	c.File = filepb.NewFileServiceClient(conn)
	return nil
}
