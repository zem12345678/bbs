package content

import (
	"context"
	"fmt"
	"strings"

	"file-service/api/proto/contentpb"
	app "file-service/internal/application/file"
	"file-service/internal/clients/etcdresolver"
	domain "file-service/internal/domain/file"

	"github.com/spf13/viper"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type Client struct {
	client contentpb.ContentServiceClient
	close  func() error
}

const etcdResolverScheme = "file-content-etcd"

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

func NewClient(v *viper.Viper) (*Client, error) {
	token := strings.TrimSpace(v.GetString("upstreams.contentInternalAuthToken"))
	if token == "" {
		return nil, fmt.Errorf("content internal auth token required")
	}
	conn, err := etcdresolver.Dial(
		v.GetStringSlice("grpc.client.etcdAddr"),
		etcdResolverScheme,
		normalizeServiceName(v.GetString("upstreams.content")),
		"content",
		grpc.WithPerRPCCredentials(internalAuthCredentials{token: token}),
	)
	if err != nil {
		return nil, err
	}
	return &Client{client: contentpb.NewContentServiceClient(conn), close: conn.Close}, nil
}

func (c *Client) Close() error {
	if c == nil || c.close == nil {
		return nil
	}
	return c.close()
}

func (c *Client) GetTopic(ctx context.Context, topicID int64) (app.Topic, error) {
	response, err := c.client.GetTopic(ctx, &contentpb.GetTopicRequest{Key: &contentpb.GetTopicRequest_Id{Id: topicID}})
	if err != nil {
		return app.Topic{}, topicError(err)
	}
	topic := response.GetTopic()
	if topic == nil {
		return app.Topic{}, domain.ErrAttachmentTopicUnavailable
	}
	return app.Topic{ID: topic.GetId(), AuthorID: topic.GetAuthorId(), Status: topic.GetStatus()}, nil
}

func topicError(err error) error {
	if err == nil {
		return nil
	}
	switch status.Code(err) {
	case codes.NotFound:
		return domain.ErrAttachmentTopicUnavailable
	default:
		return domain.ErrContentServiceUnavailable
	}
}

func normalizeServiceName(value string) string {
	value = strings.TrimSpace(value)
	if value == "" || value == "content-service" {
		return "bbs-content-service"
	}
	return value
}

var _ app.TopicReader = (*Client)(nil)
