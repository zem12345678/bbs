package clients

import (
	"context"
	"fmt"
	"strings"

	"api-gateway/api/proto/chatpb"
	iocgrpc "api-gateway/internal/ioc/grpc"

	"google.golang.org/grpc"
)

const chatInternalAuthMetadataKey = "x-bbs-internal-token"

type chatInternalAuthCredentials struct {
	token  string
	secure bool
}

func (c chatInternalAuthCredentials) GetRequestMetadata(context.Context, ...string) (map[string]string, error) {
	return map[string]string{chatInternalAuthMetadataKey: c.token}, nil
}

func (c chatInternalAuthCredentials) RequireTransportSecurity() bool {
	return c.secure
}

func (c *Clients) initChat(grpcClient *iocgrpc.Client, o Options) error {
	token := strings.TrimSpace(o.ChatInternalAuthToken)
	if token == "" {
		return fmt.Errorf("chat internal auth token required")
	}
	conn, err := c.dial(grpcClient, o.Chat, "chat",
		iocgrpc.WithSecureConnection(o.ChatInternalAuthSecure),
		iocgrpc.WithGrpcDialOptions(grpc.WithPerRPCCredentials(chatInternalAuthCredentials{token: token, secure: o.ChatInternalAuthSecure})),
	)
	if err != nil {
		return err
	}
	c.Chat = chatpb.NewChatServiceClient(conn)
	return nil
}
