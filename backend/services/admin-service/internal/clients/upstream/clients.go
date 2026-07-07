package upstream

import (
	"fmt"

	"admin/api/proto/commentpb"
	"admin/api/proto/contentpb"
	"admin/api/proto/reactionpb"
	"admin/api/proto/userpb"
	iocgrpc "admin/internal/ioc/grpc"

	"google.golang.org/grpc"
)

type Options struct {
	User     string
	Reaction string
	Content  string
	Comment  string
}

type Clients struct {
	user     userpb.UserServiceClient
	reaction reactionpb.ReactionServiceClient
	content  contentpb.ContentServiceClient
	comment  commentpb.CommentServiceClient
	conns    []*grpc.ClientConn
}

func New(client *iocgrpc.Client, o Options) (*Clients, error) {
	userConn, err := dial(client, o.User, "user")
	if err != nil {
		return nil, err
	}
	reactionConn, err := dial(client, o.Reaction, "reaction")
	if err != nil {
		_ = userConn.Close()
		return nil, err
	}
	contentConn, err := dial(client, o.Content, "content")
	if err != nil {
		_ = userConn.Close()
		_ = reactionConn.Close()
		return nil, err
	}
	commentConn, err := dial(client, o.Comment, "comment")
	if err != nil {
		_ = userConn.Close()
		_ = reactionConn.Close()
		_ = contentConn.Close()
		return nil, err
	}
	return &Clients{
		user:     userpb.NewUserServiceClient(userConn),
		reaction: reactionpb.NewReactionServiceClient(reactionConn),
		content:  contentpb.NewContentServiceClient(contentConn),
		comment:  commentpb.NewCommentServiceClient(commentConn),
		conns:    []*grpc.ClientConn{userConn, reactionConn, contentConn, commentConn},
	}, nil
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

func dial(client *iocgrpc.Client, serviceName string, name string) (*grpc.ClientConn, error) {
	if client == nil {
		return nil, fmt.Errorf("%s grpc client required", name)
	}
	if serviceName == "" {
		return nil, fmt.Errorf("%s upstream required", name)
	}
	return client.Dial(serviceName, false)
}
