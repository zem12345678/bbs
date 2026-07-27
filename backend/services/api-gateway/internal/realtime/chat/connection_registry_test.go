package chat

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
)

func TestRedisConnectionRegistrySharesUserAndIPLimits(t *testing.T) {
	server := miniredis.RunT(t)
	clientA := redis.NewClient(&redis.Options{Addr: server.Addr()})
	clientB := redis.NewClient(&redis.Options{Addr: server.Addr()})
	t.Cleanup(func() {
		_ = clientA.Close()
		_ = clientB.Close()
		server.Close()
	})

	registryA := newRedisConnectionRegistry(clientA, 1, 1)
	registryB := newRedisConnectionRegistry(clientB, 1, 1)
	lease, err := registryA.acquire(context.Background(), 7, "198.51.100.20")
	if err != nil {
		t.Fatal(err)
	}
	if err := registryB.canAcquire(context.Background(), 7, "198.51.100.21"); !errors.Is(err, ErrUserConnectionLimit) {
		t.Fatalf("same-user capacity check error = %v, want %v", err, ErrUserConnectionLimit)
	}
	if err := registryB.canAcquire(context.Background(), 8, "198.51.100.20"); !errors.Is(err, ErrIPConnectionLimit) {
		t.Fatalf("same-IP capacity check error = %v, want %v", err, ErrIPConnectionLimit)
	}
	if _, err := registryB.acquire(context.Background(), 7, "198.51.100.21"); !errors.Is(err, ErrUserConnectionLimit) {
		t.Fatalf("same-user lease error = %v, want %v", err, ErrUserConnectionLimit)
	}
	if _, err := registryB.acquire(context.Background(), 8, "198.51.100.20"); !errors.Is(err, ErrIPConnectionLimit) {
		t.Fatalf("same-IP lease error = %v, want %v", err, ErrIPConnectionLimit)
	}
	if err := lease.Release(context.Background()); err != nil {
		t.Fatal(err)
	}
	secondLease, err := registryB.acquire(context.Background(), 8, "198.51.100.20")
	if err != nil {
		t.Fatalf("released lease should free shared capacity: %v", err)
	}
	if err := secondLease.Release(context.Background()); err != nil {
		t.Fatal(err)
	}
}

func TestRedisConnectionLeaseRefreshExtendsItsExpiry(t *testing.T) {
	server := miniredis.RunT(t)
	client := redis.NewClient(&redis.Options{Addr: server.Addr()})
	t.Cleanup(func() {
		_ = client.Close()
		server.Close()
	})

	now := time.Unix(1_700_000_000, 0).UTC()
	registry := newRedisConnectionRegistry(client, 1, 1)
	registry.now = func() time.Time { return now }
	lease, err := registry.acquire(context.Background(), 7, "198.51.100.20")
	if err != nil {
		t.Fatal(err)
	}
	firstExpiry, err := client.ZScore(context.Background(), lease.userKey, lease.id).Result()
	if err != nil {
		t.Fatal(err)
	}
	now = now.Add(time.Minute)
	if err := lease.Refresh(context.Background()); err != nil {
		t.Fatal(err)
	}
	refreshedExpiry, err := client.ZScore(context.Background(), lease.userKey, lease.id).Result()
	if err != nil {
		t.Fatal(err)
	}
	if refreshedExpiry <= firstExpiry {
		t.Fatalf("lease expiry = %f, want greater than %f", refreshedExpiry, firstExpiry)
	}
	if err := lease.Release(context.Background()); err != nil {
		t.Fatal(err)
	}
	if err := lease.Refresh(context.Background()); !errors.Is(err, ErrConnectionLeaseLost) {
		t.Fatalf("released lease refresh error = %v, want %v", err, ErrConnectionLeaseLost)
	}
	if count, err := client.ZCard(context.Background(), lease.userKey).Result(); err != nil || count != 0 {
		t.Fatalf("remaining user leases = %d, err = %v", count, err)
	}
}
