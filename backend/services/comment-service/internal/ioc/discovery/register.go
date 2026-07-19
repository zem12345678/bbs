package discovery

import (
	"comment-service/pkg/logger"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	clientv3 "go.etcd.io/etcd/client/v3"
)

const reRegistrationRetryDelay = time.Second

type registrationEtcdClient interface {
	Get(context.Context, string, ...clientv3.OpOption) (*clientv3.GetResponse, error)
	Put(context.Context, string, string, ...clientv3.OpOption) (*clientv3.PutResponse, error)
	Delete(context.Context, string, ...clientv3.OpOption) (*clientv3.DeleteResponse, error)
	Grant(context.Context, int64) (*clientv3.LeaseGrantResponse, error)
	KeepAlive(context.Context, clientv3.LeaseID) (<-chan *clientv3.LeaseKeepAliveResponse, error)
	Revoke(context.Context, clientv3.LeaseID) (*clientv3.LeaseRevokeResponse, error)
	Close() error
}

type registrationEtcdClientFactory func(clientv3.Config) (registrationEtcdClient, error)

type Register struct {
	EtcdAddrs   []string
	DialTimeout int

	srvInfo Server
	srvTTL  int64
	logger  logger.Logger

	ctx        context.Context
	cancel     context.CancelFunc
	newClient  registrationEtcdClientFactory
	retryDelay time.Duration

	mu       sync.Mutex
	cli      registrationEtcdClient
	leasesID clientv3.LeaseID
	closed   bool
	once     sync.Once
}

func NewRegister(etcdAddrs []string, l logger.Logger) *Register {
	return &Register{
		EtcdAddrs:   append([]string(nil), etcdAddrs...),
		DialTimeout: 3,
		logger:      l,
		newClient:   newRegistrationEtcdClient,
		retryDelay:  reRegistrationRetryDelay,
	}
}

func newRegistrationEtcdClient(config clientv3.Config) (registrationEtcdClient, error) {
	return clientv3.New(config)
}

// Register a service.
func (r *Register) Register(srvInfo Server, ttl int64) (chan<- struct{}, error) {
	if strings.Split(srvInfo.Addr, ":")[0] == "" {
		return nil, errors.New("invalid ip")
	}
	if ttl <= 0 {
		return nil, errors.New("invalid lease ttl")
	}

	r.mu.Lock()
	r.srvInfo = srvInfo
	r.srvTTL = ttl
	r.ctx, r.cancel = context.WithCancel(context.Background())
	r.mu.Unlock()

	keepAlive, err := r.establishLease()
	if err != nil {
		r.cancel()
		return nil, err
	}

	stopCh := make(chan struct{}, 1)
	go func() {
		select {
		case <-stopCh:
			_ = r.Close()
		case <-r.ctx.Done():
		}
	}()
	go r.maintainLease(keepAlive)

	return stopCh, nil
}

// Stop stops registration and releases the current lease.
func (r *Register) Stop() {
	_ = r.Close()
}

func (r *Register) establishLease() (<-chan *clientv3.LeaseKeepAliveResponse, error) {
	r.mu.Lock()
	endpoints := append([]string(nil), r.EtcdAddrs...)
	dialTimeout := time.Duration(r.DialTimeout) * time.Second
	ctx := r.ctx
	factory := r.newClient
	srvInfo := r.srvInfo
	ttl := r.srvTTL
	r.mu.Unlock()

	if dialTimeout <= 0 {
		dialTimeout = 3 * time.Second
	}
	if factory == nil {
		factory = newRegistrationEtcdClient
	}
	client, err := factory(clientv3.Config{Endpoints: endpoints, DialTimeout: dialTimeout})
	if err != nil {
		return nil, err
	}
	closeClient := true
	defer func() {
		if closeClient {
			_ = client.Close()
		}
	}()

	requestCtx, cancel := context.WithTimeout(ctx, dialTimeout)
	defer cancel()
	lease, err := client.Grant(requestCtx, ttl)
	if err != nil {
		return nil, err
	}
	data, err := json.Marshal(srvInfo)
	if err != nil {
		return nil, err
	}
	if _, err := client.Put(requestCtx, BuildRegPath(srvInfo), string(data), clientv3.WithLease(lease.ID)); err != nil {
		return nil, err
	}
	keepAlive, err := client.KeepAlive(ctx, lease.ID)
	if err != nil {
		return nil, err
	}
	if keepAlive == nil {
		return nil, errors.New("etcd keepalive channel is nil")
	}

	r.mu.Lock()
	if r.closed {
		r.mu.Unlock()
		return nil, context.Canceled
	}
	previous := r.cli
	r.cli = client
	r.leasesID = lease.ID
	r.mu.Unlock()
	if previous != nil {
		_ = previous.Close()
	}
	closeClient = false
	return keepAlive, nil
}

func (r *Register) maintainLease(keepAlive <-chan *clientv3.LeaseKeepAliveResponse) {
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
			if r.logger != nil {
				r.logger.Error("service reregistration failed", logger.Error(err))
			}
			if !r.waitRetry() {
				return
			}
		}
	}
}

func (r *Register) waitRetry() bool {
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

func (r *Register) Close() error {
	if r == nil {
		return nil
	}
	var closeErr error
	r.once.Do(func() {
		r.mu.Lock()
		r.closed = true
		if r.cancel != nil {
			r.cancel()
		}
		client := r.cli
		leaseID := r.leasesID
		srvInfo := r.srvInfo
		r.cli = nil
		r.leasesID = 0
		r.mu.Unlock()
		if client == nil {
			return
		}

		closeCtx, cancel := context.WithTimeout(context.Background(), time.Duration(r.DialTimeout)*time.Second)
		defer cancel()
		if _, err := client.Delete(closeCtx, BuildRegPath(srvInfo)); err != nil {
			closeErr = err
		}
		if leaseID != 0 {
			if _, err := client.Revoke(closeCtx, leaseID); closeErr == nil {
				closeErr = err
			}
		}
		if err := client.Close(); closeErr == nil {
			closeErr = err
		}
	})
	return closeErr
}

// UpdateHandler returns an HTTP handler for changing the service weight.
func (r *Register) UpdateHandler() http.HandlerFunc {
	return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		wi := req.URL.Query().Get("weight")
		weight, err := strconv.Atoi(wi)
		if err != nil {
			w.WriteHeader(http.StatusBadRequest)
			_, _ = w.Write([]byte(err.Error()))
			return
		}

		r.mu.Lock()
		if r.closed || r.cli == nil {
			r.mu.Unlock()
			w.WriteHeader(http.StatusServiceUnavailable)
			_, _ = w.Write([]byte("service registration unavailable"))
			return
		}
		r.srvInfo.Weight = int64(weight)
		srvInfo := r.srvInfo
		client := r.cli
		leaseID := r.leasesID
		r.mu.Unlock()

		data, err := json.Marshal(srvInfo)
		if err == nil {
			_, err = client.Put(req.Context(), BuildRegPath(srvInfo), string(data), clientv3.WithLease(leaseID))
		}
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			_, _ = w.Write([]byte(err.Error()))
			return
		}
		_, _ = w.Write([]byte("update server weight success"))
	})
}

func (r *Register) GetServerInfo() (Server, error) {
	r.mu.Lock()
	srvInfo := r.srvInfo
	client := r.cli
	r.mu.Unlock()
	if client == nil {
		return srvInfo, errors.New("service registration unavailable")
	}

	resp, err := client.Get(context.Background(), BuildRegPath(srvInfo))
	if err != nil {
		return srvInfo, err
	}
	info := Server{}
	if resp.Count >= 1 {
		if err := json.Unmarshal(resp.Kvs[0].Value, &info); err != nil {
			return info, err
		}
	}
	return info, nil
}
