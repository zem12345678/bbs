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

type Registration struct {
	client  *clientv3.Client
	leaseID clientv3.LeaseID
	once    sync.Once
}

func Register(ctx context.Context, endpoints []string, serviceName, addr string) (*Registration, error) {
	if len(endpoints) == 0 {
		endpoints = []string{"127.0.0.1:2379"}
	}
	serviceName = strings.Trim(strings.TrimSpace(serviceName), "/")
	if serviceName == "" || strings.TrimSpace(addr) == "" {
		return nil, fmt.Errorf("service name and address are required")
	}
	client, err := clientv3.New(clientv3.Config{Endpoints: endpoints, DialTimeout: 3 * time.Second})
	if err != nil {
		return nil, err
	}
	lease, err := client.Grant(ctx, 10)
	if err != nil {
		_ = client.Close()
		return nil, err
	}
	data, err := json.Marshal(struct {
		Name   string `json:"name"`
		Addr   string `json:"addr"`
		Weight int64  `json:"weight"`
	}{Name: serviceName, Addr: addr, Weight: 1})
	if err != nil {
		_ = client.Close()
		return nil, err
	}
	key := fmt.Sprintf("/%s/%s", serviceName, addr)
	if _, err := client.Put(ctx, key, string(data), clientv3.WithLease(lease.ID)); err != nil {
		_ = client.Close()
		return nil, err
	}
	keepAlive, err := client.KeepAlive(context.Background(), lease.ID)
	if err != nil {
		_ = client.Close()
		return nil, err
	}
	go func() {
		for range keepAlive {
		}
	}()
	return &Registration{client: client, leaseID: lease.ID}, nil
}

func (r *Registration) Close() error {
	if r == nil || r.client == nil {
		return nil
	}
	var closeErr error
	r.once.Do(func() {
		_, closeErr = r.client.Revoke(context.Background(), r.leaseID)
		if err := r.client.Close(); closeErr == nil {
			closeErr = err
		}
	})
	return closeErr
}
