package deletion

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"strings"
	"time"

	domain "user-service/internal/domain/user"
	"user-service/pkg/logger"
)

const (
	defaultPollInterval = 5 * time.Second
	defaultLease        = 2 * time.Minute
	defaultStepTimeout  = 30 * time.Second
	defaultRetryBase    = 10 * time.Second
	defaultMaxAttempts  = int16(8)
	defaultDrainLimit   = 10
)

type AccountDataEraser interface {
	EraseUserData(ctx context.Context, userID, jobID int64, policyVersion int32) error
}

type CredentialVersionCache interface {
	SetCurrent(ctx context.Context, userID int64, version string) error
}

type Options struct {
	WorkerID     string
	PollInterval time.Duration
	Lease        time.Duration
	StepTimeout  time.Duration
	RetryBase    time.Duration
	MaxAttempts  int16
	DrainLimit   int
}

type Worker struct {
	repo       domain.AccountDeletionJobRepository
	erasers    map[string]AccountDataEraser
	cache      CredentialVersionCache
	log        logger.Logger
	options    Options
	now        func() time.Time
	credential func() (string, error)
}

func NewWorker(repo domain.AccountDeletionJobRepository, erasers map[string]AccountDataEraser, cache CredentialVersionCache, log logger.Logger, options Options) (*Worker, error) {
	if repo == nil {
		return nil, domain.ErrAccountLifecycleRepositoryUnavailable
	}
	options.WorkerID = strings.TrimSpace(options.WorkerID)
	if options.WorkerID == "" {
		return nil, fmt.Errorf("account deletion worker ID required")
	}
	if options.PollInterval <= 0 {
		options.PollInterval = defaultPollInterval
	}
	if options.Lease <= 0 {
		options.Lease = defaultLease
	}
	if options.StepTimeout <= 0 || options.StepTimeout >= options.Lease {
		options.StepTimeout = defaultStepTimeout
		if options.StepTimeout >= options.Lease {
			options.StepTimeout = options.Lease / 2
		}
	}
	if options.RetryBase <= 0 {
		options.RetryBase = defaultRetryBase
	}
	if options.MaxAttempts <= 0 {
		options.MaxAttempts = defaultMaxAttempts
	}
	if options.DrainLimit <= 0 {
		options.DrainLimit = defaultDrainLimit
	}
	copyErasers := make(map[string]AccountDataEraser, len(erasers))
	for service, eraser := range erasers {
		service = strings.TrimSpace(service)
		if service != "" && eraser != nil {
			copyErasers[service] = eraser
		}
	}
	return &Worker{
		repo: repo, erasers: copyErasers, cache: cache, log: log, options: options,
		now: time.Now, credential: randomCredentialVersion,
	}, nil
}

func (w *Worker) Run(ctx context.Context) {
	ticker := time.NewTicker(w.options.PollInterval)
	defer ticker.Stop()
	for {
		w.drain(ctx)
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		}
	}
}

func (w *Worker) RunOnce(ctx context.Context) (bool, error) {
	now := w.now()
	claim, err := w.repo.ClaimAccountDeletionJob(ctx, w.options.WorkerID, now, now.Add(w.options.Lease))
	if err != nil || claim == nil {
		return false, err
	}
	for _, step := range claim.Steps {
		if step.Status == domain.AccountJobSucceeded {
			continue
		}
		eraser := w.erasers[step.Service]
		stepNow := w.now()
		attempts, beginErr := w.repo.BeginAccountDeletionStep(ctx, claim.Job.ID, step.Service, claim.LeaseOwner, stepNow, stepNow.Add(w.options.Lease))
		if beginErr != nil {
			return true, beginErr
		}
		var eraseErr error
		if eraser == nil {
			eraseErr = fmt.Errorf("account data eraser unavailable for %s", step.Service)
		} else {
			stepCtx, cancel := context.WithTimeout(ctx, w.options.StepTimeout)
			eraseErr = eraser.EraseUserData(stepCtx, claim.Job.UserID, claim.Job.ID, claim.Job.PolicyVersion)
			cancel()
		}
		if eraseErr != nil {
			failedAt := w.now()
			retryAt := failedAt.Add(w.retryDelay(attempts))
			if retryErr := w.repo.RetryAccountDeletionStep(ctx, claim.Job.ID, step.Service, claim.LeaseOwner, eraseErr.Error(), failedAt, retryAt, w.options.MaxAttempts); retryErr != nil {
				return true, retryErr
			}
			return true, eraseErr
		}
		if err := w.repo.CompleteAccountDeletionStep(ctx, claim.Job.ID, step.Service, claim.LeaseOwner, w.now()); err != nil {
			return true, err
		}
	}
	credentialVersion, err := w.credential()
	if err != nil {
		return true, err
	}
	completedAt := w.now()
	user, err := w.repo.FinalizeAccountDeletionJob(ctx, claim.Job.ID, claim.LeaseOwner, domain.AccountAnonymization{
		Username:          anonymizedUsername(claim.Job.UserID),
		Email:             anonymizedEmail(claim.Job.UserID, claim.Job.ID),
		PasswordHash:      "!account-anonymized!",
		CredentialVersion: credentialVersion,
		CompletedAt:       completedAt,
	})
	if err != nil {
		return true, err
	}
	if w.cache != nil {
		if err := w.cache.SetCurrent(ctx, user.ID, user.CredentialVersion); err != nil && w.log != nil {
			w.log.Warn("refresh anonymized credential version cache failed", logger.Int64("user_id", user.ID), logger.Error(err))
		}
	}
	return true, nil
}

func (w *Worker) drain(ctx context.Context) {
	for count := 0; count < w.options.DrainLimit; count++ {
		processed, err := w.RunOnce(ctx)
		if err != nil {
			if w.log != nil {
				w.log.Error("account deletion job failed", logger.Error(err))
			}
			return
		}
		if !processed {
			return
		}
	}
}

func (w *Worker) retryDelay(attempts int16) time.Duration {
	shift := attempts - 1
	if shift < 0 {
		shift = 0
	}
	if shift > 8 {
		shift = 8
	}
	delay := w.options.RetryBase * time.Duration(1<<shift)
	if delay > time.Hour {
		return time.Hour
	}
	return delay
}

func anonymizedUsername(userID int64) string {
	return fmt.Sprintf("__erased_%x", userID)
}

func anonymizedEmail(userID, jobID int64) string {
	return fmt.Sprintf("erased+%x+%x@invalid.local", userID, jobID)
}

func randomCredentialVersion() (string, error) {
	var raw [32]byte
	if _, err := rand.Read(raw[:]); err != nil {
		return "", err
	}
	return hex.EncodeToString(raw[:]), nil
}
