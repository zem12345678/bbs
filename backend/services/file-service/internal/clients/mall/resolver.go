package mall

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"sync"
	"time"

	clientv3 "go.etcd.io/etcd/client/v3"
	"google.golang.org/grpc"
	"google.golang.org/grpc/balancer/roundrobin"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/resolver"
)

const etcdResolverScheme = "file-mall-etcd"

type etcdResolver struct {
	endpoints []string

	client *clientv3.Client
	cc     resolver.ClientConn
	prefix string
	cancel context.CancelFunc
	once   sync.Once
}

func dialEtcd(endpoints []string, service string) (*grpc.ClientConn, error) {
	if len(endpoints) == 0 {
		endpoints = []string{"127.0.0.1:2379"}
	}
	service = strings.Trim(strings.TrimSpace(service), "/")
	if service == "" {
		return nil, fmt.Errorf("mall upstream service required")
	}
	builder := &etcdResolver{endpoints: endpoints}
	resolver.Register(builder)
	return grpc.NewClient(
		fmt.Sprintf("%s:///%s", builder.Scheme(), service),
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithDefaultServiceConfig(fmt.Sprintf(`{"loadBalancingPolicy":%q}`, roundrobin.Name)),
	)
}

func (r *etcdResolver) Scheme() string {
	return etcdResolverScheme
}

func (r *etcdResolver) Build(target resolver.Target, cc resolver.ClientConn, _ resolver.BuildOptions) (resolver.Resolver, error) {
	client, err := clientv3.New(clientv3.Config{Endpoints: r.endpoints, DialTimeout: 3 * time.Second})
	if err != nil {
		return nil, err
	}
	r.client = client
	r.cc = cc
	service := strings.Trim(strings.TrimSpace(target.URL.Path), "/")
	if service == "" {
		_ = client.Close()
		return nil, fmt.Errorf("missing etcd service name")
	}
	r.prefix = "/" + service + "/"
	if err := r.sync(context.Background()); err != nil {
		_ = client.Close()
		return nil, err
	}
	ctx, cancel := context.WithCancel(context.Background())
	r.cancel = cancel
	go r.watch(ctx)
	return r, nil
}

func (r *etcdResolver) ResolveNow(resolver.ResolveNowOptions) {
	if r.client != nil {
		_ = r.sync(context.Background())
	}
}

func (r *etcdResolver) Close() {
	r.once.Do(func() {
		if r.cancel != nil {
			r.cancel()
		}
		if r.client != nil {
			_ = r.client.Close()
		}
	})
}

func (r *etcdResolver) watch(ctx context.Context) {
	for range r.client.Watch(ctx, r.prefix, clientv3.WithPrefix()) {
		if err := r.sync(ctx); err != nil && ctx.Err() != nil {
			return
		}
	}
}

func (r *etcdResolver) sync(ctx context.Context) error {
	queryCtx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()
	response, err := r.client.Get(queryCtx, r.prefix, clientv3.WithPrefix())
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
		addresses = append(addresses, resolver.Address{Addr: registered.Addr})
	}
	r.cc.UpdateState(resolver.State{Addresses: addresses})
	return nil
}
