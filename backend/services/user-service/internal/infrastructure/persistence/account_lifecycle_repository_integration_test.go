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

func TestAccountLifecycleRepoPostgresIntegration(t *testing.T) {
	if os.Getenv("BBS_PG_SMOKE") != "1" {
		t.Skip("set BBS_PG_SMOKE=1 to run PostgreSQL account-lifecycle transaction test")
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
	userID := base
	protectedUserID := base + 1
	userIDs := []int64{userID, protectedUserID}
	defer func() {
		_ = db.Exec("DELETE FROM user_account_actions WHERE target_user_id IN ?", userIDs).Error
		_ = db.Exec("DELETE FROM user_account_jobs WHERE user_id IN ?", userIDs).Error
		_ = db.Exec("DELETE FROM users WHERE id IN ?", userIDs).Error
	}()

	now := time.Now().UTC()
	users := []userPO{
		integrationAccountLifecycleUser(userID, false, now),
		integrationAccountLifecycleUser(protectedUserID, true, now),
	}
	if err := db.Create(&users).Error; err != nil {
		t.Fatalf("create test users: %v", err)
	}

	type deletionResult struct {
		lifecycle domain.AccountLifecycle
		err       error
	}
	results := make(chan deletionResult, 2)
	start := make(chan struct{})
	var wg sync.WaitGroup
	for i := 0; i < 2; i++ {
		i := i
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-start
			lifecycle, requestErr := repo.RequestAccountDeletion(ctx, domain.AccountDeletionRequest{
				JobID:                     base + 100 + int64(i),
				UserID:                    userID,
				ActorUserID:               userID,
				ExpectedCredentialVersion: domain.InitialCredentialVersion,
				CredentialVersion:         fmt.Sprintf("lifecycle-version-%d-%d", base, i),
				RequestedAt:               now.Add(time.Duration(i) * time.Millisecond),
				PolicyVersion:             7,
				Steps:                     domain.AccountDeletionSteps(),
			})
			results <- deletionResult{lifecycle: lifecycle, err: requestErr}
		}()
	}
	close(start)
	wg.Wait()
	close(results)

	var activeJobID int64
	for result := range results {
		if result.err != nil {
			t.Fatalf("concurrent deletion request: %v", result.err)
		}
		if result.lifecycle.State != domain.AccountStateDeletionPending || result.lifecycle.ActiveDeletionJob == nil {
			t.Fatalf("lifecycle = %+v, want deletion_pending with active job", result.lifecycle)
		}
		if activeJobID == 0 {
			activeJobID = result.lifecycle.ActiveDeletionJob.ID
		} else if result.lifecycle.ActiveDeletionJob.ID != activeJobID {
			t.Fatalf("concurrent requests returned job IDs %d and %d", activeJobID, result.lifecycle.ActiveDeletionJob.ID)
		}
	}

	var jobCount, stepCount, actionCount int64
	if err := db.Model(&accountJobPO{}).Where("user_id = ?", userID).Count(&jobCount).Error; err != nil {
		t.Fatalf("count jobs: %v", err)
	}
	if err := db.Model(&accountJobStepPO{}).Where("job_id = ?", activeJobID).Count(&stepCount).Error; err != nil {
		t.Fatalf("count job steps: %v", err)
	}
	if err := db.Model(&accountActionPO{}).Where("target_user_id = ? AND action = ?", userID, "request_deletion").Count(&actionCount).Error; err != nil {
		t.Fatalf("count actions: %v", err)
	}
	if jobCount != 1 || stepCount != int64(len(domain.AccountDeletionSteps())) || actionCount != 1 {
		t.Fatalf("jobs=%d steps=%d actions=%d, want 1/%d/1", jobCount, stepCount, actionCount, len(domain.AccountDeletionSteps()))
	}
	var policyVersion int32
	if err := db.Raw("SELECT (metadata ->> 'policy_version')::integer FROM user_account_actions WHERE target_user_id = ? AND action = ?", userID, "request_deletion").Scan(&policyVersion).Error; err != nil {
		t.Fatalf("read audit policy version: %v", err)
	}
	if policyVersion != 7 {
		t.Fatalf("audit policy version=%d, want 7", policyVersion)
	}

	var persisted userPO
	if err := db.First(&persisted, userID).Error; err != nil {
		t.Fatalf("read pending user: %v", err)
	}
	if persisted.AccountState != string(domain.AccountStateDeletionPending) || persisted.AccountStateVersion != 2 || persisted.CredentialVersion == domain.InitialCredentialVersion || persisted.DeletionRequestedAt == nil {
		t.Fatalf("persisted lifecycle = %+v", persisted)
	}

	_, err = repo.RequestAccountDeletion(ctx, domain.AccountDeletionRequest{
		JobID: base + 200, UserID: protectedUserID, ActorUserID: protectedUserID,
		ExpectedCredentialVersion: domain.InitialCredentialVersion,
		CredentialVersion:         fmt.Sprintf("protected-version-%d", base),
		RequestedAt:               now, PolicyVersion: 1, Steps: domain.AccountDeletionSteps(),
	})
	if !errors.Is(err, domain.ErrAccountProtected) {
		t.Fatalf("protected account error=%v, want ErrAccountProtected", err)
	}
	if err := db.Model(&accountJobPO{}).Where("user_id = ?", protectedUserID).Count(&jobCount).Error; err != nil {
		t.Fatalf("count protected jobs: %v", err)
	}
	if jobCount != 0 {
		t.Fatalf("protected account jobs=%d, want 0", jobCount)
	}
}

func TestAccountDeletionJobRepoPostgresIntegration(t *testing.T) {
	if os.Getenv("BBS_PG_SMOKE") != "1" {
		t.Skip("set BBS_PG_SMOKE=1 to run PostgreSQL account-deletion worker test")
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
	userID, followerID, followeeID := base, base+1, base+2
	userIDs := []int64{userID, followerID, followeeID}
	defer func() {
		_ = db.Exec("DELETE FROM user_account_actions WHERE target_user_id IN ?", userIDs).Error
		_ = db.Exec("DELETE FROM user_account_jobs WHERE user_id IN ?", userIDs).Error
		_ = db.Exec("DELETE FROM users WHERE id IN ?", userIDs).Error
	}()

	now := time.Now().UTC()
	users := []userPO{
		integrationAccountLifecycleUser(userID, false, now),
		integrationAccountLifecycleUser(followerID, false, now),
		integrationAccountLifecycleUser(followeeID, false, now),
	}
	users[0].FollowerCount, users[0].FollowingCount = 1, 1
	users[1].FollowingCount = 1
	users[2].FollowerCount = 1
	if err := db.Create(&users).Error; err != nil {
		t.Fatalf("create worker test users: %v", err)
	}
	if err := db.Create(&[]followPO{{FollowerID: userID, FolloweeID: followeeID, CreatedAt: now}, {FollowerID: followerID, FolloweeID: userID, CreatedAt: now}}).Error; err != nil {
		t.Fatalf("create follows: %v", err)
	}
	if err := db.Create(&oauthAccountPO{Provider: "worker-test", ProviderUserID: fmt.Sprint(base), UserID: userID, Username: "private", Email: "private@example.com", Nickname: "Private", CreatedAt: now, UpdatedAt: now}).Error; err != nil {
		t.Fatalf("create oauth account: %v", err)
	}
	if err := db.Create(&passwordResetTokenPO{TokenHash: fmt.Sprintf("reset-%d", base), UserID: userID, Email: "private@example.com", ExpiresAt: now.Add(time.Hour), CreatedAt: now}).Error; err != nil {
		t.Fatalf("create password reset: %v", err)
	}
	if err := db.Create(&emailVerificationTokenPO{TokenHash: fmt.Sprintf("verify-%d", base), UserID: userID, Email: "private@example.com", ExpiresAt: now.Add(time.Hour), CreatedAt: now}).Error; err != nil {
		t.Fatalf("create email verification: %v", err)
	}
	ownedList := mustIntegrationUserList(t, base+500, userID, "Owned before deletion", true)
	if err := repo.CreateUserList(ctx, ownedList); err != nil {
		t.Fatalf("create owned list: %v", err)
	}
	otherList := mustIntegrationUserList(t, base+501, followeeID, "Other list", true)
	if err := repo.CreateUserList(ctx, otherList); err != nil {
		t.Fatalf("create other list: %v", err)
	}
	if err := repo.AddUserListMember(ctx, followeeID, domain.UserListMembership{ListID: otherList.ID, UserID: userID}); err != nil {
		t.Fatalf("add target to other list: %v", err)
	}
	if err := repo.FavoriteUserList(ctx, domain.UserListFavorite{ListID: otherList.ID, UserID: userID}); err != nil {
		t.Fatalf("favorite other list: %v", err)
	}

	request, err := repo.RequestAccountDeletion(ctx, domain.AccountDeletionRequest{
		JobID: base + 600, UserID: userID, ActorUserID: userID,
		ExpectedCredentialVersion: domain.InitialCredentialVersion,
		CredentialVersion:         fmt.Sprintf("pending-version-%d", base),
		RequestedAt:               now, PolicyVersion: 1, Steps: domain.AccountDeletionSteps(),
	})
	if err != nil {
		t.Fatalf("request deletion: %v", err)
	}
	jobID := request.ActiveDeletionJob.ID
	claim, err := repo.ClaimAccountDeletionJob(ctx, "worker-a", now.Add(time.Second), now.Add(time.Minute))
	if err != nil || claim == nil || claim.Job.ID != jobID {
		t.Fatalf("claim job=%+v error=%v", claim, err)
	}
	firstService := domain.AccountDeletionSteps()[0]
	if _, err := repo.BeginAccountDeletionStep(ctx, jobID, firstService, "worker-a", now.Add(2*time.Second), now.Add(time.Minute)); err != nil {
		t.Fatalf("begin first step: %v", err)
	}
	if err := repo.RetryAccountDeletionStep(ctx, jobID, firstService, "worker-a", "transient failure", now.Add(3*time.Second), now.Add(13*time.Second), 8); err != nil {
		t.Fatalf("retry first step: %v", err)
	}
	earlyClaim, err := repo.ClaimAccountDeletionJob(ctx, "worker-b", now.Add(10*time.Second), now.Add(time.Minute))
	if err != nil || earlyClaim != nil {
		t.Fatalf("early claim=%+v error=%v, want nil", earlyClaim, err)
	}
	claim, err = repo.ClaimAccountDeletionJob(ctx, "worker-b", now.Add(13*time.Second), now.Add(2*time.Minute))
	if err != nil || claim == nil || claim.Job.ID != jobID {
		t.Fatalf("reclaim job=%+v error=%v", claim, err)
	}
	if _, err := repo.FinalizeAccountDeletionJob(ctx, jobID, "worker-b", domain.AccountAnonymization{
		Username: "__erased_early", Email: fmt.Sprintf("early+%d@invalid.local", base), PasswordHash: "!deleted!",
		CredentialVersion: fmt.Sprintf("early-version-%d", base), CompletedAt: now.Add(14 * time.Second),
	}); !errors.Is(err, domain.ErrAccountDeletionStepsIncomplete) {
		t.Fatalf("early finalize error=%v, want ErrAccountDeletionStepsIncomplete", err)
	}
	for _, step := range claim.Steps {
		if step.Status == domain.AccountJobSucceeded {
			continue
		}
		if _, err := repo.BeginAccountDeletionStep(ctx, jobID, step.Service, "worker-b", now.Add(15*time.Second), now.Add(2*time.Minute)); err != nil {
			t.Fatalf("begin %s: %v", step.Service, err)
		}
		if err := repo.CompleteAccountDeletionStep(ctx, jobID, step.Service, "worker-b", now.Add(16*time.Second)); err != nil {
			t.Fatalf("complete %s: %v", step.Service, err)
		}
	}
	finalCredential := fmt.Sprintf("final-version-%d", base)
	finalized, err := repo.FinalizeAccountDeletionJob(ctx, jobID, "worker-b", domain.AccountAnonymization{
		Username: fmt.Sprintf("__erased_%x", userID), Email: fmt.Sprintf("erased+%x+%x@invalid.local", userID, jobID),
		PasswordHash: "!account-anonymized!", CredentialVersion: finalCredential, CompletedAt: now.Add(17 * time.Second),
	})
	if err != nil {
		t.Fatalf("finalize deletion: %v", err)
	}
	if finalized.AccountState != domain.AccountStateAnonymized || finalized.Username == users[0].Username || finalized.Email == users[0].Email || finalized.Nickname != "已注销用户" || finalized.AvatarURL != "" || finalized.CredentialVersion != finalCredential || finalized.DeletedAt == nil {
		t.Fatalf("finalized user=%+v", finalized)
	}
	for _, model := range []any{&oauthAccountPO{}, &passwordResetTokenPO{}, &emailVerificationTokenPO{}, &followPO{}, &userListMembershipPO{}, &userListFavoritePO{}} {
		var count int64
		query := db.Model(model)
		switch model.(type) {
		case *followPO:
			query = query.Where("follower_id = ? OR followee_id = ?", userID, userID)
		case *userListMembershipPO, *userListFavoritePO:
			query = query.Where("user_id = ?", userID)
		default:
			query = query.Where("user_id = ?", userID)
		}
		if err := query.Count(&count).Error; err != nil || count != 0 {
			t.Fatalf("cleanup %T count=%d error=%v", model, count, err)
		}
	}
	var ownedListCount int64
	if err := db.Model(&userListPO{}).Where("owner_id = ?", userID).Count(&ownedListCount).Error; err != nil || ownedListCount != 0 {
		t.Fatalf("owned lists count=%d error=%v", ownedListCount, err)
	}
	var follower, followee userPO
	if err := db.First(&follower, followerID).Error; err != nil {
		t.Fatalf("read follower: %v", err)
	}
	if err := db.First(&followee, followeeID).Error; err != nil {
		t.Fatalf("read followee: %v", err)
	}
	if follower.FollowingCount != 0 || followee.FollowerCount != 0 {
		t.Fatalf("follow counters follower.following=%d followee.followers=%d", follower.FollowingCount, followee.FollowerCount)
	}
	var job accountJobPO
	if err := db.First(&job, jobID).Error; err != nil || job.Status != string(domain.AccountJobSucceeded) || job.CompletedAt == nil || job.LeaseOwner != nil {
		t.Fatalf("completed job=%+v error=%v", job, err)
	}
	var actionCount int64
	if err := db.Model(&accountActionPO{}).Where("target_user_id = ?", userID).Count(&actionCount).Error; err != nil || actionCount != 2 {
		t.Fatalf("action count=%d error=%v, want 2", actionCount, err)
	}
}

func integrationAccountLifecycleUser(id int64, protected bool, now time.Time) userPO {
	return userPO{
		ID: id, Username: fmt.Sprintf("lifecycle_%d", id), Email: fmt.Sprintf("lifecycle-%d@example.com", id),
		PasswordHash: "hash", CredentialVersion: domain.InitialCredentialVersion,
		Nickname: "Lifecycle Test", ProfileTheme: domain.ProfileThemeDefault,
		Status: int32(domain.StatusActive), AccountState: string(domain.AccountStateActive), AccountStateVersion: 1,
		ProtectedAccount: protected, CreatedAt: now, UpdatedAt: now,
	}
}
