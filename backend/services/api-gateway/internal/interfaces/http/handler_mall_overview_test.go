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
			OrderTotal:         3,
			PendingOutboxTotal: 0,
			OrderStatusCounts: []*mallpb.MallStatusCount{
				{Status: "paid", Count: 1},
			},
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
	require.Contains(t, overview, "order_status_counts")
	require.Contains(t, overview, "low_stock_products")
}
