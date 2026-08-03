package persistence

import (
	"context"
	"errors"
	"fmt"
	"os"
	"sync"
	"testing"
	"time"

	domain "user-service/internal/domain/user"

	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

func TestUserListRepoPostgresIntegration(t *testing.T) {
	if os.Getenv("BBS_PG_SMOKE") != "1" {
		t.Skip("set BBS_PG_SMOKE=1 to run PostgreSQL user-list transaction test")
	}
	dsn := os.Getenv("BBS_USER_PG_DSN")
	if dsn == "" {
		dsn = "postgres://bbs_user_app:local_user_pass@127.0.0.1:5432/bbs?sslmode=disable&search_path=bbs_user"
	}
	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Fatalf("open postgres: %v", err)
	}
	repo := NewRepo(db)
	ctx := context.Background()
	base := time.Now().UnixNano()

	const memberCount = domain.MaxUserListMembers + 1
	ownerID := base
	viewerID := base + 1
	limitOwnerID := base + 2
	memberOwnerID := base + 3
	copyOwnerID := base + 4
	blockedMemberID := base + 5
	memberStartID := base + 10
	userIDs := []int64{ownerID, viewerID, limitOwnerID, memberOwnerID, copyOwnerID, blockedMemberID}
	for i := 0; i < memberCount; i++ {
		userIDs = append(userIDs, memberStartID+int64(i))
	}
	defer func() {
		_ = db.Exec("DELETE FROM users WHERE id IN ?", userIDs).Error
	}()
	rows := make([]userPO, 0, len(userIDs))
	now := time.Now()
	for _, id := range userIDs {
		rows = append(rows, integrationUserListUser(id, now))
	}
	if err := db.CreateInBatches(&rows, 50).Error; err != nil {
		t.Fatalf("create test users: %v", err)
	}

	privateList := mustIntegrationUserList(t, base+1_000, ownerID, "Private", false)
	publicList := mustIntegrationUserList(t, base+1_001, ownerID, "Public", true)
	uniqueList := mustIntegrationUserList(t, base+1_002, ownerID, "Case Sensitive", false)
	for _, list := range []*domain.UserList{privateList, publicList, uniqueList} {
		if err := repo.CreateUserList(ctx, list); err != nil {
			t.Fatalf("create %q list: %v", list.Name, err)
		}
	}
	if _, err := repo.GetUserList(ctx, 0, privateList.ID); !errors.Is(err, domain.ErrUserListNotFound) {
		t.Fatalf("anonymous private list error = %v, want ErrUserListNotFound", err)
	}
	if _, err := repo.GetUserList(ctx, ownerID, privateList.ID); err != nil {
		t.Fatalf("owner get private list: %v", err)
	}
	if _, err := repo.GetUserList(ctx, 0, publicList.ID); err != nil {
		t.Fatalf("anonymous get public list: %v", err)
	}
	visible, visibleTotal, err := repo.ListUserLists(ctx, domain.UserListsQuery{ViewerID: viewerID, OwnerID: ownerID, Page: 1, PageSize: 20})
	if err != nil {
		t.Fatalf("list visible user lists: %v", err)
	}
	if visibleTotal != 1 || len(visible) != 1 || visible[0].ID != publicList.ID {
		t.Fatalf("visible lists total=%d items=%+v", visibleTotal, visible)
	}
	duplicateName := mustIntegrationUserList(t, base+1_003, ownerID, "case sensitive", true)
	if err := repo.CreateUserList(ctx, duplicateName); !errors.Is(err, domain.ErrUserListNameExists) {
		t.Fatalf("case-insensitive duplicate error = %v, want ErrUserListNameExists", err)
	}

	for i := 0; i < domain.MaxUserListsPerOwner-1; i++ {
		list := mustIntegrationUserList(t, base+1_100+int64(i), limitOwnerID, fmt.Sprintf("List %02d", i), false)
		if err := repo.CreateUserList(ctx, list); err != nil {
			t.Fatalf("seed list limit item %d: %v", i, err)
		}
	}
	listRaceErrors := make(chan error, 2)
	listRaceStart := make(chan struct{})
	var listRaceWG sync.WaitGroup
	for i := 0; i < 2; i++ {
		list := mustIntegrationUserList(t, base+1_200+int64(i), limitOwnerID, fmt.Sprintf("Race %d", i), false)
		listRaceWG.Add(1)
		go func() {
			defer listRaceWG.Done()
			<-listRaceStart
			listRaceErrors <- repo.CreateUserList(ctx, list)
		}()
	}
	close(listRaceStart)
	listRaceWG.Wait()
	close(listRaceErrors)
	assertIntegrationRaceResult(t, listRaceErrors, domain.ErrUserListLimitReached)
	_, listTotal, err := repo.ListUserLists(ctx, domain.UserListsQuery{ViewerID: limitOwnerID, OwnerID: limitOwnerID, Page: 1, PageSize: 100})
	if err != nil || listTotal != domain.MaxUserListsPerOwner {
		t.Fatalf("list limit total=%d error=%v, want %d", listTotal, err, domain.MaxUserListsPerOwner)
	}

	memberList := mustIntegrationUserList(t, base+1_300, memberOwnerID, "Members", false)
	if err := repo.CreateUserList(ctx, memberList); err != nil {
		t.Fatalf("create member-limit list: %v", err)
	}
	for i := 0; i < domain.MaxUserListMembers-1; i++ {
		if err := repo.AddUserListMember(ctx, memberOwnerID, domain.UserListMembership{ListID: memberList.ID, UserID: memberStartID + int64(i)}); err != nil {
			t.Fatalf("seed member %d: %v", i, err)
		}
	}
	memberRaceErrors := make(chan error, 2)
	memberRaceStart := make(chan struct{})
	var memberRaceWG sync.WaitGroup
	for i := domain.MaxUserListMembers - 1; i <= domain.MaxUserListMembers; i++ {
		membership := domain.UserListMembership{ListID: memberList.ID, UserID: memberStartID + int64(i)}
		memberRaceWG.Add(1)
		go func() {
			defer memberRaceWG.Done()
			<-memberRaceStart
			memberRaceErrors <- repo.AddUserListMember(ctx, memberOwnerID, membership)
		}()
	}
	close(memberRaceStart)
	memberRaceWG.Wait()
	close(memberRaceErrors)
	assertIntegrationRaceResult(t, memberRaceErrors, domain.ErrUserListMemberLimitReached)
	_, membersTotal, err := repo.ListUserListMembers(ctx, domain.UserListMembersQuery{ViewerID: memberOwnerID, ListID: memberList.ID, Page: 1, PageSize: 100})
	if err != nil || membersTotal != domain.MaxUserListMembers {
		t.Fatalf("member limit total=%d error=%v, want %d", membersTotal, err, domain.MaxUserListMembers)
	}

	if err := repo.Block(ctx, blockedMemberID, ownerID); err != nil {
		t.Fatalf("block list owner: %v", err)
	}
	if err := repo.AddUserListMember(ctx, ownerID, domain.UserListMembership{ListID: publicList.ID, UserID: blockedMemberID}); !errors.Is(err, domain.ErrUserListMemberBlocked) {
		t.Fatalf("blocked-owner membership error = %v, want ErrUserListMemberBlocked", err)
	}
	if err := repo.AddUserListMember(ctx, ownerID, domain.UserListMembership{ListID: publicList.ID, UserID: memberStartID}); err != nil {
		t.Fatalf("add source-list member: %v", err)
	}

	copiedList := mustIntegrationUserList(t, base+1_400, copyOwnerID, "Copied", false)
	if err := repo.CopyUserList(ctx, publicList.ID, copiedList); err != nil {
		t.Fatalf("copy public list: %v", err)
	}
	copiedMembers, copiedTotal, err := repo.ListUserListMembers(ctx, domain.UserListMembersQuery{ViewerID: copyOwnerID, ListID: copiedList.ID, Page: 1, PageSize: 10})
	if err != nil || copiedTotal != 1 || len(copiedMembers) != 1 || copiedMembers[0].ID != memberStartID {
		t.Fatalf("copied members total=%d items=%+v error=%v", copiedTotal, copiedMembers, err)
	}

	if err := repo.FavoriteUserList(ctx, domain.UserListFavorite{ListID: publicList.ID, UserID: viewerID}); err != nil {
		t.Fatalf("favorite public list: %v", err)
	}
	favorites, favoriteTotal, err := repo.ListFavoriteUserLists(ctx, domain.UserListFavoritesQuery{UserID: viewerID, Page: 1, PageSize: 10})
	if err != nil || favoriteTotal != 1 || len(favorites) != 1 || favorites[0].ID != publicList.ID || !favorites[0].IsFavorited {
		t.Fatalf("favorite lists total=%d items=%+v error=%v", favoriteTotal, favorites, err)
	}
	if err := publicList.Update(publicList.Name, false); err != nil {
		t.Fatalf("make favorite private: %v", err)
	}
	if err := repo.UpdateUserList(ctx, publicList); err != nil {
		t.Fatalf("persist private favorite: %v", err)
	}
	favorites, favoriteTotal, err = repo.ListFavoriteUserLists(ctx, domain.UserListFavoritesQuery{UserID: viewerID, Page: 1, PageSize: 10})
	if err != nil || favoriteTotal != 0 || len(favorites) != 0 {
		t.Fatalf("private favorite lists total=%d items=%+v error=%v", favoriteTotal, favorites, err)
	}

	if err := copiedList.Update(copiedList.Name, true); err != nil {
		t.Fatalf("make copied list public: %v", err)
	}
	if err := repo.UpdateUserList(ctx, copiedList); err != nil {
		t.Fatalf("persist public copied list: %v", err)
	}
	if err := repo.FavoriteUserList(ctx, domain.UserListFavorite{ListID: copiedList.ID, UserID: viewerID}); err != nil {
		t.Fatalf("favorite copied list: %v", err)
	}
	if err := repo.DeleteUserList(ctx, copyOwnerID, copiedList.ID); err != nil {
		t.Fatalf("delete copied list: %v", err)
	}
	cascadeChecks := []struct {
		name  string
		model any
		where string
	}{
		{name: "lists", model: &userListPO{}, where: "id = ?"},
		{name: "memberships", model: &userListMembershipPO{}, where: "list_id = ?"},
		{name: "favorites", model: &userListFavoritePO{}, where: "list_id = ?"},
	}
	for _, check := range cascadeChecks {
		var count int64
		if err := db.Model(check.model).Where(check.where, copiedList.ID).Count(&count).Error; err != nil {
			t.Fatalf("count cascade %s: %v", check.name, err)
		}
		if count != 0 {
			t.Fatalf("cascade %s count=%d, want 0", check.name, count)
		}
	}
}

func integrationUserListUser(id int64, now time.Time) userPO {
	return userPO{
		ID: id, Username: fmt.Sprintf("ul_%d", id), Email: fmt.Sprintf("ul-%d@example.com", id),
		PasswordHash: "hash", CredentialVersion: domain.InitialCredentialVersion,
		Nickname: "User List Test", ProfileTheme: domain.ProfileThemeDefault,
		Status: int32(domain.StatusActive), CreatedAt: now, UpdatedAt: now,
	}
}

func mustIntegrationUserList(t *testing.T, id, ownerID int64, name string, isPublic bool) *domain.UserList {
	t.Helper()
	list, err := domain.NewUserList(id, ownerID, name, isPublic)
	if err != nil {
		t.Fatalf("new user list %q: %v", name, err)
	}
	return list
}

func assertIntegrationRaceResult(t *testing.T, errs <-chan error, wantError error) {
	t.Helper()
	var successes, expectedErrors int
	for err := range errs {
		switch {
		case err == nil:
			successes++
		case errors.Is(err, wantError):
			expectedErrors++
		default:
			t.Fatalf("unexpected concurrent error: %v", err)
		}
	}
	if successes != 1 || expectedErrors != 1 {
		t.Fatalf("concurrent result successes=%d expectedErrors=%d, want 1/1", successes, expectedErrors)
	}
}
