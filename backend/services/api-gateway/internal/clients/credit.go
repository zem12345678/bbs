package clients

import (
	"api-gateway/api/proto/creditpb"
	iocgrpc "api-gateway/internal/ioc/grpc"
)

func (c *Clients) initCredit(grpcClient *iocgrpc.Client, o Options) error {
	conn, err := c.dial(grpcClient, o.Credit, "credit")
	if err != nil {
		return err
	}
	c.Credit = creditpb.NewCreditServiceClient(conn)
	return nil
}
