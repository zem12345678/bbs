package grpc

import (
	"chat-service/api/proto/chatpb"
	iocgrpc "chat-service/internal/ioc/grpc"

	"github.com/google/wire"
	stdgrpc "google.golang.org/grpc"
)

func Register(server *stdgrpc.Server, handler *Handler) {
	chatpb.RegisterChatServiceServer(server, handler)
}

func NewInitServers(handler *Handler) iocgrpc.InitServers {
	return func(server *stdgrpc.Server) {
		Register(server, handler)
	}
}

var ProviderSet = wire.NewSet(NewHandler, NewInitServers)
