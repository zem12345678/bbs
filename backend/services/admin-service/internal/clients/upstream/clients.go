package upstream

import (
	"context"
	"fmt"
	"strings"

	"admin/api/proto/commentpb"
	"admin/api/proto/contentpb"
	"admin/api/proto/notificationpb"
	"admin/api/proto/reactionpb"
	"admin/api/proto/searchpb"
	"admin/api/proto/userpb"
	iocgrpc "admin/internal/ioc/grpc"

	"google.golang.org/grpc"
)

type Options struct {
	User                          string
	UserInternalAuthToken         string
	Reaction                      string
	ReactionInternalAuthToken     string
	Content                       string
	ContentInternalAuthToken      string
	Comment                       string
	CommentInternalAuthToken      string
	Notification                  string
	NotificationInternalAuthToken string
	Search                        string
	SearchInternalAuthToken       string
}

type Clients struct {
	user         userpb.UserServiceClient
	reaction     reactionpb.ReactionServiceClient
	content      contentpb.ContentServiceClient
	comment      commentpb.CommentServiceClient
	notification notificationpb.InternalNotificationServiceClient
	search       searchpb.SearchServiceClient
	conns        []*grpc.ClientConn
}

func New(client *iocgrpc.Client, o Options) (*Clients, error) {
	userToken := strings.TrimSpace(o.UserInternalAuthToken)
	if userToken == "" {
		return nil, fmt.Errorf("user internal auth token required")
	}
	reactionToken := strings.TrimSpace(o.ReactionInternalAuthToken)
	if reactionToken == "" {
		return nil, fmt.Errorf("reaction internal auth token required")
	}
	notificationToken := strings.TrimSpace(o.NotificationInternalAuthToken)
	if notificationToken == "" {
		return nil, fmt.Errorf("notification internal auth token required")
	}
	searchToken := strings.TrimSpace(o.SearchInternalAuthToken)
	if searchToken == "" {
		return nil, fmt.Errorf("search internal auth token required")
	}
	commentToken := strings.TrimSpace(o.CommentInternalAuthToken)
	if commentToken == "" {
		return nil, fmt.Errorf("comment internal auth token required")
	}
	contentToken := strings.TrimSpace(o.ContentInternalAuthToken)
	if contentToken == "" {
		return nil, fmt.Errorf("content internal auth token required")
	}
	userConn, err := dial(client, o.User, "user",
		iocgrpc.WithGrpcDialOptions(grpc.WithPerRPCCredentials(internalAuthCredentials{token: userToken})),
	)
	if err != nil {
		return nil, err
	}
	reactionConn, err := dial(client, o.Reaction, "reaction",
		iocgrpc.WithGrpcDialOptions(grpc.WithPerRPCCredentials(internalAuthCredentials{token: reactionToken})),
	)
	if err != nil {
		_ = userConn.Close()
		return nil, err
	}
	contentConn, err := dial(client, o.Content, "content",
		iocgrpc.WithGrpcDialOptions(grpc.WithPerRPCCredentials(internalAuthCredentials{token: contentToken})),
	)
	if err != nil {
		_ = userConn.Close()
		_ = reactionConn.Close()
		return nil, err
	}
	commentConn, err := dial(client, o.Comment, "comment",
		iocgrpc.WithGrpcDialOptions(grpc.WithPerRPCCredentials(internalAuthCredentials{token: commentToken})),
	)
	if err != nil {
		_ = userConn.Close()
		_ = reactionConn.Close()
		_ = contentConn.Close()
		return nil, err
	}
	notificationConn, err := dial(client, o.Notification, "notification",
		iocgrpc.WithGrpcDialOptions(grpc.WithPerRPCCredentials(internalAuthCredentials{token: notificationToken})),
	)
	if err != nil {
		_ = userConn.Close()
		_ = reactionConn.Close()
		_ = contentConn.Close()
		_ = commentConn.Close()
		return nil, err
	}
	searchConn, err := dial(client, o.Search, "search",
		iocgrpc.WithGrpcDialOptions(grpc.WithPerRPCCredentials(internalAuthCredentials{token: searchToken})),
	)
	if err != nil {
		_ = userConn.Close()
		_ = reactionConn.Close()
		_ = contentConn.Close()
		_ = commentConn.Close()
		_ = notificationConn.Close()
		return nil, err
	}
	return &Clients{
		user:         userpb.NewUserServiceClient(userConn),
		reaction:     reactionpb.NewReactionServiceClient(reactionConn),
		content:      contentpb.NewContentServiceClient(contentConn),
		comment:      commentpb.NewCommentServiceClient(commentConn),
		notification: notificationpb.NewInternalNotificationServiceClient(notificationConn),
		search:       searchpb.NewSearchServiceClient(searchConn),
		conns:        []*grpc.ClientConn{userConn, reactionConn, contentConn, commentConn, notificationConn, searchConn},
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

func dial(client *iocgrpc.Client, serviceName string, name string, options ...iocgrpc.ClientOptional) (*grpc.ClientConn, error) {
	if client == nil {
		return nil, fmt.Errorf("%s grpc client required", name)
	}
	if serviceName == "" {
		return nil, fmt.Errorf("%s upstream required", name)
	}
	return client.Dial(serviceName, false, options...)
}

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
