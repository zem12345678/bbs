package messaging

import (
	"encoding/json"
	"testing"
)

func TestMallDigitalEntitlementRevokedPayloadContract(t *testing.T) {
	payloadJSON := []byte(`{
		"event_id":"evt-entitlement-revoked",
		"event_type":"mall.digital_entitlement.revoked.v1",
		"occurred_at_unix_ms":1784025000000,
		"entitlement_id":503,
		"order_id":8804,
		"order_no":"MO202607140001",
		"user_id":42,
		"product_id":101,
		"sku":"VIP-MONTH",
		"title":"会员月卡",
		"fulfillment_code":"BBS-VIP-503",
		"grant_type":"membership",
		"grant_key":"vip-month",
		"status":"REVOKED",
		"operator_id":"admin-7",
		"reason":"risk review"
	}`)

	var payload mallDigitalEntitlementRevokedPayload
	if err := json.Unmarshal(payloadJSON, &payload); err != nil {
		t.Fatalf("unmarshal mall entitlement revoked payload: %v", err)
	}
	if payload.EventID != "evt-entitlement-revoked" || payload.EntitlementID != 503 || payload.OrderID != 8804 || payload.UserID != 42 {
		t.Fatalf("payload identifiers = %+v", payload)
	}
	if payload.GrantType != "membership" || payload.GrantKey != "vip-month" || payload.Status != "REVOKED" {
		t.Fatalf("payload entitlement state = %+v", payload)
	}
	if payload.FulfillmentCode != "BBS-VIP-503" || payload.OperatorID != "admin-7" || payload.Reason != "risk review" {
		t.Fatalf("payload notification details = %+v", payload)
	}
}
