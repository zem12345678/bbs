package discovery

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	clientv3 "go.etcd.io/etcd/client/v3"
)

func TestRegisterReregistersAfterKeepAliveCloses(t *testing.T) {
	first := newFakeRegisterClient(101)
	second := newFakeRegisterClient(202)
	clients := []*fakeRegisterClient{first, second}
	var (
		mu    sync.Mutex
		calls int
	)
	registration := newTestRegister(func(clientv3.Config) (registrationEtcdClient, error) {
		mu.Lock()
		defer mu.Unlock()
		client := clients[calls]
		calls++
		return client, nil
	})
	registration.retryDelay = time.Millisecond
	if _, err := registration.Register(Server{Name: "feed-service", Addr: "127.0.0.1:9111"}, 10); err != nil {
		t.Fatalf("Register() error = %v", err)
	}
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
		if clientCalls == 2 && second.putCalls() == 1 && first.closeCalls() == 1 {
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("registration did not recover after lease close: factory calls=%d second puts=%d", clientCalls, second.putCalls())
		}
		time.Sleep(time.Millisecond)
	}
}

func TestRegisterRetriesFailedReregistration(t *testing.T) {
	first := newFakeRegisterClient(101)
	second := newFakeRegisterClient(202)
	var (
		mu    sync.Mutex
		calls int
	)
	registration := newTestRegister(func(clientv3.Config) (registrationEtcdClient, error) {
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
	registration.retryDelay = time.Millisecond
	if _, err := registration.Register(Server{Name: "feed-service", Addr: "127.0.0.1:9111"}, 10); err != nil {
		t.Fatalf("Register() error = %v", err)
	}
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

func TestRegisterCloseRevokesCurrentLease(t *testing.T) {
	client := newFakeRegisterClient(101)
	registration := newTestRegister(func(clientv3.Config) (registrationEtcdClient, error) {
		return client, nil
	})
	if _, err := registration.Register(Server{Name: "feed-service", Addr: "127.0.0.1:9111"}, 10); err != nil {
		t.Fatalf("Register() error = %v", err)
	}
	if err := registration.Close(); err != nil {
		t.Fatalf("Close() error = %v", err)
	}
	if client.deleteCalls() != 1 || client.revokeCalls() != 1 || client.closeCalls() != 1 {
		t.Fatalf("close calls = delete:%d revoke:%d close:%d, want 1/1/1", client.deleteCalls(), client.revokeCalls(), client.closeCalls())
	}
}

func newTestRegister(factory registrationEtcdClientFactory) *Register {
	registration := NewRegister([]string{"127.0.0.1:2379"}, nil)
	registration.newClient = factory
	return registration
}

type fakeRegisterClient struct {
	mu        sync.Mutex
	leaseID   clientv3.LeaseID
	keepAlive chan *clientv3.LeaseKeepAliveResponse
	puts      int
	deletes   int
	revokes   int
	closes    int
}

func newFakeRegisterClient(leaseID clientv3.LeaseID) *fakeRegisterClient {
	return &fakeRegisterClient{leaseID: leaseID, keepAlive: make(chan *clientv3.LeaseKeepAliveResponse)}
}

func (c *fakeRegisterClient) Get(context.Context, string, ...clientv3.OpOption) (*clientv3.GetResponse, error) {
	return &clientv3.GetResponse{}, nil
}

func (c *fakeRegisterClient) Put(context.Context, string, string, ...clientv3.OpOption) (*clientv3.PutResponse, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.puts++
	return &clientv3.PutResponse{}, nil
}

func (c *fakeRegisterClient) Delete(context.Context, string, ...clientv3.OpOption) (*clientv3.DeleteResponse, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.deletes++
	return &clientv3.DeleteResponse{}, nil
}

func (c *fakeRegisterClient) Grant(context.Context, int64) (*clientv3.LeaseGrantResponse, error) {
	return &clientv3.LeaseGrantResponse{ID: c.leaseID}, nil
}

func (c *fakeRegisterClient) KeepAlive(context.Context, clientv3.LeaseID) (<-chan *clientv3.LeaseKeepAliveResponse, error) {
	return c.keepAlive, nil
}

func (c *fakeRegisterClient) Revoke(context.Context, clientv3.LeaseID) (*clientv3.LeaseRevokeResponse, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.revokes++
	return &clientv3.LeaseRevokeResponse{}, nil
}

func (c *fakeRegisterClient) Close() error {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.closes++
	return nil
}

func (c *fakeRegisterClient) putCalls() int {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.puts
}

func (c *fakeRegisterClient) deleteCalls() int {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.deletes
}

func (c *fakeRegisterClient) revokeCalls() int {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.revokes
}

func (c *fakeRegisterClient) closeCalls() int {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.closes
}
