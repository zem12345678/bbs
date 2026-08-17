package clients

import (
	"context"
	"fmt"
	"strings"

	"api-gateway/api/proto/notificationpb"
	iocgrpc "api-gateway/internal/ioc/grpc"

	"google.golang.org/grpc"
)

const notificationInternalAuthMetadataKey = "x-bbs-internal-token"

type notificationInternalAuthCredentials struct {
	token string
}

func (c notificationInternalAuthCredentials) GetRequestMetadata(context.Context, ...string) (map[string]string, error) {
	return map[string]string{notificationInternalAuthMetadataKey: c.token}, nil
}

func (notificationInternalAuthCredentials) RequireTransportSecurity() bool {
	return false
}

func (c *Clients) initNotification(grpcClient *iocgrpc.Client, o Options) error {
	token := strings.TrimSpace(o.NotificationInternalAuthToken)
	if token == "" {
		return fmt.Errorf("notification internal auth token required")
	}
	conn, err := c.dial(grpcClient, o.Notification, "notification",
		iocgrpc.WithGrpcDialOptions(grpc.WithPerRPCCredentials(notificationInternalAuthCredentials{token: token})),
	)
	if err != nil {
		return err
	}
	c.Notification = notificationpb.NewNotificationServiceClient(conn)
	c.NotificationInternal = notificationpb.NewInternalNotificationServiceClient(conn)
	return nil
}
