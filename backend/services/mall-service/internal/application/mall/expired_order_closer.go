package mall

import (
	"context"
	"mall-service/pkg/logger"
	"time"
)

type ExpiredOrderCloser struct {
	service            *Service
	expireAfter        time.Duration
	recoverPayingAfter time.Duration
	interval           time.Duration
	limit              int
	log                logger.Logger
}

type ExpiredOrderCloserOptions struct {
	ExpireAfter        time.Duration
	RecoverPayingAfter time.Duration
	Interval           time.Duration
	Limit              int
	Log                logger.Logger
}

func NewExpiredOrderCloser(service *Service, options ExpiredOrderCloserOptions) *ExpiredOrderCloser {
	if options.ExpireAfter <= 0 {
		options.ExpireAfter = DefaultOrderExpireAfter
	}
	if options.RecoverPayingAfter <= 0 {
		options.RecoverPayingAfter = options.ExpireAfter
	}
	if options.Interval <= 0 {
		options.Interval = time.Minute
	}
	if options.Limit <= 0 {
		options.Limit = DefaultOrderExpireLimit
	}
	if options.Log == nil {
		options.Log = logger.NewNopLogger()
	}
	return &ExpiredOrderCloser{
		service:            service,
		expireAfter:        options.ExpireAfter,
		recoverPayingAfter: options.RecoverPayingAfter,
		interval:           options.Interval,
		limit:              options.Limit,
		log:                options.Log,
	}
}

func (c *ExpiredOrderCloser) Start(ctx context.Context) {
	go c.run(ctx)
}

func (c *ExpiredOrderCloser) CloseOnce(ctx context.Context) (int, error) {
	orders, err := c.service.CloseExpiredOrders(ctx, CloseExpiredOrdersCommand{
		ExpireAfter: c.expireAfter,
		Limit:       c.limit,
	})
	if err != nil {
		return 0, err
	}
	return len(orders), nil
}

func (c *ExpiredOrderCloser) RecoverPayingOnce(ctx context.Context) (RecoverStalePayingOrdersResult, error) {
	return c.service.RecoverStalePayingOrders(ctx, RecoverStalePayingOrdersCommand{
		StaleAfter: c.recoverPayingAfter,
		Limit:      c.limit,
	})
}

func (c *ExpiredOrderCloser) run(ctx context.Context) {
	ticker := time.NewTicker(c.interval)
	defer ticker.Stop()
	for ctx.Err() == nil {
		closed, err := c.CloseOnce(ctx)
		if err != nil {
			c.log.Error("close expired mall orders failed",
				logger.Error(err),
				logger.String("expire_after", c.expireAfter.String()),
				logger.Int("limit", c.limit),
			)
		} else if closed > 0 {
			c.log.Info("closed expired mall orders",
				logger.Int("closed_orders", closed),
				logger.String("expire_after", c.expireAfter.String()),
				logger.Int("limit", c.limit),
			)
		}
		if ctx.Err() != nil {
			return
		}
		recovered, err := c.RecoverPayingOnce(ctx)
		if err != nil {
			c.log.Error("recover stale paying mall orders failed",
				logger.Error(err),
				logger.String("stale_after", c.recoverPayingAfter.String()),
				logger.Int("limit", c.limit),
			)
		} else if recovered.Recovered > 0 || recovered.Failed > 0 {
			c.log.Info("recovered stale paying mall orders",
				logger.Int("recovered_orders", recovered.Recovered),
				logger.Int("failed_orders", recovered.Failed),
				logger.String("stale_after", c.recoverPayingAfter.String()),
				logger.Int("limit", c.limit),
			)
		}
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		}
	}
}
