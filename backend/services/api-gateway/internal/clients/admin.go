package clients

import (
	"api-gateway/api/proto/adminpb"
	iocgrpc "api-gateway/internal/ioc/grpc"
)

func (c *Clients) initAdmin(grpcClient *iocgrpc.Client, o Options) error {
	conn, err := c.dial(grpcClient, o.Admin, "admin")
	if err != nil {
		return err
	}
	c.Admin = adminpb.NewAdminServiceClient(conn)
	return nil
}
