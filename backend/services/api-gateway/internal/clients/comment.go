package clients

import (
	"context"
	"fmt"
	"strings"

	"api-gateway/api/proto/commentpb"
	iocgrpc "api-gateway/internal/ioc/grpc"

	"google.golang.org/grpc"
)

const commentInternalAuthMetadataKey = "x-bbs-internal-token"

type commentInternalAuthCredentials struct {
	token string
}

func (c commentInternalAuthCredentials) GetRequestMetadata(context.Context, ...string) (map[string]string, error) {
	return map[string]string{commentInternalAuthMetadataKey: c.token}, nil
}

func (commentInternalAuthCredentials) RequireTransportSecurity() bool {
	return false
}

func (c *Clients) initComment(grpcClient *iocgrpc.Client, o Options) error {
	token := strings.TrimSpace(o.CommentInternalAuthToken)
	if token == "" {
		return fmt.Errorf("comment internal auth token required")
	}
	conn, err := c.dial(grpcClient, o.Comment, "comment",
		iocgrpc.WithGrpcDialOptions(grpc.WithPerRPCCredentials(commentInternalAuthCredentials{token: token})),
	)
	if err != nil {
		return err
	}
	c.Comment = commentpb.NewCommentServiceClient(conn)
	return nil
}
