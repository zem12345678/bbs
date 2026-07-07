package grpc

import (
	pb "credit-service/api/proto/creditpb"
	iocgrpc "credit-service/internal/ioc/grpc"

	"github.com/google/wire"
	stdgrpc "google.golang.org/grpc"
)

func NewInitServers(h *Handler) iocgrpc.InitServers {
	return func(s *stdgrpc.Server) {
		pb.RegisterCreditServiceServer(s, h)
	}
}

var ProviderSet = wire.NewSet(NewHandler, NewInitServers)
