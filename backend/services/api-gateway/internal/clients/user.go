package clients

import (
	"context"
	"fmt"
	"strings"

	"api-gateway/api/proto/userpb"
	iocgrpc "api-gateway/internal/ioc/grpc"

	"google.golang.org/grpc"
)

const userInternalAuthMetadataKey = "x-bbs-internal-token"
const userMaxCallReceiveBytes = 32 << 20

type userInternalAuthCredentials struct {
	token  string
	secure bool
}

func (c userInternalAuthCredentials) GetRequestMetadata(context.Context, ...string) (map[string]string, error) {
	return map[string]string{userInternalAuthMetadataKey: c.token}, nil
}

func (c userInternalAuthCredentials) RequireTransportSecurity() bool {
	return c.secure
}

func (c *Clients) initUser(grpcClient *iocgrpc.Client, o Options) error {
	token := strings.TrimSpace(o.UserInternalAuthToken)
	if token == "" {
		return fmt.Errorf("user internal auth token required")
	}
	conn, err := c.dial(grpcClient, o.User, "user",
		iocgrpc.WithSecureConnection(o.UserInternalAuthSecure),
		iocgrpc.WithGrpcDialOptions(
			grpc.WithPerRPCCredentials(userInternalAuthCredentials{token: token, secure: o.UserInternalAuthSecure}),
			grpc.WithDefaultCallOptions(grpc.MaxCallRecvMsgSize(userMaxCallReceiveBytes)),
		),
	)
	if err != nil {
		return err
	}
	client := userpb.NewUserServiceClient(conn)
	c.User = client
	c.UserMemos = client
	c.UserCharts = client
	c.UserFollowingCharts = client
	c.UserActiveUsersCharts = client
	c.UserSafety = client
	c.UserLists = client
	c.UserAntennas = client
	c.UserMFA = client
	c.UserPasskeys = client
	c.UserAccountLifecycle = client
	c.UserInvites = client
	c.UserCredentialVersion = client
	c.UserSessions = client
	c.UserAPITokens = client
	c.UserRegistry = client
	return nil
}
