package clients

import (
	"api-gateway/api/proto/mallpb"
	iocgrpc "api-gateway/internal/ioc/grpc"
)

func (c *Clients) initMall(grpcClient *iocgrpc.Client, o Options) error {
	conn, err := c.dial(grpcClient, o.Mall, "mall")
	if err != nil {
		return err
	}
	c.Mall = mallpb.NewMallServiceClient(conn)
	return nil
}
