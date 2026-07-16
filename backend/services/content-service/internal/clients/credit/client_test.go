package credit

import (
	"context"
	"errors"
	"testing"

	"content-service/api/proto/creditpb"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
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

func TestReleaseQABounty(t *testing.T) {
	creditClient := &fakeCreditServiceClient{}
	client := &Client{client: creditClient}

	got, err := client.ReleaseQABounty(context.Background(), 42, 101, 50, "如何排查回调？")
	if err != nil {
		t.Fatalf("ReleaseQABounty() error = %v", err)
	}
	if !got {
		t.Fatal("ReleaseQABounty() = false, want true")
	}
	if creditClient.releaseReq.GetUserId() != 42 || creditClient.releaseReq.GetAmount() != 50 {
		t.Fatalf("ReleaseCredits user/amount = %d/%d, want 42/50", creditClient.releaseReq.GetUserId(), creditClient.releaseReq.GetAmount())
	}
	if creditClient.releaseReq.GetReservationReason() != "qa_bounty_reserved" || creditClient.releaseReq.GetReleaseReason() != "qa_bounty_released" {
		t.Fatalf("ReleaseCredits reasons = %q/%q", creditClient.releaseReq.GetReservationReason(), creditClient.releaseReq.GetReleaseReason())
	}
	if creditClient.releaseReq.GetSourceEventId() != "content.qa.bounty:101" || creditClient.releaseReq.GetSourceType() != "topic" || creditClient.releaseReq.GetSourceId() != 101 {
		t.Fatalf("ReleaseCredits source = event:%q type:%q id:%d", creditClient.releaseReq.GetSourceEventId(), creditClient.releaseReq.GetSourceType(), creditClient.releaseReq.GetSourceId())
	}
}

func TestReleaseQABountyIgnoresMissingLegacyReservation(t *testing.T) {
	client := &Client{client: &fakeCreditServiceClient{releaseErr: status.Error(codes.NotFound, "missing")}}

	got, err := client.ReleaseQABounty(context.Background(), 42, 101, 50, "如何排查回调？")
	if err != nil {
		t.Fatalf("ReleaseQABounty() error = %v", err)
	}
	if !got {
		t.Fatal("ReleaseQABounty() = false, want true")
	}
}

type fakeCreditServiceClient struct {
	reserveReq *creditpb.ReserveCreditsRequest
	releaseReq *creditpb.ReleaseCreditsRequest
	err        error
	releaseErr error
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

func (f *fakeCreditServiceClient) ReleaseCredits(_ context.Context, req *creditpb.ReleaseCreditsRequest, _ ...grpc.CallOption) (*creditpb.ReleaseCreditsResponse, error) {
	f.releaseReq = req
	if f.releaseErr != nil {
		return nil, f.releaseErr
	}
	if f.err != nil {
		return nil, f.err
	}
	return &creditpb.ReleaseCreditsResponse{}, nil
}
