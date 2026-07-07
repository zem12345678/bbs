package grpc

import (
	pb "comment-service/api/proto/commentpb"
	iocgrpc "comment-service/internal/ioc/grpc"

	"github.com/google/wire"
	stdgrpc "google.golang.org/grpc"
)

func NewInitServers(h *Handler) iocgrpc.InitServers {
	return func(s *stdgrpc.Server) {
		pb.RegisterCommentServiceServer(s, h)
	}
}

var ProviderSet = wire.NewSet(NewHandler, NewInitServers)
