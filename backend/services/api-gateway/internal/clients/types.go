package clients

import (
	"context"

	"api-gateway/api/proto/adminpb"
	"api-gateway/api/proto/chatpb"
	"api-gateway/api/proto/commentpb"
	"api-gateway/api/proto/contentpb"
	"api-gateway/api/proto/creditpb"
	"api-gateway/api/proto/feedpb"
	"api-gateway/api/proto/filepb"
	"api-gateway/api/proto/mallpb"
	"api-gateway/api/proto/notificationpb"
	"api-gateway/api/proto/reactionpb"
	"api-gateway/api/proto/searchpb"
	"api-gateway/api/proto/userpb"

	"google.golang.org/grpc"
)

type AdminClient = adminpb.AdminServiceClient

// UserClient is the public user RPC surface used by HTTP handlers. Keep the
// internal credential lookup separate so adding an internal security RPC does
// not widen every public-user test double.
type UserClient interface {
	Register(context.Context, *userpb.RegisterRequest, ...grpc.CallOption) (*userpb.AuthResponse, error)
	Login(context.Context, *userpb.LoginRequest, ...grpc.CallOption) (*userpb.AuthResponse, error)
	OAuthLogin(context.Context, *userpb.OAuthLoginRequest, ...grpc.CallOption) (*userpb.AuthResponse, error)
	WebmasterLogin(context.Context, *userpb.WebmasterLoginRequest, ...grpc.CallOption) (*userpb.AuthResponse, error)
	ListUsers(context.Context, *userpb.ListUsersRequest, ...grpc.CallOption) (*userpb.UserListResponse, error)
	GetUser(context.Context, *userpb.UserIDRequest, ...grpc.CallOption) (*userpb.UserResponse, error)
	GetUserByUsername(context.Context, *userpb.UsernameRequest, ...grpc.CallOption) (*userpb.UserResponse, error)
	UpdateProfile(context.Context, *userpb.UpdateProfileRequest, ...grpc.CallOption) (*userpb.UserResponse, error)
	UpdateStatus(context.Context, *userpb.UpdateStatusRequest, ...grpc.CallOption) (*userpb.UserResponse, error)
	ChangePassword(context.Context, *userpb.ChangePasswordRequest, ...grpc.CallOption) (*userpb.SimpleResponse, error)
	RequestPasswordReset(context.Context, *userpb.PasswordResetRequest, ...grpc.CallOption) (*userpb.PasswordResetResponse, error)
	ResetPassword(context.Context, *userpb.ResetPasswordRequest, ...grpc.CallOption) (*userpb.SimpleResponse, error)
	RequestEmailVerification(context.Context, *userpb.EmailVerificationRequest, ...grpc.CallOption) (*userpb.EmailVerificationResponse, error)
	VerifyEmail(context.Context, *userpb.VerifyEmailRequest, ...grpc.CallOption) (*userpb.UserResponse, error)
	Follow(context.Context, *userpb.FollowRequest, ...grpc.CallOption) (*userpb.SimpleResponse, error)
	Unfollow(context.Context, *userpb.FollowRequest, ...grpc.CallOption) (*userpb.SimpleResponse, error)
	IsFollowing(context.Context, *userpb.FollowRequest, ...grpc.CallOption) (*userpb.IsFollowingResponse, error)
	ListFollowers(context.Context, *userpb.ListFollowsRequest, ...grpc.CallOption) (*userpb.UserListResponse, error)
	ListFollowing(context.Context, *userpb.ListFollowsRequest, ...grpc.CallOption) (*userpb.UserListResponse, error)
}

type UserCredentialVersionClient interface {
	GetCredentialVersion(context.Context, *userpb.UserIDRequest, ...grpc.CallOption) (*userpb.CredentialVersionResponse, error)
}

type ContentClient = contentpb.ContentServiceClient
type CommentClient = commentpb.CommentServiceClient
type ReactionClient = reactionpb.ReactionServiceClient
type SearchClient = searchpb.SearchServiceClient
type FeedClient = feedpb.FeedServiceClient
type CreditClient = creditpb.CreditServiceClient
type MallClient = mallpb.MallServiceClient
type NotificationClient = notificationpb.NotificationServiceClient
type FileClient = filepb.FileServiceClient
type ChatClient = chatpb.ChatServiceClient
