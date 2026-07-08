package clients

import (
	"fmt"

	iocgrpc "api-gateway/internal/ioc/grpc"

	"google.golang.org/grpc"
)

type Clients struct {
	Admin        AdminClient
	User         UserClient
	Content      ContentClient
	Comment      CommentClient
	Reaction     ReactionClient
	Search       SearchClient
	Feed         FeedClient
	Credit       CreditClient
	Mall         MallClient
	Notification NotificationClient

	conns []*grpc.ClientConn
}

func New(grpcClient *iocgrpc.Client, o Options) (*Clients, error) {
	if grpcClient == nil {
		return nil, fmt.Errorf("grpc client required")
	}
	o.applyDefaults()

	c := &Clients{}
	steps := []func(*iocgrpc.Client, Options) error{
		c.initAdmin,
		c.initUser,
		c.initContent,
		c.initComment,
		c.initReaction,
		c.initSearch,
		c.initFeed,
		c.initCredit,
		c.initMall,
		c.initNotification,
	}
	for _, step := range steps {
		if err := step(grpcClient, o); err != nil {
			_ = c.Close()
			return nil, err
		}
	}
	return c, nil
}

func (c *Clients) Close() error {
	var first error
	for _, conn := range c.conns {
		if err := conn.Close(); err != nil && first == nil {
			first = err
		}
	}
	return first
}

func (c *Clients) dial(grpcClient *iocgrpc.Client, service string, name string) (*grpc.ClientConn, error) {
	if service == "" {
		return nil, fmt.Errorf("%s upstream service required", name)
	}
	conn, err := grpcClient.Dial(service, false)
	if err != nil {
		return nil, fmt.Errorf("dial %s upstream %q: %w", name, service, err)
	}
	c.conns = append(c.conns, conn)
	return conn, nil
}
