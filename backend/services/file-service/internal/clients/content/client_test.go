package content

import (
	"context"
	"errors"
	"testing"

	"file-service/api/proto/contentpb"
	app "file-service/internal/application/file"
	domain "file-service/internal/domain/file"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func TestGetTopicMapsPublishedTopic(t *testing.T) {
	fake := &fakeContentServiceClient{response: &contentpb.TopicResponse{Topic: &contentpb.TopicInfo{Id: 8, AuthorId: 9, Status: 2}}}
	topic, err := (&Client{client: fake}).GetTopic(context.Background(), 8)
	if err != nil {
		t.Fatalf("GetTopic() error = %v", err)
	}
	if topic != (app.Topic{ID: 8, AuthorID: 9, Status: 2}) {
		t.Fatalf("GetTopic() = %+v", topic)
	}
	if fake.request == nil || fake.request.GetId() != 8 {
		t.Fatalf("GetTopic request = %+v", fake.request)
	}
}

func TestGetTopicMapsUnavailableTopics(t *testing.T) {
	tests := []struct {
		name string
		err  error
		resp *contentpb.TopicResponse
		want error
	}{
		{name: "not found", err: status.Error(codes.NotFound, "topic not found"), want: domain.ErrAttachmentTopicUnavailable},
		{name: "empty response", resp: &contentpb.TopicResponse{}, want: domain.ErrAttachmentTopicUnavailable},
		{name: "upstream unavailable", err: status.Error(codes.Unavailable, "content unavailable"), want: domain.ErrContentServiceUnavailable},
		{name: "unexpected upstream error", err: errors.New("content failed"), want: domain.ErrContentServiceUnavailable},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			_, err := (&Client{client: &fakeContentServiceClient{response: test.resp, err: test.err}}).GetTopic(context.Background(), 8)
			if !errors.Is(err, test.want) {
				t.Fatalf("GetTopic() error = %v, want %v", err, test.want)
			}
		})
	}
}

type fakeContentServiceClient struct {
	request  *contentpb.GetTopicRequest
	response *contentpb.TopicResponse
	err      error
}

func (f *fakeContentServiceClient) GetTopic(_ context.Context, request *contentpb.GetTopicRequest, _ ...grpc.CallOption) (*contentpb.TopicResponse, error) {
	f.request = request
	return f.response, f.err
}
