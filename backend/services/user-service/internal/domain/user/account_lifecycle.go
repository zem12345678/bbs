package user

import (
	"context"
	"strings"
	"time"
)

type AccountState string

const (
	AccountStateActive          AccountState = "active"
	AccountStateSuspended       AccountState = "suspended"
	AccountStateDeletionPending AccountState = "deletion_pending"
	AccountStateAnonymized      AccountState = "anonymized"
)

func NormalizeAccountState(value AccountState) AccountState {
	state := AccountState(strings.ToLower(strings.TrimSpace(string(value))))
	if state == "" {
		return AccountStateActive
	}
	return state
}

func (s AccountState) IsValid() bool {
	switch NormalizeAccountState(s) {
	case AccountStateActive, AccountStateSuspended, AccountStateDeletionPending, AccountStateAnonymized:
		return true
	default:
		return false
	}
}

type AccountJobStatus string

const (
	AccountJobPending   AccountJobStatus = "pending"
	AccountJobRunning   AccountJobStatus = "running"
	AccountJobRetryWait AccountJobStatus = "retry_wait"
	AccountJobBlocked   AccountJobStatus = "blocked"
	AccountJobSucceeded AccountJobStatus = "succeeded"
	AccountJobFailed    AccountJobStatus = "failed"
)

type AccountLifecycle struct {
	UserID              int64
	State               AccountState
	StateVersion        int64
	CredentialVersion   string
	Protected           bool
	DeletionRequestedAt *time.Time
	DeletedAt           *time.Time
	ActiveDeletionJob   *AccountDeletionJob
}

type AccountDeletionJob struct {
	ID             int64
	UserID         int64
	Status         AccountJobStatus
	PolicyVersion  int32
	CompletedSteps int32
	TotalSteps     int32
	CreatedAt      time.Time
	UpdatedAt      time.Time
	StartedAt      *time.Time
	CompletedAt    *time.Time
}

type AccountDeletionStep struct {
	Service     string
	Status      AccountJobStatus
	Attempts    int16
	AvailableAt time.Time
}

type AccountDeletionClaim struct {
	Job        AccountDeletionJob
	Steps      []AccountDeletionStep
	LeaseOwner string
	LeaseUntil time.Time
}

type AccountAnonymization struct {
	Username          string
	Email             string
	PasswordHash      string
	CredentialVersion string
	CompletedAt       time.Time
}

type AccountDeletionRequest struct {
	JobID                     int64
	UserID                    int64
	ActorUserID               int64
	ExpectedCredentialVersion string
	CredentialVersion         string
	RequestedAt               time.Time
	PolicyVersion             int32
	Steps                     []string
}

var accountDeletionSteps = []string{
	"content-service",
	"comment-service",
	"reaction-service",
	"chat-service",
	"notification-service",
	"file-service",
	"credit-service",
	"mall-service",
	"feed-service",
	"search-service",
}

func AccountDeletionSteps() []string {
	return append([]string(nil), accountDeletionSteps...)
}

type AccountLifecycleRepository interface {
	GetAccountLifecycle(ctx context.Context, userID int64) (AccountLifecycle, error)
	RequestAccountDeletion(ctx context.Context, request AccountDeletionRequest) (AccountLifecycle, error)
}

type AccountDeletionJobRepository interface {
	ClaimAccountDeletionJob(ctx context.Context, leaseOwner string, now, leaseUntil time.Time) (*AccountDeletionClaim, error)
	BeginAccountDeletionStep(ctx context.Context, jobID int64, service, leaseOwner string, now, leaseUntil time.Time) (int16, error)
	CompleteAccountDeletionStep(ctx context.Context, jobID int64, service, leaseOwner string, completedAt time.Time) error
	RetryAccountDeletionStep(ctx context.Context, jobID int64, service, leaseOwner, lastError string, now, retryAt time.Time, maxAttempts int16) error
	FinalizeAccountDeletionJob(ctx context.Context, jobID int64, leaseOwner string, anonymization AccountAnonymization) (*User, error)
}
