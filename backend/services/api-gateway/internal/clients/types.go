package clients

import (
	"api-gateway/api/proto/adminpb"
	"api-gateway/api/proto/commentpb"
	"api-gateway/api/proto/contentpb"
	"api-gateway/api/proto/creditpb"
	"api-gateway/api/proto/feedpb"
	"api-gateway/api/proto/mallpb"
	"api-gateway/api/proto/notificationpb"
	"api-gateway/api/proto/reactionpb"
	"api-gateway/api/proto/searchpb"
	"api-gateway/api/proto/userpb"
)

type AdminClient = adminpb.AdminServiceClient
type UserClient = userpb.UserServiceClient
type ContentClient = contentpb.ContentServiceClient
type CommentClient = commentpb.CommentServiceClient
type ReactionClient = reactionpb.ReactionServiceClient
type SearchClient = searchpb.SearchServiceClient
type FeedClient = feedpb.FeedServiceClient
type CreditClient = creditpb.CreditServiceClient
type MallClient = mallpb.MallServiceClient
type NotificationClient = notificationpb.NotificationServiceClient
