package content

import (
	"context"
	"strings"

	"file-service/api/proto/contentpb"
	app "file-service/internal/application/file"
	domain "file-service/internal/domain/file"

	"github.com/spf13/viper"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type Client struct {
	client contentpb.ContentServiceClient
	close  func() error
}

func NewClient(v *viper.Viper) (*Client, error) {
	conn, err := dialEtcd(v.GetStringSlice("grpc.client.etcdAddr"), normalizeServiceName(v.GetString("upstreams.content")))
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
