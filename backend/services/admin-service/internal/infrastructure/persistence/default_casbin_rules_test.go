package persistence

import (
	"testing"

	domain "admin/internal/domain/admin"
)

func TestDefaultCasbinRulesGrantAdminOperationalPermissions(t *testing.T) {
	required := []string{
		permission(domain.ResourceSystem, domain.ActionViewDashboard),
		permission(domain.ResourceSystem, domain.ActionListSystemUsers),
		permission(domain.ResourceSystem, domain.ActionListSystemRoles),
		permission(domain.ResourceSystem, domain.ActionAssignSystemRoleMenus),
		permission(domain.ResourceSystem, domain.ActionSendSystemNotification),
		permission(domain.ResourceSystem, domain.ActionRebuildSearch),
		permission(domain.ResourceSystem, domain.ActionViewSearchRebuild),
		permission(domain.ResourceGovernance, domain.ActionListReports),
		permission(domain.ResourceGovernance, domain.ActionAuditReport),
		permission(domain.ResourceGovernance, domain.ActionListArticles),
		permission(domain.ResourceGovernance, domain.ActionPublishArticle),
		permission(domain.ResourceGovernance, domain.ActionHideArticle),
		permission(domain.ResourceGovernance, domain.ActionArchiveArticle),
		permission(domain.ResourceGovernance, domain.ActionListTopics),
		permission(domain.ResourceGovernance, domain.ActionPublishTopic),
		permission(domain.ResourceGovernance, domain.ActionHideTopic),
		permission(domain.ResourceGovernance, domain.ActionArchiveTopic),
		permission(domain.ResourceGovernance, domain.ActionListComments),
		permission(domain.ResourceGovernance, domain.ActionHideComment),
		permission(domain.ResourceGovernance, domain.ActionRestoreComment),
		permission(domain.ResourceGovernance, domain.ActionListUsers),
		permission(domain.ResourceGovernance, domain.ActionMuteUser),
		permission(domain.ResourceGovernance, domain.ActionUnmuteUser),
		permission(domain.ResourceGovernance, domain.ActionListCategories),
		permission(domain.ResourceGovernance, domain.ActionCreateCategory),
		permission(domain.ResourceGovernance, domain.ActionUpdateCategory),
		permission(domain.ResourceGovernance, domain.ActionDeleteCategory),
		permission(domain.ResourceGovernance, domain.ActionListUserCredits),
		permission(domain.ResourceGovernance, domain.ActionAdjustUserCredits),
		permission(domain.ResourceGovernance, domain.ActionListInviteCodes),
		permission(domain.ResourceGovernance, domain.ActionCreateInviteCodes),
		permission(domain.ResourceGovernance, domain.ActionRevokeInviteCode),
	}
	required = append(required, allMallPermissions()...)

	for _, role := range []string{"admin", "superadmin"} {
		t.Run(role, func(t *testing.T) {
			permissions := defaultRulePermissionsByRoleKeys([]string{role})
			for _, want := range required {
				if !containsString(permissions, want) {
					t.Fatalf("default permissions for %q missing %q in %v", role, want, permissions)
				}
			}
		})
	}
}

func TestDefaultCasbinRulesGrantModeratorModerationPermissions(t *testing.T) {
	permissions := defaultRulePermissionsByRoleKeys([]string{"moderator"})
	required := []string{
		permission(domain.ResourceGovernance, domain.ActionListReports),
		permission(domain.ResourceGovernance, domain.ActionAuditReport),
		permission(domain.ResourceGovernance, domain.ActionListArticles),
		permission(domain.ResourceGovernance, domain.ActionPublishArticle),
		permission(domain.ResourceGovernance, domain.ActionHideArticle),
		permission(domain.ResourceGovernance, domain.ActionListTopics),
		permission(domain.ResourceGovernance, domain.ActionPublishTopic),
		permission(domain.ResourceGovernance, domain.ActionHideTopic),
		permission(domain.ResourceGovernance, domain.ActionListComments),
		permission(domain.ResourceGovernance, domain.ActionHideComment),
		permission(domain.ResourceGovernance, domain.ActionRestoreComment),
	}

	for _, want := range required {
		if !containsString(permissions, want) {
			t.Fatalf("default moderator permissions missing %q in %v", want, permissions)
		}
	}
	if containsString(permissions, permission(domain.ResourceSystem, domain.ActionSendSystemNotification)) {
		t.Fatalf("moderator must not receive system notification permission: %v", permissions)
	}
	if containsString(permissions, permission(domain.ResourceSystem, domain.ActionRebuildSearch)) || containsString(permissions, permission(domain.ResourceSystem, domain.ActionViewSearchRebuild)) {
		t.Fatalf("moderator must not receive search rebuild permissions: %v", permissions)
	}
	if containsString(permissions, permission(domain.ResourceGovernance, domain.ActionListInviteCodes)) ||
		containsString(permissions, permission(domain.ResourceGovernance, domain.ActionCreateInviteCodes)) ||
		containsString(permissions, permission(domain.ResourceGovernance, domain.ActionRevokeInviteCode)) {
		t.Fatalf("moderator must not receive invite code permissions: %v", permissions)
	}
}

func defaultRulePermissionsByRoleKeys(roles []string) []string {
	rules := defaultCasbinRules()
	roleSet := map[string]struct{}{}
	for _, role := range normalizeList(roles) {
		roleSet[role] = struct{}{}
	}
	for changed := true; changed; {
		changed = false
		for _, rule := range rules {
			if rule.Ptype != "g" {
				continue
			}
			child := normalize(rule.V0)
			parent := normalize(rule.V1)
			if child == "" || parent == "" {
				continue
			}
			if _, ok := roleSet[child]; !ok {
				continue
			}
			if _, ok := roleSet[parent]; ok {
				continue
			}
			roleSet[parent] = struct{}{}
			changed = true
		}
	}

	permissions := make([]string, 0)
	seen := map[string]struct{}{}
	for _, rule := range rules {
		if rule.Ptype != "p" {
			continue
		}
		role := normalize(rule.V0)
		resource := normalize(rule.V1)
		action := normalize(rule.V2)
		if role == "" || resource == "" || action == "" {
			continue
		}
		if _, ok := roleSet[role]; !ok {
			continue
		}
		permission := resource + ":" + action
		if _, ok := seen[permission]; ok {
			continue
		}
		seen[permission] = struct{}{}
		permissions = append(permissions, permission)
	}
	return permissions
}

func permission(resource string, action domain.Action) string {
	return resource + ":" + string(action)
}

func allMallPermissions() []string {
	actions := []domain.Action{
		domain.ActionListMallProductCategories,
		domain.ActionCreateMallProductCategory,
		domain.ActionUpdateMallProductCategory,
		domain.ActionListMallProducts,
		domain.ActionCreateMallProduct,
		domain.ActionUpdateMallProduct,
		domain.ActionListMallProductReviews,
		domain.ActionUpdateMallProductReview,
		domain.ActionListMallCoupons,
		domain.ActionListMallCouponUsages,
		domain.ActionCreateMallCoupon,
		domain.ActionUpdateMallCoupon,
		domain.ActionListMallOrders,
		domain.ActionListMallDigitalEntitlements,
		domain.ActionRevokeMallDigitalEntitlement,
		domain.ActionCloseExpiredMall,
		domain.ActionRecoverPayingMallOrders,
		domain.ActionRequeueMallOutboxEvents,
		domain.ActionUpdateMallOrder,
		domain.ActionListMallOrderLogs,
		domain.ActionListMallPayments,
		domain.ActionListMallRefunds,
		domain.ActionReviewMallRefunds,
	}
	permissions := make([]string, 0, len(actions))
	for _, action := range actions {
		permissions = append(permissions, permission(domain.ResourceMall, action))
	}
	return permissions
}
