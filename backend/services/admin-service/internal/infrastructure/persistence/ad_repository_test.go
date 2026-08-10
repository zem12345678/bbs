package persistence

import (
	"context"
	"os"
	"testing"
	"time"

	domain "admin/internal/domain/admin"
)

func TestDefaultCasbinRulesIncludeAdManagement(t *testing.T) {
	want := map[string]bool{
		string(domain.ActionListAds):  false,
		string(domain.ActionCreateAd): false,
		string(domain.ActionUpdateAd): false,
		string(domain.ActionDeleteAd): false,
	}
	for _, rule := range defaultCasbinRules() {
		if rule.V0 == "admin" && rule.V1 == domain.ResourceGovernance {
			if _, ok := want[rule.V2]; ok {
				want[rule.V2] = true
			}
		}
	}
	for action, found := range want {
		if !found {
			t.Fatalf("default admin rules do not include governance:%s", action)
		}
	}
}

func TestSeedDefaultsIncludesAdMenu(t *testing.T) {
	dsn := os.Getenv("BBS_ADMIN_TEST_DSN")
	if dsn == "" {
		t.Skip("set BBS_ADMIN_TEST_DSN to run postgres-backed repository tests")
	}

	ctx := context.Background()
	repo, cleanup := repositoryForProtectedRoleTest(t, ctx, dsn)
	defer cleanup()

	menu := systemMenuByName(t, ctx, repo, "governance.ads")
	if menu.Permission != "governance:list_ads" || menu.Path != "/governance/ads" || menu.Component != "governance/ads/index" {
		t.Fatalf("governance.ads = (permission=%q, path=%q, component=%q)", menu.Permission, menu.Path, menu.Component)
	}
	for _, want := range []struct {
		name       string
		permission string
	}{
		{name: "governance.ads.query", permission: "governance:list_ads"},
		{name: "governance.ads.create", permission: "governance:create_ad"},
		{name: "governance.ads.update", permission: "governance:update_ad"},
		{name: "governance.ads.delete", permission: "governance:delete_ad"},
	} {
		button := systemMenuByName(t, ctx, repo, want.name)
		if button.ParentID != menu.ID || button.Permission != want.permission {
			t.Fatalf("%s = (parent=%d, permission=%q), want (parent=%d, permission=%q)", want.name, button.ParentID, button.Permission, menu.ID, want.permission)
		}
	}

	permissions, err := repo.PermissionsByRoleKeys(ctx, []string{"admin"})
	if err != nil {
		t.Fatalf("PermissionsByRoleKeys(admin) error = %v", err)
	}
	for _, permission := range []string{"governance:list_ads", "governance:create_ad", "governance:update_ad", "governance:delete_ad"} {
		if !containsString(permissions, permission) {
			t.Fatalf("admin permissions = %v, want %s", permissions, permission)
		}
	}
}

func TestRepositoryFiltersAndPaginatesAds(t *testing.T) {
	dsn := os.Getenv("BBS_ADMIN_TEST_DSN")
	if dsn == "" {
		t.Skip("set BBS_ADMIN_TEST_DSN to run postgres-backed repository tests")
	}

	ctx := context.Background()
	repo, cleanup := repositoryForProtectedRoleTest(t, ctx, dsn)
	defer cleanup()

	now := time.Now().UTC().Truncate(time.Second)
	weekday := int32(1 << uint(now.Weekday()))
	create := func(priority string, startsAt time.Time, expiresAt time.Time, dayOfWeek int32) domain.Ad {
		t.Helper()
		ad, err := repo.CreateAd(ctx, domain.CreateAdCommand{
			URL:       "https://example.com/landing",
			Memo:      priority,
			Place:     "vertical",
			Priority:  priority,
			Ratio:     1,
			StartsAt:  startsAt,
			ExpiresAt: expiresAt,
			ImageURL:  "https://cdn.example.com/ad.png",
			DayOfWeek: dayOfWeek,
		})
		if err != nil {
			t.Fatalf("CreateAd(%s) error = %v", priority, err)
		}
		return ad
	}

	low := create("low", now.Add(-time.Hour), now.Add(time.Hour), weekday)
	high := create("high", now.Add(-time.Hour), now.Add(time.Hour), weekday)
	expired := create("middle", now.Add(-2*time.Hour), now.Add(-time.Hour), weekday)

	publishing := true
	active, err := repo.ListAds(ctx, 10, 0, 0, &publishing, now)
	if err != nil {
		t.Fatalf("ListAds(publishing=true) error = %v", err)
	}
	if active.Total != 2 || len(active.Items) != 2 || active.Items[0].ID != high.ID || active.Items[1].ID != low.ID {
		t.Fatalf("ListAds(publishing=true) = %#v", active)
	}

	publishing = false
	inactive, err := repo.ListAds(ctx, 10, 0, 0, &publishing, now)
	if err != nil {
		t.Fatalf("ListAds(publishing=false) error = %v", err)
	}
	if inactive.Total != 1 || len(inactive.Items) != 1 || inactive.Items[0].ID != expired.ID {
		t.Fatalf("ListAds(publishing=false) = %#v", inactive)
	}

	afterHigh, err := repo.ListAds(ctx, 10, high.ID, 0, nil, now)
	if err != nil {
		t.Fatalf("ListAds(sinceID) error = %v", err)
	}
	if afterHigh.Total != 1 || len(afterHigh.Items) != 1 || afterHigh.Items[0].ID != expired.ID {
		t.Fatalf("ListAds(sinceID=%d) = %#v", high.ID, afterHigh)
	}

	beforeHigh, err := repo.ListAds(ctx, 10, 0, high.ID, nil, now)
	if err != nil {
		t.Fatalf("ListAds(untilID) error = %v", err)
	}
	if beforeHigh.Total != 1 || len(beforeHigh.Items) != 1 || beforeHigh.Items[0].ID != low.ID {
		t.Fatalf("ListAds(untilID=%d) = %#v", high.ID, beforeHigh)
	}

	public, err := repo.ListActiveAds(ctx, now)
	if err != nil {
		t.Fatalf("ListActiveAds() error = %v", err)
	}
	if len(public) != 2 || public[0].ID != high.ID || public[1].ID != low.ID {
		t.Fatalf("ListActiveAds() = %#v", public)
	}
}
