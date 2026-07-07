package grpc

import (
	pb "content-service/api/proto/contentpb"
	iocgrpc "content-service/internal/ioc/grpc"

	"github.com/google/wire"
	stdgrpc "google.golang.org/grpc"
)

func NewInitServers(h *Handler) iocgrpc.InitServers {
	return func(s *stdgrpc.Server) {
		pb.RegisterContentServiceServer(s, h)
	}
}

var ProviderSet = wire.NewSet(NewHandler, NewInitServers)
