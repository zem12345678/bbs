package clients

import (
	"context"
	"fmt"
	"strings"

	"api-gateway/api/proto/feedpb"
	iocgrpc "api-gateway/internal/ioc/grpc"

	"google.golang.org/grpc"
)

const feedInternalAuthMetadataKey = "x-bbs-internal-token"

type feedInternalAuthCredentials struct {
	token string
}

func (c feedInternalAuthCredentials) GetRequestMetadata(context.Context, ...string) (map[string]string, error) {
	return map[string]string{feedInternalAuthMetadataKey: c.token}, nil
}

func (feedInternalAuthCredentials) RequireTransportSecurity() bool {
	return false
}

func (c *Clients) initFeed(grpcClient *iocgrpc.Client, o Options) error {
	token := strings.TrimSpace(o.FeedInternalAuthToken)
	if token == "" {
		return fmt.Errorf("feed internal auth token required")
	}
	conn, err := c.dial(grpcClient, o.Feed, "feed",
		iocgrpc.WithGrpcDialOptions(grpc.WithPerRPCCredentials(feedInternalAuthCredentials{token: token})),
	)
	if err != nil {
		return err
	}
	c.Feed = feedpb.NewFeedServiceClient(conn)
	return nil
}
