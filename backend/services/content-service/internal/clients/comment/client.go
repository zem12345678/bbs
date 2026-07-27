package comment

import (
	"context"
	"fmt"
	"strings"

	"content-service/api/proto/commentpb"
	topiccommand "content-service/internal/application/topic/command"
	topicdomain "content-service/internal/domain/topic"
	iocgrpc "content-service/internal/ioc/grpc"

	"github.com/spf13/viper"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

const internalAuthMetadataKey = "x-bbs-internal-token"

type internalAuthCredentials struct {
	token string
}

func (c internalAuthCredentials) GetRequestMetadata(context.Context, ...string) (map[string]string, error) {
	return map[string]string{internalAuthMetadataKey: c.token}, nil
}

func (internalAuthCredentials) RequireTransportSecurity() bool {
	return false
}

type Client struct {
	client commentpb.CommentServiceClient
}

func NewClient(grpcClient *iocgrpc.Client, v *viper.Viper) (*Client, error) {
	service := serviceName(v.GetString("upstreams.comment"))
	if service == "" {
		service = "bbs-comment-service"
	}
	token := strings.TrimSpace(v.GetString("upstreams.commentInternalAuthToken"))
	if token == "" {
		return nil, fmt.Errorf("comment internal auth token required")
	}
	conn, err := grpcClient.Dial(service, false,
		iocgrpc.WithGrpcDialOptions(grpc.WithPerRPCCredentials(internalAuthCredentials{token: token})),
	)
	if err != nil {
		return nil, err
	}
	return &Client{client: commentpb.NewCommentServiceClient(conn)}, nil
}

func serviceName(value string) string {
	value = strings.TrimSpace(value)
	if value == "comment-service" {
		return "bbs-comment-service"
	}
	return value
}

func (c *Client) GetComment(ctx context.Context, id int64) (topiccommand.CommentRef, error) {
	resp, err := c.client.GetComment(ctx, &commentpb.GetCommentRequest{Id: id})
	if err != nil {
		if status.Code(err) == codes.NotFound {
			return topiccommand.CommentRef{}, topicdomain.ErrCommentNotFound
		}
		return topiccommand.CommentRef{}, err
	}
	comment := resp.GetComment()
	if comment == nil || comment.GetId() <= 0 {
		return topiccommand.CommentRef{}, topicdomain.ErrCommentNotFound
	}
	return topiccommand.CommentRef{
		ID:         comment.GetId(),
		EntityType: comment.GetEntityType(),
		EntityID:   comment.GetEntityId(),
		AuthorID:   comment.GetAuthorId(),
		Status:     comment.GetStatus(),
	}, nil
}

var _ topiccommand.CommentReader = (*Client)(nil)
