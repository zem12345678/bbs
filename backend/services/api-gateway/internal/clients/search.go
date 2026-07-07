package clients

import (
	"api-gateway/api/proto/searchpb"
	iocgrpc "api-gateway/internal/ioc/grpc"
)

func (c *Clients) initSearch(grpcClient *iocgrpc.Client, o Options) error {
	conn, err := c.dial(grpcClient, o.Search, "search")
	if err != nil {
		return err
	}
	c.Search = searchpb.NewSearchServiceClient(conn)
	return nil
}
