package app

import (
	"context"
	"errors"
	"fmt"
	"sync"

	"user-service/internal/application/user/deletion"
	erasureclient "user-service/internal/clients/erasure"
	mallclient "user-service/internal/clients/mall"
	"user-service/internal/infrastructure/messaging"

	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"
	"gorm.io/gorm"
)

type RuntimeRunner struct {
	worker    *deletion.Worker
	outbox    *deletion.AccountDeletionOutboxDispatcher
	erasers   *erasureclient.Set
	mall      *mallclient.Client
	publisher *messaging.KafkaEventPublisher
	db        *gorm.DB
	redis     *redis.Client
	logger    *zap.Logger

	ctx      context.Context
	cancel   context.CancelFunc
	startOne sync.Once
	stopOne  sync.Once
	wait     sync.WaitGroup
	stopErr  error
}

func NewRuntimeRunner(worker *deletion.Worker, outbox *deletion.AccountDeletionOutboxDispatcher, erasers *erasureclient.Set, mall *mallclient.Client, publisher *messaging.KafkaEventPublisher, db *gorm.DB, redisClient *redis.Client, logger *zap.Logger) *RuntimeRunner {
	ctx, cancel := context.WithCancel(context.Background())
	return &RuntimeRunner{
		worker: worker, outbox: outbox, erasers: erasers, mall: mall,
		publisher: publisher, db: db, redis: redisClient, logger: logger,
		ctx: ctx, cancel: cancel,
	}
}

func (r *RuntimeRunner) Start() error {
	if r == nil {
		return nil
	}
	r.startOne.Do(func() {
		if r.worker != nil {
			r.wait.Add(1)
			go func() {
				defer r.wait.Done()
				r.worker.Run(r.ctx)
			}()
		}
		if r.outbox != nil {
			r.wait.Add(1)
			go func() {
				defer r.wait.Done()
				r.outbox.Run(r.ctx)
			}()
		}
	})
	return nil
}

func (r *RuntimeRunner) Stop() error {
	if r == nil {
		return nil
	}
	r.stopOne.Do(func() {
		r.cancel()
		r.wait.Wait()
		closeErrors := make([]error, 0, 5)
		if r.erasers != nil {
			if err := r.erasers.Close(); err != nil {
				closeErrors = append(closeErrors, fmt.Errorf("close account erasure connections: %w", err))
			}
		}
		if r.mall != nil {
			if err := r.mall.Close(); err != nil {
				closeErrors = append(closeErrors, fmt.Errorf("close mall client: %w", err))
			}
		}
		if r.publisher != nil {
			if err := r.publisher.Close(); err != nil {
				closeErrors = append(closeErrors, fmt.Errorf("close user event publisher: %w", err))
			}
		}
		if r.redis != nil {
			if err := r.redis.Close(); err != nil {
				closeErrors = append(closeErrors, fmt.Errorf("close user redis: %w", err))
			}
		}
		if r.db != nil {
			if sqlDB, err := r.db.DB(); err != nil {
				closeErrors = append(closeErrors, fmt.Errorf("get user database connection: %w", err))
			} else if err := sqlDB.Close(); err != nil {
				closeErrors = append(closeErrors, fmt.Errorf("close user database: %w", err))
			}
		}
		r.stopErr = errors.Join(closeErrors...)
	})
	return r.stopErr
}
