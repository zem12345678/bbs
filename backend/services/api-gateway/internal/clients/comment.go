package clients

import (
	"api-gateway/api/proto/commentpb"
	iocgrpc "api-gateway/internal/ioc/grpc"
)

func (c *Clients) initComment(grpcClient *iocgrpc.Client, o Options) error {
	conn, err := c.dial(grpcClient, o.Comment, "comment")
	if err != nil {
		return err
	}
	c.Comment = commentpb.NewCommentServiceClient(conn)
	return nil
}
