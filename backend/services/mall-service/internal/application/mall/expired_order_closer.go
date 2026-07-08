package mall

import (
	"context"
	"time"
)

type ExpiredOrderCloser struct {
	service     *Service
	expireAfter time.Duration
	interval    time.Duration
	limit       int
}

type ExpiredOrderCloserOptions struct {
	ExpireAfter time.Duration
	Interval    time.Duration
	Limit       int
}

func NewExpiredOrderCloser(service *Service, options ExpiredOrderCloserOptions) *ExpiredOrderCloser {
	if options.ExpireAfter <= 0 {
		options.ExpireAfter = DefaultOrderExpireAfter
	}
	if options.Interval <= 0 {
		options.Interval = time.Minute
	}
	if options.Limit <= 0 {
		options.Limit = DefaultOrderExpireLimit
	}
	return &ExpiredOrderCloser{
		service:     service,
		expireAfter: options.ExpireAfter,
		interval:    options.Interval,
		limit:       options.Limit,
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

func (c *ExpiredOrderCloser) run(ctx context.Context) {
	ticker := time.NewTicker(c.interval)
	defer ticker.Stop()
	for ctx.Err() == nil {
		_, _ = c.CloseOnce(ctx)
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		}
	}
}
