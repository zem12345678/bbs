package clients

import (
	"api-gateway/api/proto/chatpb"
	iocgrpc "api-gateway/internal/ioc/grpc"
)

func (c *Clients) initChat(grpcClient *iocgrpc.Client, o Options) error {
	conn, err := c.dial(grpcClient, o.Chat, "chat")
	if err != nil {
		return err
	}
	c.Chat = chatpb.NewChatServiceClient(conn)
	return nil
}
