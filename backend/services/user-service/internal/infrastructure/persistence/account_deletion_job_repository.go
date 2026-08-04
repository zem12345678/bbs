package persistence

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"

	domain "user-service/internal/domain/user"

	"github.com/google/uuid"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

var _ domain.AccountDeletionJobRepository = (*Repo)(nil)

func (r *Repo) ClaimAccountDeletionJob(ctx context.Context, leaseOwner string, now, leaseUntil time.Time) (*domain.AccountDeletionClaim, error) {
	leaseOwner = strings.TrimSpace(leaseOwner)
	if leaseOwner == "" || now.IsZero() || !leaseUntil.After(now) {
		return nil, domain.ErrAccountDeletionLeaseLost
	}
	var claim *domain.AccountDeletionClaim
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var job accountJobPO
		err := tx.Clauses(clause.Locking{Strength: "UPDATE", Options: "SKIP LOCKED"}).
			Where("kind = ? AND available_at <= ? AND attempts < ? AND (status IN ? OR (status = ? AND lease_expires_at <= ?))",
				"delete_account", now, 100,
				[]string{string(domain.AccountJobPending), string(domain.AccountJobRetryWait)},
				string(domain.AccountJobRunning), now).
			Order("available_at ASC, created_at ASC, id ASC").
			First(&job).Error
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil
		}
		if err != nil {
			return err
		}
		if err := tx.Model(&accountJobStepPO{}).
			Where("job_id = ? AND status = ? AND lease_expires_at <= ?", job.ID, string(domain.AccountJobRunning), now).
			Updates(map[string]any{
				"status": string(domain.AccountJobRetryWait), "available_at": now,
				"lease_owner": nil, "lease_expires_at": nil, "updated_at": now,
			}).Error; err != nil {
			return err
		}
		result := tx.Model(&accountJobPO{}).
			Where("id = ?", job.ID).
			Updates(map[string]any{
				"status": string(domain.AccountJobRunning), "attempts": gorm.Expr("attempts + 1"),
				"lease_owner": leaseOwner, "lease_expires_at": leaseUntil, "last_error": "",
				"started_at": gorm.Expr("COALESCE(started_at, ?)", now), "updated_at": now,
			})
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected != 1 {
			return domain.ErrAccountDeletionLeaseLost
		}
		if err := tx.First(&job, job.ID).Error; err != nil {
			return err
		}
		steps, err := loadAccountDeletionSteps(tx, job.ID)
		if err != nil {
			return err
		}
		claim = &domain.AccountDeletionClaim{
			Job:        accountDeletionJobFromPO(job, steps),
			Steps:      steps,
			LeaseOwner: leaseOwner,
			LeaseUntil: leaseUntil,
		}
		return nil
	})
	return claim, err
}

func (r *Repo) BeginAccountDeletionStep(ctx context.Context, jobID int64, service, leaseOwner string, now, leaseUntil time.Time) (int16, error) {
	service = strings.TrimSpace(service)
	leaseOwner = strings.TrimSpace(leaseOwner)
	if jobID <= 0 || service == "" || leaseOwner == "" || now.IsZero() || !leaseUntil.After(now) {
		return 0, domain.ErrAccountDeletionLeaseLost
	}
	var attempts int16
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		job := tx.Model(&accountJobPO{}).
			Where("id = ? AND status = ? AND lease_owner = ?", jobID, string(domain.AccountJobRunning), leaseOwner).
			Updates(map[string]any{"lease_expires_at": leaseUntil, "updated_at": now})
		if job.Error != nil {
			return job.Error
		}
		if job.RowsAffected != 1 {
			return domain.ErrAccountDeletionLeaseLost
		}
		step := tx.Model(&accountJobStepPO{}).
			Where("job_id = ? AND service = ? AND status IN ? AND available_at <= ?", jobID, service,
				[]string{string(domain.AccountJobPending), string(domain.AccountJobRetryWait)}, now).
			Updates(map[string]any{
				"status": string(domain.AccountJobRunning), "attempts": gorm.Expr("attempts + 1"),
				"lease_owner": leaseOwner, "lease_expires_at": leaseUntil, "last_error": "", "updated_at": now,
			})
		if step.Error != nil {
			return step.Error
		}
		if step.RowsAffected != 1 {
			return domain.ErrAccountDeletionLeaseLost
		}
		return tx.Model(&accountJobStepPO{}).
			Select("attempts").
			Where("job_id = ? AND service = ?", jobID, service).
			Scan(&attempts).Error
	})
	return attempts, err
}

func (r *Repo) CompleteAccountDeletionStep(ctx context.Context, jobID int64, service, leaseOwner string, completedAt time.Time) error {
	service = strings.TrimSpace(service)
	leaseOwner = strings.TrimSpace(leaseOwner)
	if jobID <= 0 || service == "" || leaseOwner == "" || completedAt.IsZero() {
		return domain.ErrAccountDeletionLeaseLost
	}
	result := r.db.WithContext(ctx).Model(&accountJobStepPO{}).
		Where("job_id = ? AND service = ? AND status = ? AND lease_owner = ?", jobID, service, string(domain.AccountJobRunning), leaseOwner).
		Updates(map[string]any{
			"status": string(domain.AccountJobSucceeded), "lease_owner": nil, "lease_expires_at": nil,
			"last_error": "", "completed_at": completedAt, "updated_at": completedAt,
		})
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected != 1 {
		return domain.ErrAccountDeletionLeaseLost
	}
	return nil
}

func (r *Repo) RetryAccountDeletionStep(ctx context.Context, jobID int64, service, leaseOwner, lastError string, now, retryAt time.Time, maxAttempts int16) error {
	service = strings.TrimSpace(service)
	leaseOwner = strings.TrimSpace(leaseOwner)
	lastError = truncateLifecycleError(lastError)
	if jobID <= 0 || service == "" || leaseOwner == "" || now.IsZero() || retryAt.Before(now) || maxAttempts <= 0 {
		return domain.ErrAccountDeletionLeaseLost
	}
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var step accountJobStepPO
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
			Where("job_id = ? AND service = ? AND status = ? AND lease_owner = ?", jobID, service, string(domain.AccountJobRunning), leaseOwner).
			First(&step).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return domain.ErrAccountDeletionLeaseLost
			}
			return err
		}
		status := domain.AccountJobRetryWait
		availableAt := retryAt
		if step.Attempts >= maxAttempts {
			status = domain.AccountJobBlocked
			availableAt = now
		}
		if err := tx.Model(&accountJobStepPO{}).
			Where("job_id = ? AND service = ?", jobID, service).
			Updates(map[string]any{
				"status": string(status), "available_at": availableAt, "lease_owner": nil,
				"lease_expires_at": nil, "last_error": lastError, "updated_at": now,
			}).Error; err != nil {
			return err
		}
		job := tx.Model(&accountJobPO{}).
			Where("id = ? AND status = ? AND lease_owner = ?", jobID, string(domain.AccountJobRunning), leaseOwner).
			Updates(map[string]any{
				"status": string(status), "available_at": availableAt, "lease_owner": nil,
				"lease_expires_at": nil, "last_error": lastError, "updated_at": now,
			})
		if job.Error != nil {
			return job.Error
		}
		if job.RowsAffected != 1 {
			return domain.ErrAccountDeletionLeaseLost
		}
		return nil
	})
}

func (r *Repo) FinalizeAccountDeletionJob(ctx context.Context, jobID int64, leaseOwner string, anonymization domain.AccountAnonymization) (*domain.User, error) {
	leaseOwner = strings.TrimSpace(leaseOwner)
	anonymization.Username = strings.TrimSpace(anonymization.Username)
	anonymization.Email = strings.TrimSpace(anonymization.Email)
	anonymization.PasswordHash = strings.TrimSpace(anonymization.PasswordHash)
	anonymization.CredentialVersion = strings.TrimSpace(anonymization.CredentialVersion)
	if jobID <= 0 || leaseOwner == "" || anonymization.Username == "" || anonymization.Email == "" || anonymization.PasswordHash == "" || anonymization.CredentialVersion == "" || anonymization.CompletedAt.IsZero() {
		return nil, domain.ErrAccountLifecycleChanged
	}
	var finalized *domain.User
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var job accountJobPO
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
			Where("id = ? AND kind = ? AND status = ? AND lease_owner = ?", jobID, "delete_account", string(domain.AccountJobRunning), leaseOwner).
			First(&job).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return domain.ErrAccountDeletionLeaseLost
			}
			return err
		}
		var incomplete int64
		if err := tx.Model(&accountJobStepPO{}).
			Where("job_id = ? AND status <> ?", jobID, string(domain.AccountJobSucceeded)).
			Count(&incomplete).Error; err != nil {
			return err
		}
		if incomplete != 0 {
			return domain.ErrAccountDeletionStepsIncomplete
		}
		var user userPO
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).First(&user, job.UserID).Error; err != nil {
			return err
		}
		if domain.NormalizeAccountState(domain.AccountState(user.AccountState)) != domain.AccountStateDeletionPending || user.ProtectedAccount {
			return domain.ErrAccountLifecycleChanged
		}
		if err := decrementFollowCountersForErasure(tx, user.ID); err != nil {
			return err
		}
		for _, deletion := range []struct {
			model any
			query string
			args  []any
		}{
			{&oauthAccountPO{}, "user_id = ?", []any{user.ID}},
			{&passwordResetTokenPO{}, "user_id = ?", []any{user.ID}},
			{&emailVerificationTokenPO{}, "user_id = ?", []any{user.ID}},
			{&passkeyChallengePO{}, "user_id = ?", []any{user.ID}},
			{&mfaLoginChallengePO{}, "user_id = ?", []any{user.ID}},
			{&mfaTOTPPO{}, "user_id = ?", []any{user.ID}},
			{&userListPO{}, "owner_id = ?", []any{user.ID}},
			{&userListMembershipPO{}, "user_id = ?", []any{user.ID}},
			{&userListFavoritePO{}, "user_id = ?", []any{user.ID}},
			{&followPO{}, "follower_id = ? OR followee_id = ?", []any{user.ID, user.ID}},
			{&blockPO{}, "actor_id = ? OR target_id = ?", []any{user.ID, user.ID}},
			{&mutePO{}, "actor_id = ? OR target_id = ?", []any{user.ID, user.ID}},
		} {
			if err := tx.Where(deletion.query, deletion.args...).Delete(deletion.model).Error; err != nil {
				return err
			}
		}
		result := tx.Model(&userPO{}).
			Where("id = ? AND account_state = ?", user.ID, string(domain.AccountStateDeletionPending)).
			Updates(map[string]any{
				"username": anonymization.Username, "email": anonymization.Email,
				"password_hash": anonymization.PasswordHash, "credential_version": anonymization.CredentialVersion,
				"nickname": "已注销用户", "avatar_url": "", "background_url": "", "profile_theme": domain.ProfileThemeDefault, "bio": "",
				"status": int32(domain.StatusActive), "account_state": string(domain.AccountStateAnonymized),
				"account_state_version": gorm.Expr("account_state_version + 1"), "deleted_at": anonymization.CompletedAt,
				"follower_count": 0, "following_count": 0, "last_login_at": nil, "email_verified_at": nil,
				"updated_at": anonymization.CompletedAt,
			})
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected != 1 {
			return domain.ErrAccountLifecycleChanged
		}
		if err := tx.Model(&accountJobPO{}).Where("id = ?", job.ID).Updates(map[string]any{
			"status": string(domain.AccountJobSucceeded), "lease_owner": nil, "lease_expires_at": nil,
			"last_error": "", "completed_at": anonymization.CompletedAt, "updated_at": anonymization.CompletedAt,
		}).Error; err != nil {
			return err
		}
		metadata, err := json.Marshal(struct {
			JobID int64 `json:"job_id"`
		}{JobID: job.ID})
		if err != nil {
			return err
		}
		if err := tx.Create(&accountActionPO{
			ActorUserID: user.ID, TargetUserID: user.ID, Action: "complete_deletion",
			FromState: string(domain.AccountStateDeletionPending), ToState: string(domain.AccountStateAnonymized),
			Reason: "self_service", Metadata: string(metadata), CreatedAt: anonymization.CompletedAt,
		}).Error; err != nil {
			return err
		}
		if err := tx.First(&user, user.ID).Error; err != nil {
			return err
		}
		finalized = toEntity(&user)
		deletedEvent := domain.NewDeletedEvent(finalized)
		payload, err := json.Marshal(deletedEvent)
		if err != nil {
			return err
		}
		if err := tx.Create(&accountDeletionOutboxPO{
			EventID: uuid.NewString(), JobID: job.ID, AggregateID: finalized.ID,
			EventType: deletedEvent.EventName(), MessageKey: fmt.Sprint(finalized.ID),
			PayloadJSON: string(payload), Status: "pending", AvailableAt: anonymization.CompletedAt,
			LastError: "", OccurredAt: anonymization.CompletedAt,
			CreatedAt: anonymization.CompletedAt, UpdatedAt: anonymization.CompletedAt,
		}).Error; err != nil {
			return err
		}
		return nil
	})
	return finalized, err
}

func loadAccountDeletionSteps(tx *gorm.DB, jobID int64) ([]domain.AccountDeletionStep, error) {
	var rows []accountJobStepPO
	if err := tx.Where("job_id = ?", jobID).Find(&rows).Error; err != nil {
		return nil, err
	}
	order := make(map[string]int, len(domain.AccountDeletionSteps()))
	for index, service := range domain.AccountDeletionSteps() {
		order[service] = index
	}
	sort.Slice(rows, func(i, j int) bool {
		left, leftOK := order[rows[i].Service]
		right, rightOK := order[rows[j].Service]
		if leftOK && rightOK {
			return left < right
		}
		if leftOK != rightOK {
			return leftOK
		}
		return rows[i].Service < rows[j].Service
	})
	steps := make([]domain.AccountDeletionStep, 0, len(rows))
	for _, row := range rows {
		steps = append(steps, domain.AccountDeletionStep{
			Service: row.Service, Status: domain.AccountJobStatus(row.Status), Attempts: row.Attempts, AvailableAt: row.AvailableAt,
		})
	}
	return steps, nil
}

func accountDeletionJobFromPO(job accountJobPO, steps []domain.AccountDeletionStep) domain.AccountDeletionJob {
	var completed int32
	for _, step := range steps {
		if step.Status == domain.AccountJobSucceeded {
			completed++
		}
	}
	return domain.AccountDeletionJob{
		ID: job.ID, UserID: job.UserID, Status: domain.AccountJobStatus(job.Status), PolicyVersion: job.PolicyVersion,
		CompletedSteps: completed, TotalSteps: int32(len(steps)), CreatedAt: job.CreatedAt, UpdatedAt: job.UpdatedAt,
		StartedAt: job.StartedAt, CompletedAt: job.CompletedAt,
	}
}

func decrementFollowCountersForErasure(tx *gorm.DB, userID int64) error {
	if err := tx.Exec(`UPDATE users SET follower_count = GREATEST(follower_count - 1, 0)
		WHERE id IN (SELECT followee_id FROM user_follows WHERE follower_id = ?)`, userID).Error; err != nil {
		return err
	}
	return tx.Exec(`UPDATE users SET following_count = GREATEST(following_count - 1, 0)
		WHERE id IN (SELECT follower_id FROM user_follows WHERE followee_id = ?)`, userID).Error
}

func truncateLifecycleError(value string) string {
	value = strings.TrimSpace(value)
	if len(value) <= 4000 {
		return value
	}
	return value[:4000]
}
