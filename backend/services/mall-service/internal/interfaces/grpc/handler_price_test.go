package grpc

import (
	"context"
	"testing"

	pb "mall-service/api/proto/mallpb"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func TestOrderEndpointsRequireExpectedOriginalCredits(t *testing.T) {
	handler := NewHandler(nil)
	negative := int64(-1)

	tests := []struct {
		name string
		call func() error
	}{
		{
			name: "create missing",
			call: func() error {
				_, err := handler.CreateOrder(context.Background(), &pb.CreateOrderRequest{})
				return err
			},
		},
		{
			name: "create negative",
			call: func() error {
				_, err := handler.CreateOrder(context.Background(), &pb.CreateOrderRequest{ExpectedOriginalCredits: &negative})
				return err
			},
		},
		{
			name: "cart missing",
			call: func() error {
				_, err := handler.CheckoutCart(context.Background(), &pb.CheckoutCartRequest{})
				return err
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if code := status.Code(test.call()); code != codes.InvalidArgument {
				t.Fatalf("status code = %s, want %s", code, codes.InvalidArgument)
			}
		})
	}
}
