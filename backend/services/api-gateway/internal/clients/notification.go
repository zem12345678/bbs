package clients

import (
	"api-gateway/api/proto/notificationpb"
	iocgrpc "api-gateway/internal/ioc/grpc"
)

func (c *Clients) initNotification(grpcClient *iocgrpc.Client, o Options) error {
	conn, err := c.dial(grpcClient, o.Notification, "notification")
	if err != nil {
		return err
	}
	c.Notification = notificationpb.NewNotificationServiceClient(conn)
	return nil
}
