package erasure

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"user-service/api/proto/chatpb"
	"user-service/api/proto/commentpb"
	"user-service/api/proto/contentpb"
	"user-service/api/proto/creditpb"
	"user-service/api/proto/feedpb"
	"user-service/api/proto/filepb"
	"user-service/api/proto/mallpb"
	"user-service/api/proto/notificationpb"
	"user-service/api/proto/reactionpb"
	"user-service/api/proto/searchpb"
	"user-service/internal/application/user/deletion"
	iocgrpc "user-service/internal/ioc/grpc"

	"github.com/spf13/viper"
	"google.golang.org/grpc"
)

const internalAuthMetadataKey = "x-bbs-internal-token"

type internalAuthCredentials struct {
	token string
}

func (c internalAuthCredentials) GetRequestMetadata(context.Context, ...string) (map[string]string, error) {
	return map[string]string{internalAuthMetadataKey: c.token}, nil
}

func (internalAuthCredentials) RequireTransportSecurity() bool { return false }

type Eraser struct {
	service string
	erase   func(context.Context, int64, int64, int32) (bool, error)
}

func (e *Eraser) EraseUserData(ctx context.Context, userID, jobID int64, policyVersion int32) error {
	if e == nil || e.erase == nil || userID <= 0 || jobID <= 0 || policyVersion <= 0 {
		return fmt.Errorf("invalid %s account erasure request", e.serviceName())
	}
	completed, err := e.erase(ctx, userID, jobID, policyVersion)
	if err != nil {
		return err
	}
	if !completed {
		return fmt.Errorf("%s account erasure did not complete", e.serviceName())
	}
	return nil
}

func (e *Eraser) serviceName() string {
	if e == nil || strings.TrimSpace(e.service) == "" {
		return "downstream"
	}
	return e.service
}

type Set struct {
	Erasers     map[string]deletion.AccountDataEraser
	connections []*grpc.ClientConn
}

func NewSet(client *iocgrpc.Client, v *viper.Viper) (_ *Set, err error) {
	if client == nil || v == nil {
		return nil, fmt.Errorf("account erasure gRPC client configuration required")
	}
	set := &Set{Erasers: make(map[string]deletion.AccountDataEraser, 10)}
	defer func() {
		if err != nil {
			_ = set.Close()
		}
	}()

	contentConn, err := set.dial(client, v, "content", "bbs-content-service")
	if err != nil {
		return nil, err
	}
	set.Erasers["content-service"] = newContentEraser(contentpb.NewContentServiceClient(contentConn))

	commentConn, err := set.dial(client, v, "comment", "bbs-comment-service")
	if err != nil {
		return nil, err
	}
	set.Erasers["comment-service"] = newCommentEraser(commentpb.NewCommentServiceClient(commentConn))

	reactionConn, err := set.dial(client, v, "reaction", "bbs-reaction-service")
	if err != nil {
		return nil, err
	}
	set.Erasers["reaction-service"] = newReactionEraser(reactionpb.NewReactionServiceClient(reactionConn))

	chatConn, err := set.dial(client, v, "chat", "bbs-chat-service")
	if err != nil {
		return nil, err
	}
	set.Erasers["chat-service"] = newChatEraser(chatpb.NewChatServiceClient(chatConn))

	notificationConn, err := set.dial(client, v, "notification", "bbs-notification-service")
	if err != nil {
		return nil, err
	}
	set.Erasers["notification-service"] = newNotificationEraser(notificationpb.NewInternalNotificationServiceClient(notificationConn))

	fileConn, err := set.dial(client, v, "file", "bbs-file-service")
	if err != nil {
		return nil, err
	}
	set.Erasers["file-service"] = newFileEraser(filepb.NewFileServiceClient(fileConn))

	creditConn, err := set.dial(client, v, "credit", "bbs-credit-service")
	if err != nil {
		return nil, err
	}
	set.Erasers["credit-service"] = newCreditEraser(creditpb.NewCreditServiceClient(creditConn))

	mallConn, err := set.dial(client, v, "mall", "bbs-mall-service")
	if err != nil {
		return nil, err
	}
	set.Erasers["mall-service"] = newMallEraser(mallpb.NewMallServiceClient(mallConn))

	feedConn, err := set.dial(client, v, "feed", "bbs-feed-service")
	if err != nil {
		return nil, err
	}
	set.Erasers["feed-service"] = newFeedEraser(feedpb.NewFeedServiceClient(feedConn))

	searchConn, err := set.dial(client, v, "search", "bbs-search-service")
	if err != nil {
		return nil, err
	}
	set.Erasers["search-service"] = newSearchEraser(searchpb.NewSearchServiceClient(searchConn))
	return set, nil
}

func (s *Set) dial(client *iocgrpc.Client, v *viper.Viper, key, defaultService string) (*grpc.ClientConn, error) {
	service := strings.TrimSpace(v.GetString("upstreams." + key))
	if service == "" {
		service = defaultService
	}
	token := strings.TrimSpace(v.GetString("upstreams." + key + "InternalAuthToken"))
	if token == "" {
		return nil, fmt.Errorf("%s account erasure internal auth token required", key)
	}
	conn, err := client.Dial(service, false,
		iocgrpc.WithGrpcDialOptions(grpc.WithPerRPCCredentials(internalAuthCredentials{token: token})),
	)
	if err != nil {
		return nil, fmt.Errorf("dial %s account erasure service: %w", key, err)
	}
	s.connections = append(s.connections, conn)
	return conn, nil
}

func (s *Set) Close() error {
	if s == nil {
		return nil
	}
	closeErrors := make([]error, 0, len(s.connections))
	for index := len(s.connections) - 1; index >= 0; index-- {
		if err := s.connections[index].Close(); err != nil {
			closeErrors = append(closeErrors, err)
		}
	}
	s.connections = nil
	return errors.Join(closeErrors...)
}

type contentClient interface {
	ArchiveAccountContent(context.Context, *contentpb.ArchiveAccountContentRequest, ...grpc.CallOption) (*contentpb.ArchiveAccountContentResponse, error)
}

func newContentEraser(client contentClient) *Eraser {
	return &Eraser{service: "content-service", erase: func(ctx context.Context, userID, jobID int64, policyVersion int32) (bool, error) {
		response, err := client.ArchiveAccountContent(ctx, &contentpb.ArchiveAccountContentRequest{UserId: userID, DeletionJobId: jobID, PolicyVersion: policyVersion})
		return response != nil && response.GetCompleted(), err
	}}
}

type commentClient interface {
	RedactAccountComments(context.Context, *commentpb.RedactAccountCommentsRequest, ...grpc.CallOption) (*commentpb.RedactAccountCommentsResponse, error)
}

func newCommentEraser(client commentClient) *Eraser {
	return &Eraser{service: "comment-service", erase: func(ctx context.Context, userID, jobID int64, policyVersion int32) (bool, error) {
		response, err := client.RedactAccountComments(ctx, &commentpb.RedactAccountCommentsRequest{UserId: userID, DeletionJobId: jobID, PolicyVersion: policyVersion})
		return response != nil && response.GetCompleted(), err
	}}
}

type reactionClient interface {
	EraseAccountReactions(context.Context, *reactionpb.EraseAccountReactionsRequest, ...grpc.CallOption) (*reactionpb.EraseAccountReactionsResponse, error)
}

func newReactionEraser(client reactionClient) *Eraser {
	return &Eraser{service: "reaction-service", erase: func(ctx context.Context, userID, jobID int64, policyVersion int32) (bool, error) {
		response, err := client.EraseAccountReactions(ctx, &reactionpb.EraseAccountReactionsRequest{UserId: userID, DeletionJobId: jobID, PolicyVersion: policyVersion})
		return response != nil && response.GetCompleted(), err
	}}
}

type chatClient interface {
	EraseUserData(context.Context, *chatpb.EraseUserDataRequest, ...grpc.CallOption) (*chatpb.EraseUserDataResponse, error)
}

func newChatEraser(client chatClient) *Eraser {
	return &Eraser{service: "chat-service", erase: func(ctx context.Context, userID, jobID int64, policyVersion int32) (bool, error) {
		response, err := client.EraseUserData(ctx, &chatpb.EraseUserDataRequest{UserId: userID, DeletionJobId: jobID, PolicyVersion: policyVersion})
		return response != nil && response.GetCompleted(), err
	}}
}

type notificationClient interface {
	EraseUserData(context.Context, *notificationpb.EraseUserDataRequest, ...grpc.CallOption) (*notificationpb.MutationResponse, error)
}

func newNotificationEraser(client notificationClient) *Eraser {
	return &Eraser{service: "notification-service", erase: func(ctx context.Context, userID, jobID int64, policyVersion int32) (bool, error) {
		response, err := client.EraseUserData(ctx, &notificationpb.EraseUserDataRequest{UserId: userID, DeletionJobId: jobID, PolicyVersion: policyVersion})
		return response != nil && response.GetSuccess(), err
	}}
}

type feedClient interface {
	PurgeAccountFeed(context.Context, *feedpb.PurgeAccountFeedRequest, ...grpc.CallOption) (*feedpb.PurgeAccountFeedResponse, error)
}

type fileClient interface {
	EraseUserData(context.Context, *filepb.EraseUserDataRequest, ...grpc.CallOption) (*filepb.EraseUserDataResponse, error)
}

type creditClient interface {
	EraseUserData(context.Context, *creditpb.EraseUserDataRequest, ...grpc.CallOption) (*creditpb.EraseUserDataResponse, error)
}

type mallClient interface {
	EraseUserData(context.Context, *mallpb.EraseUserDataRequest, ...grpc.CallOption) (*mallpb.EraseUserDataResponse, error)
}

func newCreditEraser(client creditClient) *Eraser {
	return &Eraser{service: "credit-service", erase: func(ctx context.Context, userID, jobID int64, policyVersion int32) (bool, error) {
		response, err := client.EraseUserData(ctx, &creditpb.EraseUserDataRequest{UserId: userID, DeletionJobId: jobID, PolicyVersion: policyVersion})
		return response != nil && response.GetCompleted(), err
	}}
}

func newMallEraser(client mallClient) *Eraser {
	return &Eraser{service: "mall-service", erase: func(ctx context.Context, userID, jobID int64, policyVersion int32) (bool, error) {
		response, err := client.EraseUserData(ctx, &mallpb.EraseUserDataRequest{UserId: userID, DeletionJobId: jobID, PolicyVersion: policyVersion})
		return response != nil && response.GetCompleted(), err
	}}
}

func newFileEraser(client fileClient) *Eraser {
	return &Eraser{service: "file-service", erase: func(ctx context.Context, userID, jobID int64, policyVersion int32) (bool, error) {
		response, err := client.EraseUserData(ctx, &filepb.EraseUserDataRequest{UserId: userID, DeletionJobId: jobID, PolicyVersion: policyVersion})
		return response != nil && response.GetCompleted(), err
	}}
}

func newFeedEraser(client feedClient) *Eraser {
	return &Eraser{service: "feed-service", erase: func(ctx context.Context, userID, jobID int64, policyVersion int32) (bool, error) {
		response, err := client.PurgeAccountFeed(ctx, &feedpb.PurgeAccountFeedRequest{UserId: userID, DeletionJobId: jobID, PolicyVersion: policyVersion})
		return response != nil && response.GetCompleted(), err
	}}
}

type searchClient interface {
	EraseUserData(context.Context, *searchpb.EraseUserDataRequest, ...grpc.CallOption) (*searchpb.EraseUserDataResponse, error)
}

func newSearchEraser(client searchClient) *Eraser {
	return &Eraser{service: "search-service", erase: func(ctx context.Context, userID, jobID int64, policyVersion int32) (bool, error) {
		response, err := client.EraseUserData(ctx, &searchpb.EraseUserDataRequest{UserId: userID, DeletionJobId: jobID, PolicyVersion: policyVersion})
		return response != nil && response.GetCompleted(), err
	}}
}

var _ deletion.AccountDataEraser = (*Eraser)(nil)
