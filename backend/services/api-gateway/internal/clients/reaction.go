package clients

import (
	"api-gateway/api/proto/reactionpb"
	iocgrpc "api-gateway/internal/ioc/grpc"
)

func (c *Clients) initReaction(grpcClient *iocgrpc.Client, o Options) error {
	conn, err := c.dial(grpcClient, o.Reaction, "reaction")
	if err != nil {
		return err
	}
	c.Reaction = reactionpb.NewReactionServiceClient(conn)
	return nil
}
