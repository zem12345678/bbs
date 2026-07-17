package clients

import (
	"api-gateway/api/proto/filepb"
	iocgrpc "api-gateway/internal/ioc/grpc"
)

func (c *Clients) initFile(grpcClient *iocgrpc.Client, o Options) error {
	conn, err := c.dial(grpcClient, o.File, "file")
	if err != nil {
		return err
	}
	c.File = filepb.NewFileServiceClient(conn)
	return nil
}
