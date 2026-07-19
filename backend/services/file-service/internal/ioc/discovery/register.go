package discovery

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"sync"
	"time"

	clientv3 "go.etcd.io/etcd/client/v3"
)

const (
	defaultEtcdEndpoint            = "127.0.0.1:2379"
	registrationLeaseTTL     int64 = 10
	registrationTimeout            = 3 * time.Second
	reRegistrationRetryDelay       = time.Second
)

type etcdClient interface {
	Grant(context.Context, int64) (*clientv3.LeaseGrantResponse, error)
	Put(context.Context, string, string, ...clientv3.OpOption) (*clientv3.PutResponse, error)
	KeepAlive(context.Context, clientv3.LeaseID) (<-chan *clientv3.LeaseKeepAliveResponse, error)
	Revoke(context.Context, clientv3.LeaseID) (*clientv3.LeaseRevokeResponse, error)
	Close() error
}

type etcdClientFactory func(clientv3.Config) (etcdClient, error)

type Registration struct {
	endpoints []string
	key       string
	value     string

	ctx        context.Context
	cancel     context.CancelFunc
	newClient  etcdClientFactory
	retryDelay time.Duration

	mu      sync.Mutex
	client  etcdClient
	leaseID clientv3.LeaseID
	closed  bool
	once    sync.Once
}

func Register(ctx context.Context, endpoints []string, serviceName, addr string) (*Registration, error) {
	return register(ctx, endpoints, serviceName, addr, newEtcdClient)
}

func register(ctx context.Context, endpoints []string, serviceName, addr string, factory etcdClientFactory) (*Registration, error) {
	if len(endpoints) == 0 {
		endpoints = []string{defaultEtcdEndpoint}
	}
	serviceName = strings.Trim(strings.TrimSpace(serviceName), "/")
	if serviceName == "" || strings.TrimSpace(addr) == "" {
		return nil, fmt.Errorf("service name and address are required")
	}
	if ctx == nil {
		ctx = context.Background()
	}
	if factory == nil {
		factory = newEtcdClient
	}
	data, err := json.Marshal(struct {
		Name   string `json:"name"`
		Addr   string `json:"addr"`
		Weight int64  `json:"weight"`
	}{Name: serviceName, Addr: addr, Weight: 1})
	if err != nil {
		return nil, err
	}
	registrationCtx, cancel := context.WithCancel(ctx)
	r := &Registration{
		endpoints:  append([]string(nil), endpoints...),
		key:        fmt.Sprintf("/%s/%s", serviceName, addr),
		value:      string(data),
		ctx:        registrationCtx,
		cancel:     cancel,
		newClient:  factory,
		retryDelay: reRegistrationRetryDelay,
	}
	keepAlive, err := r.establishLease()
	if err != nil {
		cancel()
		return nil, err
	}
	go r.maintainLease(keepAlive)
	return r, nil
}

func newEtcdClient(config clientv3.Config) (etcdClient, error) {
	return clientv3.New(config)
}

func (r *Registration) establishLease() (<-chan *clientv3.LeaseKeepAliveResponse, error) {
	client, err := r.newClient(clientv3.Config{Endpoints: r.endpoints, DialTimeout: registrationTimeout})
	if err != nil {
		return nil, err
	}
	closeClient := true
	defer func() {
		if closeClient {
			_ = client.Close()
		}
	}()

	requestCtx, cancel := context.WithTimeout(r.ctx, registrationTimeout)
	defer cancel()
	lease, err := client.Grant(requestCtx, registrationLeaseTTL)
	if err != nil {
		return nil, err
	}
	if _, err := client.Put(requestCtx, r.key, r.value, clientv3.WithLease(lease.ID)); err != nil {
		return nil, err
	}
	keepAlive, err := client.KeepAlive(r.ctx, lease.ID)
	if err != nil {
		return nil, err
	}
	if keepAlive == nil {
		return nil, fmt.Errorf("etcd keepalive channel is nil")
	}

	r.mu.Lock()
	if r.closed {
		r.mu.Unlock()
		return nil, context.Canceled
	}
	previous := r.client
	r.client = client
	r.leaseID = lease.ID
	r.mu.Unlock()
	if previous != nil {
		_ = previous.Close()
	}
	closeClient = false
	return keepAlive, nil
}

func (r *Registration) maintainLease(keepAlive <-chan *clientv3.LeaseKeepAliveResponse) {
	for {
		select {
		case <-r.ctx.Done():
			return
		case _, ok := <-keepAlive:
			if ok {
				continue
			}
		}
		if !r.waitRetry() {
			return
		}
		for {
			var err error
			keepAlive, err = r.establishLease()
			if err == nil {
				break
			}
			if !r.waitRetry() {
				return
			}
		}
	}
}

func (r *Registration) waitRetry() bool {
	delay := r.retryDelay
	if delay <= 0 {
		delay = reRegistrationRetryDelay
	}
	timer := time.NewTimer(delay)
	defer timer.Stop()
	select {
	case <-r.ctx.Done():
		return false
	case <-timer.C:
		return true
	}
}

func (r *Registration) Close() error {
	if r == nil {
		return nil
	}
	var closeErr error
	r.once.Do(func() {
		r.cancel()
		r.mu.Lock()
		r.closed = true
		client := r.client
		leaseID := r.leaseID
		r.client = nil
		r.leaseID = 0
		r.mu.Unlock()
		if client == nil {
			return
		}
		closeCtx, cancel := context.WithTimeout(context.Background(), registrationTimeout)
		defer cancel()
		if leaseID != 0 {
			_, closeErr = client.Revoke(closeCtx, leaseID)
		}
		if err := client.Close(); closeErr == nil {
			closeErr = err
		}
	})
	return closeErr
}
