package mall

import "testing"

func TestOrderMatchesIdempotencyRequestAllowsSameRequestSnapshot(t *testing.T) {
	existing := Order{
		IdempotencyKey: "order-key",
		UserID:         7,
		CouponCode:     "SAVE10",
		Receiver:       "Alice",
		Phone:          "13800138000",
		Address:        "No.1 Road",
		Items: []OrderItem{
			{ProductID: 101, Quantity: 3},
			{ProductID: 102, Quantity: 1},
		},
	}
	requested := Order{
		IdempotencyKey: "order-key",
		UserID:         7,
		CouponCode:     " save10 ",
		Receiver:       " Alice ",
		Phone:          " 13800138000 ",
		Address:        " No.1 Road ",
		Items: []OrderItem{
			{ProductID: 102, Quantity: 1},
			{ProductID: 101, Quantity: 1},
			{ProductID: 101, Quantity: 2},
		},
	}

	if !OrderMatchesIdempotencyRequest(existing, requested) {
		t.Fatal("OrderMatchesIdempotencyRequest() = false, want true for same logical request")
	}
}

func TestOrderMatchesIdempotencyRequestRejectsDifferentRequestSnapshot(t *testing.T) {
	existing := Order{
		IdempotencyKey: "order-key",
		UserID:         7,
		CouponCode:     "SAVE10",
		Receiver:       "Alice",
		Phone:          "13800138000",
		Address:        "No.1 Road",
		Items:          []OrderItem{{ProductID: 101, Quantity: 1}},
	}
	tests := []struct {
		name      string
		requested Order
	}{
		{
			name: "different user",
			requested: Order{
				IdempotencyKey: "order-key",
				UserID:         8,
				CouponCode:     "SAVE10",
				Receiver:       "Alice",
				Phone:          "13800138000",
				Address:        "No.1 Road",
				Items:          []OrderItem{{ProductID: 101, Quantity: 1}},
			},
		},
		{
			name: "different coupon",
			requested: Order{
				IdempotencyKey: "order-key",
				UserID:         7,
				CouponCode:     "SAVE20",
				Receiver:       "Alice",
				Phone:          "13800138000",
				Address:        "No.1 Road",
				Items:          []OrderItem{{ProductID: 101, Quantity: 1}},
			},
		},
		{
			name: "different address",
			requested: Order{
				IdempotencyKey: "order-key",
				UserID:         7,
				CouponCode:     "SAVE10",
				Receiver:       "Alice",
				Phone:          "13800138000",
				Address:        "No.2 Road",
				Items:          []OrderItem{{ProductID: 101, Quantity: 1}},
			},
		},
		{
			name: "different quantity",
			requested: Order{
				IdempotencyKey: "order-key",
				UserID:         7,
				CouponCode:     "SAVE10",
				Receiver:       "Alice",
				Phone:          "13800138000",
				Address:        "No.1 Road",
				Items:          []OrderItem{{ProductID: 101, Quantity: 2}},
			},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if OrderMatchesIdempotencyRequest(existing, test.requested) {
				t.Fatal("OrderMatchesIdempotencyRequest() = true, want false")
			}
		})
	}
}
