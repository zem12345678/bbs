package persistence

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"time"

	domain "user-service/internal/domain/user"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type accountJobPO struct {
	ID             int64     `gorm:"primaryKey"`
	UserID         int64     `gorm:"not null;index"`
	Kind           string    `gorm:"size:32;not null"`
	Status         string    `gorm:"size:20;not null;index"`
	PolicyVersion  int32     `gorm:"not null"`
	Attempts       int16     `gorm:"not null"`
	AvailableAt    time.Time `gorm:"not null;index"`
	LeaseOwner     *string
	LeaseExpiresAt *time.Time
	LastError      string    `gorm:"type:text;not null"`
	CreatedAt      time.Time `gorm:"not null;index"`
	UpdatedAt      time.Time `gorm:"not null"`
	StartedAt      *time.Time
	CompletedAt    *time.Time
}

func (accountJobPO) TableName() string { return "user_account_jobs" }

type accountJobStepPO struct {
	JobID          int64     `gorm:"primaryKey"`
	Service        string    `gorm:"primaryKey;size:40"`
	Status         string    `gorm:"size:20;not null;index"`
	Attempts       int16     `gorm:"not null"`
	AvailableAt    time.Time `gorm:"not null;index"`
	LeaseOwner     *string
	LeaseExpiresAt *time.Time
	LastError      string    `gorm:"type:text;not null"`
	CreatedAt      time.Time `gorm:"not null"`
	UpdatedAt      time.Time `gorm:"not null"`
	CompletedAt    *time.Time
}

func (accountJobStepPO) TableName() string { return "user_account_job_steps" }

type accountActionPO struct {
	ID           int64     `gorm:"primaryKey;autoIncrement"`
	ActorUserID  int64     `gorm:"not null"`
	TargetUserID int64     `gorm:"not null;index"`
	Action       string    `gorm:"size:40;not null"`
	FromState    string    `gorm:"size:24;not null"`
	ToState      string    `gorm:"size:24;not null"`
	Reason       string    `gorm:"type:text;not null"`
	Metadata     string    `gorm:"type:jsonb;not null"`
	CreatedAt    time.Time `gorm:"not null;index"`
}

func (accountActionPO) TableName() string { return "user_account_actions" }

var _ domain.AccountLifecycleRepository = (*Repo)(nil)

func (r *Repo) GetAccountLifecycle(ctx context.Context, userID int64) (domain.AccountLifecycle, error) {
	if userID <= 0 {
		return domain.AccountLifecycle{}, domain.ErrInvalidID
	}
	return r.accountLifecycle(r.db.WithContext(ctx), userID)
}

func (r *Repo) RequestAccountDeletion(ctx context.Context, request domain.AccountDeletionRequest) (domain.AccountLifecycle, error) {
	request.ExpectedCredentialVersion = strings.TrimSpace(request.ExpectedCredentialVersion)
	request.CredentialVersion = strings.TrimSpace(request.CredentialVersion)
	if request.JobID <= 0 || request.UserID <= 0 || request.ActorUserID <= 0 {
		return domain.AccountLifecycle{}, domain.ErrInvalidID
	}
	if request.PolicyVersion <= 0 || request.RequestedAt.IsZero() || request.ExpectedCredentialVersion == "" || request.CredentialVersion == "" || request.CredentialVersion == request.ExpectedCredentialVersion {
		return domain.AccountLifecycle{}, domain.ErrAccountLifecycleChanged
	}
	if len(request.Steps) == 0 {
		return domain.AccountLifecycle{}, domain.ErrAccountLifecycleChanged
	}

	var result domain.AccountLifecycle
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var user userPO
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).First(&user, request.UserID).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return domain.ErrNotFound
			}
			return err
		}
		state := domain.NormalizeAccountState(domain.AccountState(user.AccountState))
		if state == domain.AccountStateDeletionPending {
			var err error
			result, err = r.accountLifecycle(tx, request.UserID)
			if err == nil && result.ActiveDeletionJob == nil {
				return domain.ErrAccountLifecycleChanged
			}
			return err
		}
		if state != domain.AccountStateActive {
			return accountStateError(state)
		}
		if user.ProtectedAccount {
			return domain.ErrAccountProtected
		}
		if strings.TrimSpace(user.CredentialVersion) != request.ExpectedCredentialVersion {
			return domain.ErrAccountLifecycleChanged
		}

		stateVersion := user.AccountStateVersion
		if stateVersion <= 0 {
			stateVersion = 1
		}
		updated := tx.Model(&userPO{}).
			Where("id = ? AND account_state_version = ? AND credential_version = ?", request.UserID, user.AccountStateVersion, request.ExpectedCredentialVersion).
			Updates(map[string]any{
				"account_state":         string(domain.AccountStateDeletionPending),
				"account_state_version": stateVersion + 1,
				"credential_version":    request.CredentialVersion,
				"deletion_requested_at": request.RequestedAt,
				"deleted_at":            nil,
				"updated_at":            request.RequestedAt,
			})
		if updated.Error != nil {
			return updated.Error
		}
		if updated.RowsAffected != 1 {
			return domain.ErrAccountLifecycleChanged
		}

		job := accountJobPO{
			ID: request.JobID, UserID: request.UserID, Kind: "delete_account", Status: string(domain.AccountJobPending),
			PolicyVersion: request.PolicyVersion, AvailableAt: request.RequestedAt, CreatedAt: request.RequestedAt,
			UpdatedAt: request.RequestedAt, LastError: "",
		}
		if err := tx.Create(&job).Error; err != nil {
			return mapWriteError(err)
		}
		seen := make(map[string]struct{}, len(request.Steps))
		steps := make([]accountJobStepPO, 0, len(request.Steps))
		for _, service := range request.Steps {
			service = strings.TrimSpace(service)
			if service == "" || len(service) > 40 {
				return domain.ErrAccountLifecycleChanged
			}
			if _, exists := seen[service]; exists {
				return domain.ErrAccountLifecycleChanged
			}
			seen[service] = struct{}{}
			steps = append(steps, accountJobStepPO{
				JobID: request.JobID, Service: service, Status: string(domain.AccountJobPending),
				AvailableAt: request.RequestedAt, CreatedAt: request.RequestedAt, UpdatedAt: request.RequestedAt, LastError: "",
			})
		}
		if err := tx.Create(&steps).Error; err != nil {
			return err
		}
		metadata, err := json.Marshal(struct {
			PolicyVersion int32 `json:"policy_version"`
		}{PolicyVersion: request.PolicyVersion})
		if err != nil {
			return err
		}
		action := accountActionPO{
			ActorUserID: request.ActorUserID, TargetUserID: request.UserID, Action: "request_deletion",
			FromState: string(domain.AccountStateActive), ToState: string(domain.AccountStateDeletionPending),
			Reason: "self_service", Metadata: string(metadata), CreatedAt: request.RequestedAt,
		}
		if err := tx.Create(&action).Error; err != nil {
			return err
		}
		result, err = r.accountLifecycle(tx, request.UserID)
		return err
	})
	return result, err
}

func (r *Repo) accountLifecycle(db *gorm.DB, userID int64) (domain.AccountLifecycle, error) {
	var user userPO
	if err := db.First(&user, userID).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return domain.AccountLifecycle{}, domain.ErrNotFound
		}
		return domain.AccountLifecycle{}, err
	}
	result := domain.AccountLifecycle{
		UserID: user.ID, State: domain.NormalizeAccountState(domain.AccountState(user.AccountState)),
		StateVersion: user.AccountStateVersion, CredentialVersion: strings.TrimSpace(user.CredentialVersion),
		Protected: user.ProtectedAccount, DeletionRequestedAt: user.DeletionRequestedAt, DeletedAt: user.DeletedAt,
	}
	var job accountJobPO
	err := db.Where("user_id = ? AND kind = ? AND status IN ?", userID, "delete_account", []string{
		string(domain.AccountJobPending), string(domain.AccountJobRunning), string(domain.AccountJobRetryWait), string(domain.AccountJobBlocked),
	}).Order("created_at DESC, id DESC").First(&job).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return result, nil
	}
	if err != nil {
		return domain.AccountLifecycle{}, err
	}
	var total, completed int64
	if err := db.Model(&accountJobStepPO{}).Where("job_id = ?", job.ID).Count(&total).Error; err != nil {
		return domain.AccountLifecycle{}, err
	}
	if err := db.Model(&accountJobStepPO{}).Where("job_id = ? AND status = ?", job.ID, string(domain.AccountJobSucceeded)).Count(&completed).Error; err != nil {
		return domain.AccountLifecycle{}, err
	}
	result.ActiveDeletionJob = &domain.AccountDeletionJob{
		ID: job.ID, UserID: job.UserID, Status: domain.AccountJobStatus(job.Status), PolicyVersion: job.PolicyVersion,
		CompletedSteps: int32(completed), TotalSteps: int32(total), CreatedAt: job.CreatedAt, UpdatedAt: job.UpdatedAt,
		StartedAt: job.StartedAt, CompletedAt: job.CompletedAt,
	}
	return result, nil
}

func accountStateError(state domain.AccountState) error {
	switch state {
	case domain.AccountStateSuspended:
		return domain.ErrAccountSuspended
	case domain.AccountStateDeletionPending:
		return domain.ErrAccountDeletionPending
	case domain.AccountStateAnonymized:
		return domain.ErrAccountAnonymized
	default:
		return domain.ErrInvalidAccountState
	}
}
