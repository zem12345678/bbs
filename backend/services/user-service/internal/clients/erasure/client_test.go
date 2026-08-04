package erasure

import (
	"context"
	"errors"
	"testing"

	"user-service/api/proto/chatpb"
	"user-service/api/proto/commentpb"
	"user-service/api/proto/contentpb"
	"user-service/api/proto/creditpb"
	"user-service/api/proto/feedpb"
	"user-service/api/proto/filepb"
	"user-service/api/proto/notificationpb"
	"user-service/api/proto/reactionpb"
	"user-service/api/proto/searchpb"

	"google.golang.org/grpc"
)

func TestDownstreamErasersMapIdentityAndCompletion(t *testing.T) {
	tests := []struct {
		name   string
		eraser *Eraser
	}{
		{
			name: "content",
			eraser: newContentEraser(&fakeContentClient{call: func(req *contentpb.ArchiveAccountContentRequest) (*contentpb.ArchiveAccountContentResponse, error) {
				assertRequestIdentity(t, req.GetUserId(), req.GetDeletionJobId(), req.GetPolicyVersion())
				return &contentpb.ArchiveAccountContentResponse{Completed: true}, nil
			}}),
		},
		{
			name: "comment",
			eraser: newCommentEraser(&fakeCommentClient{call: func(req *commentpb.RedactAccountCommentsRequest) (*commentpb.RedactAccountCommentsResponse, error) {
				assertRequestIdentity(t, req.GetUserId(), req.GetDeletionJobId(), req.GetPolicyVersion())
				return &commentpb.RedactAccountCommentsResponse{Completed: true}, nil
			}}),
		},
		{
			name: "reaction",
			eraser: newReactionEraser(&fakeReactionClient{call: func(req *reactionpb.EraseAccountReactionsRequest) (*reactionpb.EraseAccountReactionsResponse, error) {
				assertRequestIdentity(t, req.GetUserId(), req.GetDeletionJobId(), req.GetPolicyVersion())
				return &reactionpb.EraseAccountReactionsResponse{Completed: true}, nil
			}}),
		},
		{
			name: "chat",
			eraser: newChatEraser(&fakeChatClient{call: func(req *chatpb.EraseUserDataRequest) (*chatpb.EraseUserDataResponse, error) {
				assertRequestIdentity(t, req.GetUserId(), req.GetDeletionJobId(), req.GetPolicyVersion())
				return &chatpb.EraseUserDataResponse{Completed: true}, nil
			}}),
		},
		{
			name: "notification",
			eraser: newNotificationEraser(&fakeNotificationClient{call: func(req *notificationpb.EraseUserDataRequest) (*notificationpb.MutationResponse, error) {
				assertRequestIdentity(t, req.GetUserId(), req.GetDeletionJobId(), req.GetPolicyVersion())
				return &notificationpb.MutationResponse{Success: true}, nil
			}}),
		},
		{
			name: "feed",
			eraser: newFeedEraser(&fakeFeedClient{call: func(req *feedpb.PurgeAccountFeedRequest) (*feedpb.PurgeAccountFeedResponse, error) {
				assertRequestIdentity(t, req.GetUserId(), req.GetDeletionJobId(), req.GetPolicyVersion())
				return &feedpb.PurgeAccountFeedResponse{Completed: true}, nil
			}}),
		},
		{
			name: "file",
			eraser: newFileEraser(&fakeFileClient{call: func(req *filepb.EraseUserDataRequest) (*filepb.EraseUserDataResponse, error) {
				assertRequestIdentity(t, req.GetUserId(), req.GetDeletionJobId(), req.GetPolicyVersion())
				return &filepb.EraseUserDataResponse{Completed: true}, nil
			}}),
		},
		{
			name: "credit",
			eraser: newCreditEraser(&fakeCreditClient{call: func(req *creditpb.EraseUserDataRequest) (*creditpb.EraseUserDataResponse, error) {
				assertRequestIdentity(t, req.GetUserId(), req.GetDeletionJobId(), req.GetPolicyVersion())
				return &creditpb.EraseUserDataResponse{Completed: true}, nil
			}}),
		},
		{
			name: "search",
			eraser: newSearchEraser(&fakeSearchClient{call: func(req *searchpb.EraseUserDataRequest) (*searchpb.EraseUserDataResponse, error) {
				assertRequestIdentity(t, req.GetUserId(), req.GetDeletionJobId(), req.GetPolicyVersion())
				return &searchpb.EraseUserDataResponse{Completed: true}, nil
			}}),
		},
	}
	for _, testCase := range tests {
		t.Run(testCase.name, func(t *testing.T) {
			if err := testCase.eraser.EraseUserData(t.Context(), 42, 9001, 3); err != nil {
				t.Fatalf("EraseUserData() error = %v", err)
			}
		})
	}
}

func TestEraserRejectsIncompleteAndPropagatesRPCError(t *testing.T) {
	eraser := &Eraser{service: "test-service", erase: func(context.Context, int64, int64, int32) (bool, error) { return false, nil }}
	if err := eraser.EraseUserData(t.Context(), 42, 9001, 3); err == nil {
		t.Fatal("incomplete response error = nil")
	}
	wantErr := errors.New("downstream unavailable")
	eraser.erase = func(context.Context, int64, int64, int32) (bool, error) { return false, wantErr }
	if err := eraser.EraseUserData(t.Context(), 42, 9001, 3); !errors.Is(err, wantErr) {
		t.Fatalf("RPC error = %v, want %v", err, wantErr)
	}
}

func TestInternalAuthCredentials(t *testing.T) {
	credentials := internalAuthCredentials{token: "internal-token"}
	metadata, err := credentials.GetRequestMetadata(t.Context())
	if err != nil || metadata[internalAuthMetadataKey] != "internal-token" {
		t.Fatalf("metadata = %v, error = %v", metadata, err)
	}
	if credentials.RequireTransportSecurity() {
		t.Fatal("configured local service discovery requires insecure transport support")
	}
}

func assertRequestIdentity(t *testing.T, userID, jobID int64, policyVersion int32) {
	t.Helper()
	if userID != 42 || jobID != 9001 || policyVersion != 3 {
		t.Fatalf("request identity = user %d job %d policy %d", userID, jobID, policyVersion)
	}
}

type fakeContentClient struct {
	call func(*contentpb.ArchiveAccountContentRequest) (*contentpb.ArchiveAccountContentResponse, error)
}

func (f *fakeContentClient) ArchiveAccountContent(_ context.Context, req *contentpb.ArchiveAccountContentRequest, _ ...grpc.CallOption) (*contentpb.ArchiveAccountContentResponse, error) {
	return f.call(req)
}

type fakeReactionClient struct {
	call func(*reactionpb.EraseAccountReactionsRequest) (*reactionpb.EraseAccountReactionsResponse, error)
}

type fakeCommentClient struct {
	call func(*commentpb.RedactAccountCommentsRequest) (*commentpb.RedactAccountCommentsResponse, error)
}

func (f *fakeCommentClient) RedactAccountComments(_ context.Context, req *commentpb.RedactAccountCommentsRequest, _ ...grpc.CallOption) (*commentpb.RedactAccountCommentsResponse, error) {
	return f.call(req)
}

func (f *fakeReactionClient) EraseAccountReactions(_ context.Context, req *reactionpb.EraseAccountReactionsRequest, _ ...grpc.CallOption) (*reactionpb.EraseAccountReactionsResponse, error) {
	return f.call(req)
}

type fakeChatClient struct {
	call func(*chatpb.EraseUserDataRequest) (*chatpb.EraseUserDataResponse, error)
}

func (f *fakeChatClient) EraseUserData(_ context.Context, req *chatpb.EraseUserDataRequest, _ ...grpc.CallOption) (*chatpb.EraseUserDataResponse, error) {
	return f.call(req)
}

type fakeNotificationClient struct {
	call func(*notificationpb.EraseUserDataRequest) (*notificationpb.MutationResponse, error)
}

func (f *fakeNotificationClient) EraseUserData(_ context.Context, req *notificationpb.EraseUserDataRequest, _ ...grpc.CallOption) (*notificationpb.MutationResponse, error) {
	return f.call(req)
}

type fakeFeedClient struct {
	call func(*feedpb.PurgeAccountFeedRequest) (*feedpb.PurgeAccountFeedResponse, error)
}

type fakeFileClient struct {
	call func(*filepb.EraseUserDataRequest) (*filepb.EraseUserDataResponse, error)
}

type fakeCreditClient struct {
	call func(*creditpb.EraseUserDataRequest) (*creditpb.EraseUserDataResponse, error)
}

func (f *fakeCreditClient) EraseUserData(_ context.Context, req *creditpb.EraseUserDataRequest, _ ...grpc.CallOption) (*creditpb.EraseUserDataResponse, error) {
	return f.call(req)
}

func (f *fakeFileClient) EraseUserData(_ context.Context, req *filepb.EraseUserDataRequest, _ ...grpc.CallOption) (*filepb.EraseUserDataResponse, error) {
	return f.call(req)
}

func (f *fakeFeedClient) PurgeAccountFeed(_ context.Context, req *feedpb.PurgeAccountFeedRequest, _ ...grpc.CallOption) (*feedpb.PurgeAccountFeedResponse, error) {
	return f.call(req)
}

type fakeSearchClient struct {
	call func(*searchpb.EraseUserDataRequest) (*searchpb.EraseUserDataResponse, error)
}

func (f *fakeSearchClient) EraseUserData(_ context.Context, req *searchpb.EraseUserDataRequest, _ ...grpc.CallOption) (*searchpb.EraseUserDataResponse, error) {
	return f.call(req)
}
