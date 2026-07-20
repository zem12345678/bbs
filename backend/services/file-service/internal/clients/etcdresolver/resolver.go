package etcdresolver

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"sync"
	"time"

	clientv3 "go.etcd.io/etcd/client/v3"
	"google.golang.org/grpc"
	"google.golang.org/grpc/attributes"
	"google.golang.org/grpc/balancer/roundrobin"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/resolver"
)

const (
	defaultEtcdEndpoint      = "127.0.0.1:2379"
	etcdRevisionAttributeKey = "etcd_revision"
)

type etcdClient interface {
	Get(context.Context, string, ...clientv3.OpOption) (*clientv3.GetResponse, error)
	Watch(context.Context, string, ...clientv3.OpOption) clientv3.WatchChan
	Close() error
}

type etcdClientFactory func(clientv3.Config) (etcdClient, error)

type Builder struct {
	scheme       string
	endpoints    []string
	syncInterval time.Duration
	newClient    etcdClientFactory
}

type Resolver struct {
	endpoints    []string
	syncInterval time.Duration
	newClient    etcdClientFactory

	client     etcdClient
	cc         resolver.ClientConn
	prefix     string
	ctx        context.Context
	cancel     context.CancelFunc
	closeOnce  sync.Once
	watchCh    clientv3.WatchChan
	resolveNow chan struct{}
}

func NewBuilder(scheme string, endpoints []string) *Builder {
	return &Builder{
		scheme:       strings.TrimSpace(scheme),
		endpoints:    append([]string(nil), endpoints...),
		syncInterval: time.Minute,
		newClient:    newEtcdClient,
	}
}

func Dial(endpoints []string, scheme, service, upstream string) (*grpc.ClientConn, error) {
	service = strings.Trim(strings.TrimSpace(service), "/")
	if service == "" {
		return nil, fmt.Errorf("%s upstream service required", strings.TrimSpace(upstream))
	}
	if len(endpoints) == 0 {
		endpoints = []string{defaultEtcdEndpoint}
	}
	builder := NewBuilder(scheme, endpoints)
	return grpc.NewClient(
		fmt.Sprintf("%s:///%s", builder.Scheme(), service),
		grpc.WithResolvers(builder),
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithDefaultServiceConfig(fmt.Sprintf(`{"loadBalancingPolicy":%q}`, roundrobin.Name)),
	)
}

func (b *Builder) Scheme() string {
	return b.scheme
}

func (b *Builder) Build(target resolver.Target, cc resolver.ClientConn, _ resolver.BuildOptions) (resolver.Resolver, error) {
	service := strings.Trim(target.Endpoint(), "/")
	if service == "" {
		return nil, fmt.Errorf("missing etcd service name")
	}
	resolved := &Resolver{
		endpoints:    append([]string(nil), b.endpoints...),
		syncInterval: b.syncInterval,
		newClient:    b.newClient,
		cc:           cc,
		prefix:       "/" + service + "/",
		resolveNow:   make(chan struct{}, 1),
	}
	if err := resolved.start(); err != nil {
		return nil, err
	}
	return resolved, nil
}

func (r *Resolver) ResolveNow(resolver.ResolveNowOptions) {
	select {
	case r.resolveNow <- struct{}{}:
	default:
	}
}

func (r *Resolver) Close() {
	r.closeOnce.Do(func() {
		if r.cancel != nil {
			r.cancel()
		}
		if r.client != nil {
			_ = r.client.Close()
		}
	})
}

func newEtcdClient(config clientv3.Config) (etcdClient, error) {
	return clientv3.New(config)
}

func (r *Resolver) start() error {
	newClient := r.newClient
	if newClient == nil {
		newClient = newEtcdClient
	}
	client, err := newClient(clientv3.Config{Endpoints: r.endpoints, DialTimeout: 3 * time.Second})
	if err != nil {
		return err
	}
	r.client = client
	r.ctx, r.cancel = context.WithCancel(context.Background())
	if err := r.sync(); err != nil {
		r.Close()
		return err
	}
	go r.watch()
	return nil
}

func (r *Resolver) watch() {
	interval := r.syncInterval
	if interval <= 0 {
		interval = time.Minute
	}
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	r.startWatch()

	for {
		select {
		case <-r.ctx.Done():
			return
		case response, ok := <-r.watchCh:
			if !ok || response.Canceled {
				r.watchCh = nil
				continue
			}
			r.syncAndReport()
		case <-r.resolveNow:
			r.syncAndReport()
			if r.watchCh == nil {
				r.startWatch()
			}
		case <-ticker.C:
			if err := r.sync(); err != nil {
				r.cc.ReportError(err)
				continue
			}
			if r.watchCh == nil {
				r.startWatch()
			}
		}
	}
}

func (r *Resolver) startWatch() {
	if r.ctx.Err() == nil {
		r.watchCh = r.client.Watch(r.ctx, r.prefix, clientv3.WithPrefix())
	}
}

func (r *Resolver) syncAndReport() {
	if err := r.sync(); err != nil {
		r.cc.ReportError(err)
	}
}

func (r *Resolver) sync() error {
	ctx, cancel := context.WithTimeout(r.ctx, 3*time.Second)
	defer cancel()
	response, err := r.client.Get(ctx, r.prefix, clientv3.WithPrefix())
	if err != nil {
		return err
	}
	addresses := make([]resolver.Address, 0, response.Count)
	for _, item := range response.Kvs {
		var registered struct {
			Addr string `json:"addr"`
		}
		if err := json.Unmarshal(item.Value, &registered); err != nil || strings.TrimSpace(registered.Addr) == "" {
			continue
		}
		addresses = append(addresses, resolver.Address{
			Addr:       registered.Addr,
			Attributes: attributes.New(etcdRevisionAttributeKey, item.ModRevision),
		})
	}
	return r.cc.UpdateState(resolver.State{Addresses: addresses})
}
