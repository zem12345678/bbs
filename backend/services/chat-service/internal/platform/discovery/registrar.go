package discovery

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	clientv3 "go.etcd.io/etcd/client/v3"
	"go.uber.org/zap"
)

const registrationTTL = int64(10)

type Registrar struct {
	client *clientv3.Client
	logger *zap.Logger
	cancel context.CancelFunc
	wait   sync.WaitGroup
}

type registration struct {
	Name   string `json:"name"`
	Addr   string `json:"addr"`
	Weight int64  `json:"weight"`
}

func New(endpoints []string, logger *zap.Logger) (*Registrar, error) {
	client, err := clientv3.New(clientv3.Config{Endpoints: endpoints, DialTimeout: 3 * time.Second})
	if err != nil {
		return nil, fmt.Errorf("create chat etcd client: %w", err)
	}
	return &Registrar{client: client, logger: logger}, nil
}

func (r *Registrar) Start(parent context.Context, serviceName, address string) error {
	ctx, cancel := context.WithCancel(parent)
	r.cancel = cancel
	ready := make(chan error, 1)
	r.wait.Add(1)
	go func() {
		defer r.wait.Done()
		r.run(ctx, serviceName, address, ready)
	}()
	return <-ready
}

func (r *Registrar) run(ctx context.Context, serviceName, address string, ready chan<- error) {
	first := true
	for {
		keepAlive, err := r.establish(ctx, serviceName, address)
		if first {
			ready <- err
			first = false
		}
		if err == nil {
			for {
				select {
				case <-ctx.Done():
					return
				case _, open := <-keepAlive:
					if !open {
						err = fmt.Errorf("chat etcd keepalive channel closed")
						goto Retry
					}
				}
			}
		}
	Retry:
		if first {
			ready <- err
			first = false
		}
		r.logger.Warn("chat service registration lost", zap.Error(err))
		select {
		case <-ctx.Done():
			return
		case <-time.After(time.Second):
		}
	}
}

func (r *Registrar) establish(ctx context.Context, serviceName, address string) (<-chan *clientv3.LeaseKeepAliveResponse, error) {
	requestCtx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()
	lease, err := r.client.Grant(requestCtx, registrationTTL)
	if err != nil {
		return nil, err
	}
	payload, err := json.Marshal(registration{Name: serviceName, Addr: address, Weight: 100})
	if err != nil {
		return nil, err
	}
	key := fmt.Sprintf("/%s/%s", serviceName, address)
	if _, err := r.client.Put(requestCtx, key, string(payload), clientv3.WithLease(lease.ID)); err != nil {
		return nil, err
	}
	return r.client.KeepAlive(ctx, lease.ID)
}

func (r *Registrar) Close() error {
	if r.cancel != nil {
		r.cancel()
	}
	r.wait.Wait()
	return r.client.Close()
}
