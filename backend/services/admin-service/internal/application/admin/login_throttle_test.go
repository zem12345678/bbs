package admin

import (
	"context"
	"errors"
	"testing"
	"time"
)

type loginFailureCounterFunc func(ctx context.Context, username string, ip string, since time.Time) (int64, error)

func (f loginFailureCounterFunc) CountRecentLoginFailures(ctx context.Context, username string, ip string, since time.Time) (int64, error) {
	return f(ctx, username, ip, since)
}

func TestTooManyLoginFailures(t *testing.T) {
	now := time.Unix(1_700_000_000, 0)
	tests := []struct {
		name            string
		accountFailures int64
		ipFailures      int64
		wantLimited     bool
	}{
		{name: "below thresholds", accountFailures: loginFailureAccountMax - 1, ipFailures: loginFailureAccountIPMax - 1, wantLimited: false},
		{name: "account ip threshold", accountFailures: loginFailureAccountMax - 1, ipFailures: loginFailureAccountIPMax, wantLimited: true},
		{name: "account threshold", accountFailures: loginFailureAccountMax, ipFailures: 0, wantLimited: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			counter := loginFailureCounterFunc(func(ctx context.Context, username string, ip string, since time.Time) (int64, error) {
				if username != "admin" {
					t.Fatalf("username = %q, want admin", username)
				}
				if !since.Equal(now.Add(-loginFailureWindow)) {
					t.Fatalf("since = %v, want %v", since, now.Add(-loginFailureWindow))
				}
				if ip == "" {
					return tt.accountFailures, nil
				}
				return tt.ipFailures, nil
			})

			limited, err := tooManyLoginFailures(context.Background(), counter, "admin", "127.0.0.1", now)
			if err != nil {
				t.Fatalf("tooManyLoginFailures() error = %v", err)
			}
			if limited != tt.wantLimited {
				t.Fatalf("tooManyLoginFailures() limited = %v, want %v", limited, tt.wantLimited)
			}
		})
	}
}

func TestTooManyLoginFailuresReturnsCounterError(t *testing.T) {
	wantErr := errors.New("counter failed")
	counter := loginFailureCounterFunc(func(ctx context.Context, username string, ip string, since time.Time) (int64, error) {
		return 0, wantErr
	})

	limited, err := tooManyLoginFailures(context.Background(), counter, "admin", "127.0.0.1", time.Now())
	if !errors.Is(err, wantErr) {
		t.Fatalf("tooManyLoginFailures() error = %v, want %v", err, wantErr)
	}
	if limited {
		t.Fatal("tooManyLoginFailures() limited = true, want false")
	}
}
