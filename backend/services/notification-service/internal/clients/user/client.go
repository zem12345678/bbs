package user

import (
	"context"
	"fmt"
	"strings"

	"notification-service/api/proto/userpb"
	iocgrpc "notification-service/internal/ioc/grpc"

	"github.com/spf13/viper"
	"google.golang.org/grpc"
)

const internalAuthMetadataKey = "x-bbs-internal-token"

type internalAuthCredentials struct {
	token string
}

func (c internalAuthCredentials) GetRequestMetadata(context.Context, ...string) (map[string]string, error) {
	return map[string]string{internalAuthMetadataKey: c.token}, nil
}

func (internalAuthCredentials) RequireTransportSecurity() bool { return false }

type Client struct {
	client userServiceClient
	conn   *grpc.ClientConn
}

type userServiceClient interface {
	ListNoteNotificationSubscribers(context.Context, *userpb.ListNoteNotificationSubscribersRequest, ...grpc.CallOption) (*userpb.NoteNotificationSubscribersResponse, error)
}

func NewClient(grpcClient *iocgrpc.Client, v *viper.Viper) (*Client, error) {
	service := strings.TrimSpace(v.GetString("upstreams.user"))
	if service == "" {
		service = "bbs-user-service"
	}
	token := strings.TrimSpace(v.GetString("upstreams.userInternalAuthToken"))
	if token == "" {
		return nil, fmt.Errorf("user internal auth token required")
	}
	conn, err := grpcClient.Dial(service, false,
		iocgrpc.WithGrpcDialOptions(grpc.WithPerRPCCredentials(internalAuthCredentials{token: token})),
	)
	if err != nil {
		return nil, err
	}
	return &Client{client: userpb.NewUserServiceClient(conn), conn: conn}, nil
}

func (c *Client) Close() error {
	if c == nil || c.conn == nil {
		return nil
	}
	err := c.conn.Close()
	c.conn = nil
	return err
}

func (c *Client) ListNoteNotificationSubscribers(ctx context.Context, req *userpb.ListNoteNotificationSubscribersRequest, opts ...grpc.CallOption) (*userpb.NoteNotificationSubscribersResponse, error) {
	if c == nil || c.client == nil {
		return nil, fmt.Errorf("user client unavailable")
	}
	return c.client.ListNoteNotificationSubscribers(ctx, req, opts...)
}

var _ userServiceClient = (*Client)(nil)
