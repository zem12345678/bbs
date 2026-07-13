package http

import (
	"encoding/json"
	"testing"

	"api-gateway/api/proto/mallpb"

	"github.com/stretchr/testify/require"
)

func TestAdminMallOverviewPayloadIncludesZeroOutboxTotal(t *testing.T) {
	payload := adminMallOverviewPayload(&mallpb.AdminMallOverviewResponse{
		Overview: &mallpb.MallOverview{
			OrderTotal:                   3,
			PendingOutboxTotal:           0,
			SucceededPaymentCreditsTotal: 90,
			FailedPaymentTotal:           1,
			FailedPaymentCreditsTotal:    30,
			PendingRefundCreditsTotal:    10,
			NetRevenueCreditsTotal:       70,
			FinanceAnomalyTotal:          1,
			FinanceAnomalies: []*mallpb.FinanceAnomaly{
				{
					IssueType:                 "PAYMENT_MISMATCH",
					OrderId:                   99,
					OrderNo:                   "M202607130001",
					UserId:                    7,
					OrderStatus:               mallpb.OrderStatus_ORDER_STATUS_PAID,
					OrderTotalCredits:         120,
					SucceededPaymentCredits:   80,
					RefundedCredits:           10,
					DifferenceCredits:         -40,
					UpdatedAt:                 1700000900000,
				},
			},
			OrderStatusCounts: []*mallpb.MallStatusCount{
				{Status: "paid", Count: 1},
			},
			OutboxStatusCounts: []*mallpb.MallStatusCount{
				{Status: "failed", Count: 2},
			},
			OutboxLastError:     "kafka timeout",
			OutboxLastErrorAt:   1700000000000,
			OutboxNextAttemptAt: 1700000600000,
			LowStockProducts: []*mallpb.Product{
				{Id: 42, Title: "Low stock", Stock: 0},
			},
		},
	})

	body, err := json.Marshal(payload)
	require.NoError(t, err)

	var decoded map[string]map[string]any
	require.NoError(t, json.Unmarshal(body, &decoded))

	overview := decoded["overview"]
	require.Contains(t, overview, "pending_outbox_total")
	require.Equal(t, float64(0), overview["pending_outbox_total"])
	require.Equal(t, float64(90), overview["succeeded_payment_credits_total"])
	require.Equal(t, float64(1), overview["failed_payment_total"])
	require.Equal(t, float64(30), overview["failed_payment_credits_total"])
	require.Equal(t, float64(10), overview["pending_refund_credits_total"])
	require.Equal(t, float64(70), overview["net_revenue_credits_total"])
	require.Equal(t, float64(1), overview["finance_anomaly_total"])
	require.Len(t, overview["finance_anomalies"], 1)
	require.Equal(t, "kafka timeout", overview["outbox_last_error"])
	require.Equal(t, float64(1700000000000), overview["outbox_last_error_at"])
	require.Equal(t, float64(1700000600000), overview["outbox_next_attempt_at"])
	require.Contains(t, overview, "order_status_counts")
	require.Contains(t, overview, "outbox_status_counts")
	require.Contains(t, overview, "low_stock_products")
}
