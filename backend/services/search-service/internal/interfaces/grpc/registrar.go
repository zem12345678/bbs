package grpc

import (
	pb "search-service/api/proto/searchpb"
	iocgrpc "search-service/internal/ioc/grpc"

	"github.com/google/wire"
	stdgrpc "google.golang.org/grpc"
)

func NewInitServers(h *Handler) iocgrpc.InitServers {
	return func(s *stdgrpc.Server) {
		pb.RegisterSearchServiceServer(s, h)
	}
}

var ProviderSet = wire.NewSet(NewHandler, NewInitServers)
