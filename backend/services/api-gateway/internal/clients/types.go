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
	Follow(context.Context, *userpb.FollowRequest, ...grpc.CallOption) (*userpb.FollowResponse, error)
	Unfollow(context.Context, *userpb.FollowRequest, ...grpc.CallOption) (*userpb.SimpleResponse, error)
	IsFollowing(context.Context, *userpb.FollowRequest, ...grpc.CallOption) (*userpb.IsFollowingResponse, error)
	ListFollowers(context.Context, *userpb.ListFollowsRequest, ...grpc.CallOption) (*userpb.UserListResponse, error)
	ListFollowing(context.Context, *userpb.ListFollowsRequest, ...grpc.CallOption) (*userpb.UserListResponse, error)
	ListReceivedFollowRequests(context.Context, *userpb.ListFollowRequestsRequest, ...grpc.CallOption) (*userpb.FollowRequestListResponse, error)
	ListSentFollowRequests(context.Context, *userpb.ListFollowRequestsRequest, ...grpc.CallOption) (*userpb.FollowRequestListResponse, error)
	AcceptFollowRequest(context.Context, *userpb.FollowRequestActionRequest, ...grpc.CallOption) (*userpb.SimpleResponse, error)
	RejectFollowRequest(context.Context, *userpb.FollowRequestActionRequest, ...grpc.CallOption) (*userpb.SimpleResponse, error)
	CancelFollowRequest(context.Context, *userpb.FollowRequestActionRequest, ...grpc.CallOption) (*userpb.SimpleResponse, error)
	SetFollowApprovalRequired(context.Context, *userpb.SetFollowApprovalRequest, ...grpc.CallOption) (*userpb.SimpleResponse, error)
}

type UserFollowingClient interface {
	GetFollowing(context.Context, *userpb.GetFollowingRequest, ...grpc.CallOption) (*userpb.FollowingResponse, error)
	UpdateFollowing(context.Context, *userpb.UpdateFollowingRequest, ...grpc.CallOption) (*userpb.FollowingResponse, error)
	UpdateAllFollowings(context.Context, *userpb.UpdateAllFollowingsRequest, ...grpc.CallOption) (*userpb.SimpleResponse, error)
	ListFollowingEdges(context.Context, *userpb.ListFollowingEdgesRequest, ...grpc.CallOption) (*userpb.FollowingListResponse, error)
	ListFollowerEdges(context.Context, *userpb.ListFollowingEdgesRequest, ...grpc.CallOption) (*userpb.FollowingListResponse, error)
}

type UserMemoClient interface {
	UpdateUserMemo(context.Context, *userpb.UpdateUserMemoRequest, ...grpc.CallOption) (*userpb.SimpleResponse, error)
	GetUserMemo(context.Context, *userpb.GetUserMemoRequest, ...grpc.CallOption) (*userpb.UserMemoResponse, error)
}

// UserChartClient is kept separate from UserClient so chart additions do not
// widen the public profile test surface.
type UserChartClient interface {
	GetUserChart(context.Context, *userpb.UserChartRequest, ...grpc.CallOption) (*userpb.UserChartResponse, error)
}

type UserFollowingChartClient interface {
	GetUserFollowingChart(context.Context, *userpb.UserFollowingChartRequest, ...grpc.CallOption) (*userpb.UserFollowingChartResponse, error)
}

type UserActiveUsersChartClient interface {
	GetActiveUsersChart(context.Context, *userpb.ActiveUsersChartRequest, ...grpc.CallOption) (*userpb.ActiveUsersChartResponse, error)
}

type UserSafetyClient interface {
	Block(context.Context, *userpb.UserRelationRequest, ...grpc.CallOption) (*userpb.SimpleResponse, error)
	Unblock(context.Context, *userpb.UserRelationRequest, ...grpc.CallOption) (*userpb.SimpleResponse, error)
	Mute(context.Context, *userpb.UserRelationRequest, ...grpc.CallOption) (*userpb.SimpleResponse, error)
	Unmute(context.Context, *userpb.UserRelationRequest, ...grpc.CallOption) (*userpb.SimpleResponse, error)
	GetSafetyRelation(context.Context, *userpb.UserRelationRequest, ...grpc.CallOption) (*userpb.SafetyRelationResponse, error)
	ListBlockedUsers(context.Context, *userpb.ListUserRelationsRequest, ...grpc.CallOption) (*userpb.UserListResponse, error)
	ListMutedUsers(context.Context, *userpb.ListUserRelationsRequest, ...grpc.CallOption) (*userpb.UserListResponse, error)
}

type UserListClient interface {
	CreateUserList(context.Context, *userpb.CreateUserListRequest, ...grpc.CallOption) (*userpb.UserListInfoResponse, error)
	UpdateUserList(context.Context, *userpb.UpdateUserListRequest, ...grpc.CallOption) (*userpb.UserListInfoResponse, error)
	DeleteUserList(context.Context, *userpb.DeleteUserListRequest, ...grpc.CallOption) (*userpb.SimpleResponse, error)
	GetUserList(context.Context, *userpb.GetUserListRequest, ...grpc.CallOption) (*userpb.UserListInfoResponse, error)
	ListUserLists(context.Context, *userpb.ListUserListsRequest, ...grpc.CallOption) (*userpb.UserListsResponse, error)
	ListFavoriteUserLists(context.Context, *userpb.ListFavoriteUserListsRequest, ...grpc.CallOption) (*userpb.UserListsResponse, error)
	AddUserListMember(context.Context, *userpb.UserListMemberRequest, ...grpc.CallOption) (*userpb.SimpleResponse, error)
	RemoveUserListMember(context.Context, *userpb.UserListMemberRequest, ...grpc.CallOption) (*userpb.SimpleResponse, error)
	ListUserListMembers(context.Context, *userpb.ListUserListMembersRequest, ...grpc.CallOption) (*userpb.UserListResponse, error)
	CopyUserList(context.Context, *userpb.CopyUserListRequest, ...grpc.CallOption) (*userpb.UserListInfoResponse, error)
	FavoriteUserList(context.Context, *userpb.UserListFavoriteRequest, ...grpc.CallOption) (*userpb.UserListInfoResponse, error)
	UnfavoriteUserList(context.Context, *userpb.UserListFavoriteRequest, ...grpc.CallOption) (*userpb.UserListInfoResponse, error)
}

type UserAntennaClient interface {
	CreateAntenna(context.Context, *userpb.CreateAntennaRequest, ...grpc.CallOption) (*userpb.AntennaInfoResponse, error)
	UpdateAntenna(context.Context, *userpb.UpdateAntennaRequest, ...grpc.CallOption) (*userpb.AntennaInfoResponse, error)
	DeleteAntenna(context.Context, *userpb.DeleteAntennaRequest, ...grpc.CallOption) (*userpb.SimpleResponse, error)
	GetAntenna(context.Context, *userpb.GetAntennaRequest, ...grpc.CallOption) (*userpb.AntennaInfoResponse, error)
	ListAntennas(context.Context, *userpb.ListAntennasRequest, ...grpc.CallOption) (*userpb.AntennaListResponse, error)
}

type UserMFAClient interface {
	GetMFAStatus(context.Context, *userpb.UserIDRequest, ...grpc.CallOption) (*userpb.MFAStatusResponse, error)
	BeginTOTPEnrollment(context.Context, *userpb.BeginTOTPEnrollmentRequest, ...grpc.CallOption) (*userpb.TOTPEnrollmentResponse, error)
	ConfirmTOTPEnrollment(context.Context, *userpb.ConfirmTOTPEnrollmentRequest, ...grpc.CallOption) (*userpb.MFARecoveryCodesResponse, error)
	RegenerateMFARecoveryCodes(context.Context, *userpb.MFAReauthenticateRequest, ...grpc.CallOption) (*userpb.MFARecoveryCodesResponse, error)
	DisableTOTP(context.Context, *userpb.MFAReauthenticateRequest, ...grpc.CallOption) (*userpb.SimpleResponse, error)
	CompleteMFALogin(context.Context, *userpb.CompleteMFALoginRequest, ...grpc.CallOption) (*userpb.AuthResponse, error)
}

type UserPasskeyClient interface {
	ListPasskeys(context.Context, *userpb.UserIDRequest, ...grpc.CallOption) (*userpb.PasskeyListResponse, error)
	BeginPasskeyRegistration(context.Context, *userpb.BeginPasskeyRegistrationRequest, ...grpc.CallOption) (*userpb.PasskeyOptionsResponse, error)
	FinishPasskeyRegistration(context.Context, *userpb.FinishPasskeyRegistrationRequest, ...grpc.CallOption) (*userpb.PasskeyInfoResponse, error)
	UpdatePasskey(context.Context, *userpb.UpdatePasskeyRequest, ...grpc.CallOption) (*userpb.PasskeyInfoResponse, error)
	DeletePasskey(context.Context, *userpb.DeletePasskeyRequest, ...grpc.CallOption) (*userpb.SimpleResponse, error)
	SetPasskeyPasswordless(context.Context, *userpb.SetPasskeyPasswordlessRequest, ...grpc.CallOption) (*userpb.SimpleResponse, error)
	BeginPasskeyMFALogin(context.Context, *userpb.BeginPasskeyMFALoginRequest, ...grpc.CallOption) (*userpb.PasskeyOptionsResponse, error)
	CompletePasskeyMFALogin(context.Context, *userpb.CompletePasskeyLoginRequest, ...grpc.CallOption) (*userpb.AuthResponse, error)
	BeginPasswordlessPasskeyLogin(context.Context, *userpb.PasswordlessPasskeyOptionsRequest, ...grpc.CallOption) (*userpb.PasskeyOptionsResponse, error)
	CompletePasswordlessPasskeyLogin(context.Context, *userpb.CompletePasskeyLoginRequest, ...grpc.CallOption) (*userpb.AuthResponse, error)
}

type UserAccountLifecycleClient interface {
	GetAccountLifecycle(context.Context, *userpb.UserIDRequest, ...grpc.CallOption) (*userpb.AccountLifecycleResponse, error)
	RequestAccountDeletion(context.Context, *userpb.RequestAccountDeletionRequest, ...grpc.CallOption) (*userpb.AccountLifecycleResponse, error)
}

// UserInviteClient is the administrative invite-code RPC surface. Keep it
// separate from UserClient so existing public-user test doubles do not need
// to implement admin-only operations.
type UserInviteClient interface {
	CreateInviteCodes(context.Context, *userpb.CreateInviteCodesRequest, ...grpc.CallOption) (*userpb.InviteCodeListResponse, error)
	ListInviteCodes(context.Context, *userpb.ListInviteCodesRequest, ...grpc.CallOption) (*userpb.InviteCodeListResponse, error)
	RevokeInviteCode(context.Context, *userpb.RevokeInviteCodeRequest, ...grpc.CallOption) (*userpb.SimpleResponse, error)
}

// UserSessionClient is the authenticated-user session and login-history RPC
// surface. It stays separate from UserClient so existing test doubles do not
// need to implement session operations.
type UserSessionClient interface {
	ListSessions(context.Context, *userpb.ListSessionsRequest, ...grpc.CallOption) (*userpb.SessionListResponse, error)
	GetSession(context.Context, *userpb.GetSessionRequest, ...grpc.CallOption) (*userpb.SessionResponse, error)
	RevokeSession(context.Context, *userpb.RevokeSessionRequest, ...grpc.CallOption) (*userpb.SessionResponse, error)
	ListLoginEvents(context.Context, *userpb.ListLoginEventsRequest, ...grpc.CallOption) (*userpb.LoginEventListResponse, error)
}

type UserAPITokenClient interface {
	CreateAPIToken(context.Context, *userpb.CreateAPITokenRequest, ...grpc.CallOption) (*userpb.CreateAPITokenResponse, error)
	ListAPITokens(context.Context, *userpb.ListAPITokensRequest, ...grpc.CallOption) (*userpb.APITokenListResponse, error)
	RevokeAPIToken(context.Context, *userpb.RevokeAPITokenRequest, ...grpc.CallOption) (*userpb.APITokenResponse, error)
}

type UserRegistryClient interface {
	SetRegistryItem(context.Context, *userpb.SetRegistryItemRequest, ...grpc.CallOption) (*userpb.RegistryItemResponse, error)
	GetRegistryItem(context.Context, *userpb.GetRegistryItemRequest, ...grpc.CallOption) (*userpb.RegistryItemResponse, error)
	ListRegistryItems(context.Context, *userpb.ListRegistryItemsRequest, ...grpc.CallOption) (*userpb.RegistryItemListResponse, error)
	RemoveRegistryItem(context.Context, *userpb.GetRegistryItemRequest, ...grpc.CallOption) (*userpb.SimpleResponse, error)
	ListRegistryScopeDomains(context.Context, *userpb.UserIDRequest, ...grpc.CallOption) (*userpb.RegistryScopeDomainListResponse, error)
}

type UserCredentialVersionClient interface {
	GetCredentialVersion(context.Context, *userpb.UserIDRequest, ...grpc.CallOption) (*userpb.CredentialVersionResponse, error)
}

type ContentClient = contentpb.ContentServiceClient
type CommentClient = commentpb.CommentServiceClient
type ReactionClient = reactionpb.ReactionServiceClient
type SearchClient = searchpb.SearchServiceClient
type FeedClient interface {
	ListLatest(context.Context, *feedpb.ListFeedRequest, ...grpc.CallOption) (*feedpb.FeedListResponse, error)
	ListHot(context.Context, *feedpb.ListFeedRequest, ...grpc.CallOption) (*feedpb.FeedListResponse, error)
	ListActive(context.Context, *feedpb.ListFeedRequest, ...grpc.CallOption) (*feedpb.FeedListResponse, error)
}

type FeedFilteredClient interface {
	ListFiltered(context.Context, *feedpb.FilteredFeedRequest, ...grpc.CallOption) (*feedpb.FeedListResponse, error)
}
type CreditClient = creditpb.CreditServiceClient
type MallClient = mallpb.MallServiceClient
type NotificationClient = notificationpb.NotificationServiceClient
type NotificationInternalClient = notificationpb.InternalNotificationServiceClient
type FileClient = filepb.FileServiceClient
type ChatClient = chatpb.ChatServiceClient
