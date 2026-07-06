package admin

import (
	"context"
	"strings"
	"time"
)

const (
	loginFailureWindow       = 15 * time.Minute
	loginFailureAccountIPMax = int64(5)
	loginFailureAccountMax   = int64(10)
)

type loginFailureCounter interface {
	CountRecentLoginFailures(ctx context.Context, username string, ip string, since time.Time) (int64, error)
}

func tooManyLoginFailures(ctx context.Context, counter loginFailureCounter, username string, ip string, now time.Time) (bool, error) {
	if counter == nil || strings.TrimSpace(username) == "" {
		return false, nil
	}
	since := now.Add(-loginFailureWindow)
	accountFailures, err := counter.CountRecentLoginFailures(ctx, username, "", since)
	if err != nil {
		return false, err
	}
	if accountFailures >= loginFailureAccountMax {
		return true, nil
	}
	if strings.TrimSpace(ip) == "" {
		return false, nil
	}
	accountIPFailures, err := counter.CountRecentLoginFailures(ctx, username, ip, since)
	if err != nil {
		return false, err
	}
	return accountIPFailures >= loginFailureAccountIPMax, nil
}
