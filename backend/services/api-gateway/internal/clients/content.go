package clients

import (
	"api-gateway/api/proto/contentpb"
	iocgrpc "api-gateway/internal/ioc/grpc"
)

func (c *Clients) initContent(grpcClient *iocgrpc.Client, o Options) error {
	conn, err := c.dial(grpcClient, o.Content, "content")
	if err != nil {
		return err
	}
	c.Content = contentpb.NewContentServiceClient(conn)
	return nil
}
