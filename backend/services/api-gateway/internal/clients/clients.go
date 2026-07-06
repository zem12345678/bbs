package clients

import (
	"fmt"

	"api-gateway/internal/clients/pb/adminpb"
	"api-gateway/internal/clients/pb/commentpb"
	"api-gateway/internal/clients/pb/contentpb"
	"api-gateway/internal/clients/pb/creditpb"
	"api-gateway/internal/clients/pb/feedpb"
	"api-gateway/internal/clients/pb/notificationpb"
	"api-gateway/internal/clients/pb/reactionpb"
	"api-gateway/internal/clients/pb/searchpb"
	"api-gateway/internal/clients/pb/userpb"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

type Options struct {
	Admin        string
	User         string
	Content      string
	Comment      string
	Reaction     string
	Search       string
	Feed         string
	Credit       string
	Notification string
}

type Clients struct {
	Admin        adminpb.AdminServiceClient
	User         userpb.UserServiceClient
	Content      contentpb.ContentServiceClient
	Comment      commentpb.CommentServiceClient
	Reaction     reactionpb.ReactionServiceClient
	Search       searchpb.SearchServiceClient
	Feed         feedpb.FeedServiceClient
	Credit       creditpb.CreditServiceClient
	Notification notificationpb.NotificationServiceClient

	conns []*grpc.ClientConn
}

func New(o Options) (*Clients, error) {
	adminConn, err := dial(o.Admin, "admin")
	if err != nil {
		return nil, err
	}
	userConn, err := dial(o.User, "user")
	if err != nil {
		_ = adminConn.Close()
		return nil, err
	}
	contentConn, err := dial(o.Content, "content")
	if err != nil {
		_ = adminConn.Close()
		_ = userConn.Close()
		return nil, err
	}
	commentConn, err := dial(o.Comment, "comment")
	if err != nil {
		_ = adminConn.Close()
		_ = userConn.Close()
		_ = contentConn.Close()
		return nil, err
	}
	reactionConn, err := dial(o.Reaction, "reaction")
	if err != nil {
		_ = adminConn.Close()
		_ = userConn.Close()
		_ = contentConn.Close()
		_ = commentConn.Close()
		return nil, err
	}
	searchConn, err := dial(o.Search, "search")
	if err != nil {
		_ = adminConn.Close()
		_ = userConn.Close()
		_ = contentConn.Close()
		_ = commentConn.Close()
		_ = reactionConn.Close()
		return nil, err
	}
	feedConn, err := dial(o.Feed, "feed")
	if err != nil {
		_ = adminConn.Close()
		_ = userConn.Close()
		_ = contentConn.Close()
		_ = commentConn.Close()
		_ = reactionConn.Close()
		_ = searchConn.Close()
		return nil, err
	}
	creditConn, err := dial(o.Credit, "credit")
	if err != nil {
		_ = adminConn.Close()
		_ = userConn.Close()
		_ = contentConn.Close()
		_ = commentConn.Close()
		_ = reactionConn.Close()
		_ = searchConn.Close()
		_ = feedConn.Close()
		return nil, err
	}
	notificationConn, err := dial(o.Notification, "notification")
	if err != nil {
		_ = adminConn.Close()
		_ = userConn.Close()
		_ = contentConn.Close()
		_ = commentConn.Close()
		_ = reactionConn.Close()
		_ = searchConn.Close()
		_ = feedConn.Close()
		_ = creditConn.Close()
		return nil, err
	}
	return &Clients{
		Admin:        adminpb.NewAdminServiceClient(adminConn),
		User:         userpb.NewUserServiceClient(userConn),
		Content:      contentpb.NewContentServiceClient(contentConn),
		Comment:      commentpb.NewCommentServiceClient(commentConn),
		Reaction:     reactionpb.NewReactionServiceClient(reactionConn),
		Search:       searchpb.NewSearchServiceClient(searchConn),
		Feed:         feedpb.NewFeedServiceClient(feedConn),
		Credit:       creditpb.NewCreditServiceClient(creditConn),
		Notification: notificationpb.NewNotificationServiceClient(notificationConn),
		conns:        []*grpc.ClientConn{adminConn, userConn, contentConn, commentConn, reactionConn, searchConn, feedConn, creditConn, notificationConn},
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

func dial(addr string, name string) (*grpc.ClientConn, error) {
	if addr == "" {
		return nil, fmt.Errorf("%s upstream required", name)
	}
	return grpc.NewClient(addr, grpc.WithTransportCredentials(insecure.NewCredentials()))
}
