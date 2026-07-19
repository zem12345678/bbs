package discovery

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	clientv3 "go.etcd.io/etcd/client/v3"
)

func TestRegistrationReregistersAfterKeepAliveCloses(t *testing.T) {
	first := newFakeEtcdClient(101)
	second := newFakeEtcdClient(202)
	clients := []*fakeEtcdClient{first, second}
	var (
		mu    sync.Mutex
		calls int
	)
	registration, err := register(context.Background(), []string{"127.0.0.1:2379"}, "bbs-file-service", "127.0.0.1:9111", func(clientv3.Config) (etcdClient, error) {
		mu.Lock()
		defer mu.Unlock()
		client := clients[calls]
		calls++
		return client, nil
	})
	if err != nil {
		t.Fatalf("register() error = %v", err)
	}
	registration.retryDelay = time.Millisecond
	t.Cleanup(func() { _ = registration.Close() })

	if first.putCalls() != 1 {
		t.Fatalf("initial put calls = %d, want 1", first.putCalls())
	}
	close(first.keepAlive)

	deadline := time.Now().Add(time.Second)
	for {
		mu.Lock()
		clientCalls := calls
		mu.Unlock()
		if clientCalls == 2 && second.putCalls() == 1 {
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("registration did not recover after lease close: factory calls=%d second puts=%d", clientCalls, second.putCalls())
		}
		time.Sleep(time.Millisecond)
	}
	if first.closeCalls() != 1 {
		t.Fatalf("first client closes = %d, want 1", first.closeCalls())
	}
}

func TestRegistrationCloseCancelsCurrentLease(t *testing.T) {
	client := newFakeEtcdClient(101)
	registration, err := register(context.Background(), []string{"127.0.0.1:2379"}, "bbs-file-service", "127.0.0.1:9111", func(clientv3.Config) (etcdClient, error) {
		return client, nil
	})
	if err != nil {
		t.Fatalf("register() error = %v", err)
	}
	if err := registration.Close(); err != nil {
		t.Fatalf("Close() error = %v", err)
	}
	if client.revokeCalls() != 1 || client.closeCalls() != 1 {
		t.Fatalf("close calls = revoke:%d close:%d, want 1/1", client.revokeCalls(), client.closeCalls())
	}
}

func TestRegistrationRetriesFailedReregistration(t *testing.T) {
	first := newFakeEtcdClient(101)
	second := newFakeEtcdClient(202)
	var (
		mu    sync.Mutex
		calls int
	)
	registration, err := register(context.Background(), []string{"127.0.0.1:2379"}, "bbs-file-service", "127.0.0.1:9111", func(clientv3.Config) (etcdClient, error) {
		mu.Lock()
		defer mu.Unlock()
		calls++
		switch calls {
		case 1:
			return first, nil
		case 2:
			return nil, errors.New("etcd unavailable")
		default:
			return second, nil
		}
	})
	if err != nil {
		t.Fatalf("register() error = %v", err)
	}
	registration.retryDelay = time.Millisecond
	t.Cleanup(func() { _ = registration.Close() })
	close(first.keepAlive)

	deadline := time.Now().Add(time.Second)
	for {
		mu.Lock()
		clientCalls := calls
		mu.Unlock()
		if clientCalls == 3 && second.putCalls() == 1 {
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("registration did not retry failed re-registration: factory calls=%d second puts=%d", clientCalls, second.putCalls())
		}
		time.Sleep(time.Millisecond)
	}
}

type fakeEtcdClient struct {
	mu        sync.Mutex
	leaseID   clientv3.LeaseID
	keepAlive chan *clientv3.LeaseKeepAliveResponse
	puts      int
	revokes   int
	closes    int
}

func newFakeEtcdClient(leaseID clientv3.LeaseID) *fakeEtcdClient {
	return &fakeEtcdClient{leaseID: leaseID, keepAlive: make(chan *clientv3.LeaseKeepAliveResponse)}
}

func (c *fakeEtcdClient) Grant(context.Context, int64) (*clientv3.LeaseGrantResponse, error) {
	return &clientv3.LeaseGrantResponse{ID: c.leaseID}, nil
}

func (c *fakeEtcdClient) Put(context.Context, string, string, ...clientv3.OpOption) (*clientv3.PutResponse, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.puts++
	return &clientv3.PutResponse{}, nil
}

func (c *fakeEtcdClient) KeepAlive(context.Context, clientv3.LeaseID) (<-chan *clientv3.LeaseKeepAliveResponse, error) {
	return c.keepAlive, nil
}

func (c *fakeEtcdClient) Revoke(context.Context, clientv3.LeaseID) (*clientv3.LeaseRevokeResponse, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.revokes++
	return &clientv3.LeaseRevokeResponse{}, nil
}

func (c *fakeEtcdClient) Close() error {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.closes++
	return nil
}

func (c *fakeEtcdClient) putCalls() int {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.puts
}

func (c *fakeEtcdClient) revokeCalls() int {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.revokes
}

func (c *fakeEtcdClient) closeCalls() int {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.closes
}
