package clients

import (
	"api-gateway/api/proto/userpb"
	iocgrpc "api-gateway/internal/ioc/grpc"
)

func (c *Clients) initUser(grpcClient *iocgrpc.Client, o Options) error {
	conn, err := c.dial(grpcClient, o.User, "user")
	if err != nil {
		return err
	}
	c.User = userpb.NewUserServiceClient(conn)
	return nil
}
