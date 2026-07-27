package clients

import (
	"context"
	"fmt"
	"strings"

	"api-gateway/api/proto/searchpb"
	iocgrpc "api-gateway/internal/ioc/grpc"

	"google.golang.org/grpc"
)

const searchInternalAuthMetadataKey = "x-bbs-internal-token"

type searchInternalAuthCredentials struct {
	token string
}

func (c searchInternalAuthCredentials) GetRequestMetadata(context.Context, ...string) (map[string]string, error) {
	return map[string]string{searchInternalAuthMetadataKey: c.token}, nil
}

func (searchInternalAuthCredentials) RequireTransportSecurity() bool {
	return false
}

func (c *Clients) initSearch(grpcClient *iocgrpc.Client, o Options) error {
	token := strings.TrimSpace(o.SearchInternalAuthToken)
	if token == "" {
		return fmt.Errorf("search internal auth token required")
	}
	conn, err := c.dial(grpcClient, o.Search, "search",
		iocgrpc.WithGrpcDialOptions(grpc.WithPerRPCCredentials(searchInternalAuthCredentials{token: token})),
	)
	if err != nil {
		return err
	}
	c.Search = searchpb.NewSearchServiceClient(conn)
	return nil
}
