package credit

import (
	"context"
	"errors"
	"testing"

	"content-service/api/proto/creditpb"

	"google.golang.org/grpc"
)

func TestServiceNameNormalizesLegacyCreditService(t *testing.T) {
	if got := serviceName("credit-service"); got != "bbs-credit-service" {
		t.Fatalf("serviceName = %q", got)
	}
}

func TestReserveQABounty(t *testing.T) {
	creditClient := &fakeCreditServiceClient{}
	client := &Client{client: creditClient}

	got, err := client.ReserveQABounty(context.Background(), 42, 101, 50, "如何排查回调？")
	if err != nil {
		t.Fatalf("ReserveQABounty() error = %v", err)
	}
	if !got {
		t.Fatal("ReserveQABounty() = false, want true")
	}
	if creditClient.reserveReq.GetUserId() != 42 || creditClient.reserveReq.GetAmount() != 50 {
		t.Fatalf("ReserveCredits user/amount = %d/%d, want 42/50", creditClient.reserveReq.GetUserId(), creditClient.reserveReq.GetAmount())
	}
	if creditClient.reserveReq.GetReason() != "qa_bounty_reserved" || creditClient.reserveReq.GetSourceEventId() != "content.qa.bounty:101" {
		t.Fatalf("ReserveCredits reason/event = %q/%q", creditClient.reserveReq.GetReason(), creditClient.reserveReq.GetSourceEventId())
	}
	if creditClient.reserveReq.GetSourceType() != "topic" || creditClient.reserveReq.GetSourceId() != 101 {
		t.Fatalf("ReserveCredits source = %q/%d", creditClient.reserveReq.GetSourceType(), creditClient.reserveReq.GetSourceId())
	}
}

func TestReserveQABountyPropagatesReserveError(t *testing.T) {
	wantErr := errors.New("credit unavailable")
	client := &Client{client: &fakeCreditServiceClient{err: wantErr}}

	_, err := client.ReserveQABounty(context.Background(), 42, 101, 50, "如何排查回调？")
	if !errors.Is(err, wantErr) {
		t.Fatalf("err = %v, want %v", err, wantErr)
	}
}

type fakeCreditServiceClient struct {
	reserveReq *creditpb.ReserveCreditsRequest
	err        error
}

func (f *fakeCreditServiceClient) GetBalance(_ context.Context, req *creditpb.GetBalanceRequest, _ ...grpc.CallOption) (*creditpb.BalanceResponse, error) {
	return &creditpb.BalanceResponse{Balance: &creditpb.Balance{UserId: req.GetUserId()}}, nil
}

func (f *fakeCreditServiceClient) ReserveCredits(_ context.Context, req *creditpb.ReserveCreditsRequest, _ ...grpc.CallOption) (*creditpb.ReserveCreditsResponse, error) {
	f.reserveReq = req
	if f.err != nil {
		return nil, f.err
	}
	return &creditpb.ReserveCreditsResponse{}, nil
}
