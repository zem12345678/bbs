package grpc

import (
	"chat-service/api/proto/chatpb"

	stdgrpc "google.golang.org/grpc"
)

func Register(server *stdgrpc.Server, handler *Handler) {
	chatpb.RegisterChatServiceServer(server, handler)
}
