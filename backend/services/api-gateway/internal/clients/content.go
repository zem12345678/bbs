package clients

import (
	"context"
	"fmt"
	"strings"

	"api-gateway/api/proto/contentpb"
	iocgrpc "api-gateway/internal/ioc/grpc"

	"google.golang.org/grpc"
)

const contentInternalAuthMetadataKey = "x-bbs-internal-token"

type contentInternalAuthCredentials struct {
	token string
}

func (c contentInternalAuthCredentials) GetRequestMetadata(context.Context, ...string) (map[string]string, error) {
	return map[string]string{contentInternalAuthMetadataKey: c.token}, nil
}

func (contentInternalAuthCredentials) RequireTransportSecurity() bool {
	return false
}

func (c *Clients) initContent(grpcClient *iocgrpc.Client, o Options) error {
	token := strings.TrimSpace(o.ContentInternalAuthToken)
	if token == "" {
		return fmt.Errorf("content internal auth token required")
	}
	conn, err := c.dial(grpcClient, o.Content, "content",
		iocgrpc.WithGrpcDialOptions(grpc.WithPerRPCCredentials(contentInternalAuthCredentials{token: token})),
	)
	if err != nil {
		return err
	}
	c.Content = contentpb.NewContentServiceClient(conn)
	return nil
}
