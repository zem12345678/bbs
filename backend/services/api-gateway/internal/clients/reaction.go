package clients

import (
	"context"
	"fmt"
	"strings"

	"api-gateway/api/proto/reactionpb"
	iocgrpc "api-gateway/internal/ioc/grpc"

	"google.golang.org/grpc"
)

const reactionInternalAuthMetadataKey = "x-bbs-internal-token"

type reactionInternalAuthCredentials struct {
	token string
}

func (c reactionInternalAuthCredentials) GetRequestMetadata(context.Context, ...string) (map[string]string, error) {
	return map[string]string{reactionInternalAuthMetadataKey: c.token}, nil
}

func (reactionInternalAuthCredentials) RequireTransportSecurity() bool {
	return false
}

func (c *Clients) initReaction(grpcClient *iocgrpc.Client, o Options) error {
	token := strings.TrimSpace(o.ReactionInternalAuthToken)
	if token == "" {
		return fmt.Errorf("reaction internal auth token required")
	}
	conn, err := c.dial(grpcClient, o.Reaction, "reaction",
		iocgrpc.WithGrpcDialOptions(grpc.WithPerRPCCredentials(reactionInternalAuthCredentials{token: token})),
	)
	if err != nil {
		return err
	}
	c.Reaction = reactionpb.NewReactionServiceClient(conn)
	return nil
}
