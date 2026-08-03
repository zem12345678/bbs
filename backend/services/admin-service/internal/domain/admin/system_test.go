package admin

import "testing"

func TestIsProtectedSystemRoleKey(t *testing.T) {
	tests := []struct {
		name string
		key  string
		want bool
	}{
		{name: "admin", key: "admin", want: true},
		{name: "superadmin", key: "superadmin", want: true},
		{name: "case and space", key: " Admin ", want: true},
		{name: "normal role", key: "moderator", want: false},
		{name: "empty", key: "", want: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := IsProtectedSystemRoleKey(tt.key); got != tt.want {
				t.Fatalf("IsProtectedSystemRoleKey(%q) = %v, want %v", tt.key, got, tt.want)
			}
		})
	}
}

func TestIsProtectedSystemUserName(t *testing.T) {
	tests := []struct {
		name     string
		username string
		want     bool
	}{
		{name: "admin", username: "admin", want: true},
		{name: "superadmin", username: "superadmin", want: true},
		{name: "case and space", username: " Admin ", want: true},
		{name: "normal user", username: "operator", want: false},
		{name: "empty", username: "", want: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := IsProtectedSystemUserName(tt.username); got != tt.want {
				t.Fatalf("IsProtectedSystemUserName(%q) = %v, want %v", tt.username, got, tt.want)
			}
		})
	}
}

func TestResourceForActionMapsMallOperations(t *testing.T) {
	for _, action := range allMallActions() {
		if got := ResourceForAction(action); got != ResourceMall {
			t.Fatalf("ResourceForAction(%q) = %q, want %q", action, got, ResourceMall)
		}
	}
}

func TestResourceForActionMapsInviteCodeOperationsToGovernance(t *testing.T) {
	for _, action := range []Action{ActionListInviteCodes, ActionCreateInviteCodes, ActionRevokeInviteCode} {
		if got := ResourceForAction(action); got != ResourceGovernance {
			t.Fatalf("ResourceForAction(%q) = %q, want %q", action, got, ResourceGovernance)
		}
	}
}

func allMallActions() []Action {
	return []Action{
		ActionListMallProductCategories,
		ActionCreateMallProductCategory,
		ActionUpdateMallProductCategory,
		ActionListMallProducts,
		ActionCreateMallProduct,
		ActionUpdateMallProduct,
		ActionListMallProductReviews,
		ActionUpdateMallProductReview,
		ActionListMallCoupons,
		ActionListMallCouponUsages,
		ActionCreateMallCoupon,
		ActionUpdateMallCoupon,
		ActionListMallOrders,
		ActionListMallDigitalEntitlements,
		ActionRevokeMallDigitalEntitlement,
		ActionCloseExpiredMall,
		ActionRecoverPayingMallOrders,
		ActionRequeueMallOutboxEvents,
		ActionUpdateMallOrder,
		ActionListMallOrderLogs,
		ActionListMallPayments,
		ActionListMallRefunds,
		ActionReviewMallRefunds,
	}
}
