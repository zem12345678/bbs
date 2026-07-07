package grpc

import (
	pb "notification-service/api/proto/notificationpb"
	iocgrpc "notification-service/internal/ioc/grpc"

	"github.com/google/wire"
	stdgrpc "google.golang.org/grpc"
)

func NewInitServers(h *Handler) iocgrpc.InitServers {
	return func(s *stdgrpc.Server) {
		pb.RegisterNotificationServiceServer(s, h)
	}
}

var ProviderSet = wire.NewSet(NewHandler, NewInitServers)
