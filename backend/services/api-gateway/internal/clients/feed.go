package clients

import (
	"api-gateway/api/proto/feedpb"
	iocgrpc "api-gateway/internal/ioc/grpc"
)

func (c *Clients) initFeed(grpcClient *iocgrpc.Client, o Options) error {
	conn, err := c.dial(grpcClient, o.Feed, "feed")
	if err != nil {
		return err
	}
	c.Feed = feedpb.NewFeedServiceClient(conn)
	return nil
}
