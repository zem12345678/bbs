package persistence

import (
	"context"
	"errors"
	"fmt"
	"os"
	"sync"
	"testing"
	"time"

	domain "admin/internal/domain/admin"
	"admin/internal/infrastructure/persistence/po"
)

func TestDefaultCasbinRulesIncludeAnnouncementManagement(t *testing.T) {
	want := map[string]bool{
		string(domain.ActionListAnnouncements):  false,
		string(domain.ActionCreateAnnouncement): false,
		string(domain.ActionUpdateAnnouncement): false,
		string(domain.ActionDeleteAnnouncement): false,
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

func TestSeedDefaultsIncludesAnnouncementMenu(t *testing.T) {
	dsn := os.Getenv("BBS_ADMIN_TEST_DSN")
	if dsn == "" {
		t.Skip("set BBS_ADMIN_TEST_DSN to run postgres-backed repository tests")
	}
	ctx := context.Background()
	repo, cleanup := repositoryForProtectedRoleTest(t, ctx, dsn)
	defer cleanup()

	menu := systemMenuByName(t, ctx, repo, "governance.announcements")
	if menu.Permission != "governance:list_announcements" || menu.Path != "/governance/announcements" || menu.Component != "governance/announcements/index" {
		t.Fatalf("announcement menu = %#v", menu)
	}
	for _, want := range []struct{ name, permission string }{
		{name: "governance.announcements.query", permission: "governance:list_announcements"},
		{name: "governance.announcements.create", permission: "governance:create_announcement"},
		{name: "governance.announcements.update", permission: "governance:update_announcement"},
		{name: "governance.announcements.delete", permission: "governance:delete_announcement"},
	} {
		button := systemMenuByName(t, ctx, repo, want.name)
		if button.ParentID != menu.ID || button.Permission != want.permission {
			t.Fatalf("%s = (parent=%d permission=%q), want (parent=%d permission=%q)", want.name, button.ParentID, button.Permission, menu.ID, want.permission)
		}
	}
}

func TestRepositoryAnnouncementLifecycleReadStateAndAudience(t *testing.T) {
	dsn := os.Getenv("BBS_ADMIN_TEST_DSN")
	if dsn == "" {
		t.Skip("set BBS_ADMIN_TEST_DSN to run postgres-backed repository tests")
	}
	ctx := context.Background()
	repo, cleanup := repositoryForProtectedRoleTest(t, ctx, dsn)
	defer cleanup()

	now := time.Now().UnixMilli()
	first, err := repo.CreateAnnouncement(ctx, "first", repositoryAnnouncementCommand("第一条"), now-2000)
	if err != nil {
		t.Fatalf("CreateAnnouncement(first) error = %v", err)
	}
	second, err := repo.CreateAnnouncement(ctx, "second", repositoryAnnouncementCommand("第二条"), now-1000)
	if err != nil {
		t.Fatalf("CreateAnnouncement(second) error = %v", err)
	}
	list, err := repo.ListAnnouncements(ctx, domain.AnnouncementListFilter{Limit: 10, Status: "all"})
	if err != nil {
		t.Fatalf("ListAnnouncements() error = %v", err)
	}
	if list.Total != 2 || len(list.Items) != 2 || list.Items[0].ID != second.ID || list.Items[1].ID != first.ID {
		t.Fatalf("ListAnnouncements() = %#v", list)
	}
	page, err := repo.ListAnnouncements(ctx, domain.AnnouncementListFilter{Limit: 10, UntilID: second.ID, Status: "all"})
	if err != nil || len(page.Items) != 1 || page.Items[0].ID != first.ID {
		t.Fatalf("cursor page = %#v, err = %v", page, err)
	}

	if err := repo.MarkAnnouncementRead(ctx, 7, second.ID, now); err != nil {
		t.Fatalf("MarkAnnouncementRead() error = %v", err)
	}
	active := true
	public, err := repo.ListPublicAnnouncements(ctx, 7, now-10000, domain.PublicAnnouncementListFilter{Limit: 10, Active: &active}, now)
	if err != nil {
		t.Fatalf("ListPublicAnnouncements() error = %v", err)
	}
	if len(public.Items) != 2 || !public.Items[0].IsRead {
		t.Fatalf("public announcements = %#v", public.Items)
	}
	newTitle := "第二条更新"
	updated, err := repo.UpdateAnnouncement(ctx, domain.UpdateAnnouncementCommand{ID: second.ID, Title: &newTitle}, now+1000)
	if err != nil || updated.Title != newTitle {
		t.Fatalf("UpdateAnnouncement() = %#v, err = %v", updated, err)
	}
	public, err = repo.ListPublicAnnouncements(ctx, 7, now-10000, domain.PublicAnnouncementListFilter{Limit: 10, Active: &active}, now+2000)
	if err != nil || len(public.Items) != 2 || !public.Items[0].IsRead {
		t.Fatalf("read state after update = %#v, err = %v", public.Items, err)
	}

	targeted := repositoryAnnouncementCommand("定向")
	targeted.UserID = 8
	if _, err := repo.CreateAnnouncement(ctx, "targeted", targeted, now); err != nil {
		t.Fatalf("CreateAnnouncement(targeted) error = %v", err)
	}
	anonymous, err := repo.ListPublicAnnouncements(ctx, 0, 0, domain.PublicAnnouncementListFilter{Limit: 10, Active: &active}, now)
	if err != nil {
		t.Fatalf("ListPublicAnnouncements(anonymous) error = %v", err)
	}
	for _, item := range anonymous.Items {
		if item.ID == "targeted" {
			t.Fatal("targeted announcement leaked to anonymous viewer")
		}
	}
	targetViewer, err := repo.ListPublicAnnouncements(ctx, 8, now-10000, domain.PublicAnnouncementListFilter{Limit: 10, Active: &active}, now)
	if err != nil {
		t.Fatalf("ListPublicAnnouncements(target) error = %v", err)
	}
	foundTarget := false
	for _, item := range targetViewer.Items {
		if item.ID == "targeted" {
			foundTarget = item.ForYou
		}
	}
	if !foundTarget {
		t.Fatalf("target viewer items = %#v", targetViewer.Items)
	}
	if err := repo.MarkAnnouncementRead(ctx, 8, "targeted", now+500); err != nil {
		t.Fatalf("MarkAnnouncementRead(targeted) error = %v", err)
	}
	targetViewer, err = repo.ListPublicAnnouncements(ctx, 8, now-10000, domain.PublicAnnouncementListFilter{Limit: 10, Active: &active}, now+1000)
	if err != nil {
		t.Fatalf("ListPublicAnnouncements(target after read) error = %v", err)
	}
	for _, item := range targetViewer.Items {
		if item.ID == "targeted" {
			t.Fatal("personal announcement remained active after it was read")
		}
	}

	if err := repo.DeleteAnnouncement(ctx, second.ID); err != nil {
		t.Fatalf("DeleteAnnouncement() error = %v", err)
	}
	var reads int64
	if err := repo.db.WithContext(ctx).Model(&po.AnnouncementRead{}).Where("announcement_id = ?", second.ID).Count(&reads).Error; err != nil {
		t.Fatalf("count announcement reads: %v", err)
	}
	if reads != 0 {
		t.Fatalf("announcement reads after delete = %d, want 0", reads)
	}
}

func TestRepositoryLimitsActiveDialogAnnouncements(t *testing.T) {
	dsn := os.Getenv("BBS_ADMIN_TEST_DSN")
	if dsn == "" {
		t.Skip("set BBS_ADMIN_TEST_DSN to run postgres-backed repository tests")
	}
	ctx := context.Background()
	repo, cleanup := repositoryForProtectedRoleTest(t, ctx, dsn)
	defer cleanup()
	for index := 0; index < maxActiveDialogAnnouncements; index++ {
		command := repositoryAnnouncementCommand("对话框")
		command.Display = "dialog"
		if _, err := repo.CreateAnnouncement(ctx, fmt.Sprintf("dialog-%d", index), command, time.Now().UnixMilli()); err != nil {
			t.Fatalf("CreateAnnouncement(dialog-%d) error = %v", index, err)
		}
	}
	command := repositoryAnnouncementCommand("超限")
	command.Display = "dialog"
	if _, err := repo.CreateAnnouncement(ctx, "dialog-over-limit", command, time.Now().UnixMilli()); !errors.Is(err, domain.ErrAnnouncementDialogLimit) {
		t.Fatalf("CreateAnnouncement(over limit) error = %v, want ErrAnnouncementDialogLimit", err)
	}
}

func TestRepositoryConcurrentAnnouncementCreatesDoNotLoseUpdates(t *testing.T) {
	dsn := os.Getenv("BBS_ADMIN_TEST_DSN")
	if dsn == "" {
		t.Skip("set BBS_ADMIN_TEST_DSN to run postgres-backed repository tests")
	}
	ctx := context.Background()
	repo, cleanup := repositoryForProtectedRoleTest(t, ctx, dsn)
	defer cleanup()

	start := make(chan struct{})
	errorsChannel := make(chan error, 2)
	var wait sync.WaitGroup
	for _, item := range []struct{ id, title string }{{"concurrent-a", "并发 A"}, {"concurrent-b", "并发 B"}} {
		item := item
		wait.Add(1)
		go func() {
			defer wait.Done()
			<-start
			_, err := repo.CreateAnnouncement(ctx, item.id, repositoryAnnouncementCommand(item.title), time.Now().UnixMilli())
			errorsChannel <- err
		}()
	}
	close(start)
	wait.Wait()
	close(errorsChannel)
	for err := range errorsChannel {
		if err != nil {
			t.Fatalf("concurrent CreateAnnouncement() error = %v", err)
		}
	}
	list, err := repo.ListAnnouncements(ctx, domain.AnnouncementListFilter{Limit: 10, Status: "all"})
	if err != nil {
		t.Fatalf("ListAnnouncements() error = %v", err)
	}
	found := map[string]bool{}
	for _, item := range list.Items {
		found[item.ID] = true
	}
	if !found["concurrent-a"] || !found["concurrent-b"] {
		t.Fatalf("concurrent items = %#v", list.Items)
	}
}

func TestRepositoryDoesNotOverwriteDamagedAnnouncementJSON(t *testing.T) {
	dsn := os.Getenv("BBS_ADMIN_TEST_DSN")
	if dsn == "" {
		t.Skip("set BBS_ADMIN_TEST_DSN to run postgres-backed repository tests")
	}
	ctx := context.Background()
	repo, cleanup := repositoryForProtectedRoleTest(t, ctx, dsn)
	defer cleanup()

	if err := repo.db.WithContext(ctx).Model(&po.SiteSetting{}).Where("key = ?", announcementSettingKey).Update("value", "not-json").Error; err != nil {
		t.Fatalf("damage announcement JSON: %v", err)
	}
	if _, err := repo.CreateAnnouncement(ctx, "must-not-write", repositoryAnnouncementCommand("不应写入"), time.Now().UnixMilli()); !errors.Is(err, domain.ErrInvalidAnnouncement) {
		t.Fatalf("CreateAnnouncement() error = %v, want ErrInvalidAnnouncement", err)
	}
	var setting po.SiteSetting
	if err := repo.db.WithContext(ctx).Where("key = ?", announcementSettingKey).First(&setting).Error; err != nil {
		t.Fatalf("load damaged setting: %v", err)
	}
	if setting.Value != "not-json" {
		t.Fatalf("damaged JSON was overwritten with %q", setting.Value)
	}
}

func repositoryAnnouncementCommand(title string) domain.CreateAnnouncementCommand {
	return domain.CreateAnnouncementCommand{Title: title, Text: "正文", Icon: "info", Display: "banner", Active: true}
}
