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

func TestHasEnoughCredit(t *testing.T) {
	creditClient := &fakeCreditServiceClient{
		resp: &creditpb.BalanceResponse{Balance: &creditpb.Balance{UserId: 42, Total: 80}},
	}
	client := &Client{client: creditClient}

	got, err := client.HasEnoughCredit(context.Background(), 42, 50)
	if err != nil {
		t.Fatalf("HasEnoughCredit() error = %v", err)
	}
	if !got {
		t.Fatal("HasEnoughCredit() = false, want true")
	}
	if creditClient.req.GetUserId() != 42 {
		t.Fatalf("GetBalance user id = %d, want 42", creditClient.req.GetUserId())
	}
}

func TestHasEnoughCreditRejectsInsufficientBalance(t *testing.T) {
	client := &Client{client: &fakeCreditServiceClient{
		resp: &creditpb.BalanceResponse{Balance: &creditpb.Balance{UserId: 42, Total: 20}},
	}}

	got, err := client.HasEnoughCredit(context.Background(), 42, 50)
	if err != nil {
		t.Fatalf("HasEnoughCredit() error = %v", err)
	}
	if got {
		t.Fatal("HasEnoughCredit() = true, want false")
	}
}

func TestHasEnoughCreditPropagatesGetBalanceError(t *testing.T) {
	wantErr := errors.New("credit unavailable")
	client := &Client{client: &fakeCreditServiceClient{err: wantErr}}

	_, err := client.HasEnoughCredit(context.Background(), 42, 50)
	if !errors.Is(err, wantErr) {
		t.Fatalf("err = %v, want %v", err, wantErr)
	}
}

type fakeCreditServiceClient struct {
	req  *creditpb.GetBalanceRequest
	resp *creditpb.BalanceResponse
	err  error
}

func (f *fakeCreditServiceClient) GetBalance(_ context.Context, req *creditpb.GetBalanceRequest, _ ...grpc.CallOption) (*creditpb.BalanceResponse, error) {
	f.req = req
	if f.err != nil {
		return nil, f.err
	}
	return f.resp, nil
}
