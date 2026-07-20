package discovery

import (
	"context"
	"encoding/json"
	"net/url"
	"sync"
	"testing"
	"time"

	"content-service/pkg/logger"
	"go.etcd.io/etcd/api/v3/mvccpb"
	clientv3 "go.etcd.io/etcd/client/v3"
	"go.uber.org/zap"
	"google.golang.org/grpc/resolver"
)

func TestResolverBuildCreatesIndependentConnectionState(t *testing.T) {
	clients := make([]*fakeEtcdClient, 0, 2)
	builder := newTestResolver(func(clientv3.Config) (etcdClient, error) {
		client := newFakeEtcdClient(make(chan clientv3.WatchResponse))
		clients = append(clients, client)
		return client, nil
	})

	firstConn := &fakeClientConn{}
	first, err := builder.Build(resolver.Target{URL: url.URL{Path: "/mall/1.0.0"}}, firstConn, resolver.BuildOptions{})
	if err != nil {
		t.Fatalf("Build() first connection error = %v", err)
	}
	t.Cleanup(first.Close)

	secondConn := &fakeClientConn{}
	second, err := builder.Build(resolver.Target{URL: url.URL{Path: "/credit/1.0.0"}}, secondConn, resolver.BuildOptions{})
	if err != nil {
		t.Fatalf("Build() second connection error = %v", err)
	}
	t.Cleanup(second.Close)

	firstResolver := first.(*Resolver)
	secondResolver := second.(*Resolver)
	if firstResolver == secondResolver {
		t.Fatal("Build() reused the same resolver instance")
	}
	if firstResolver.cc != firstConn || secondResolver.cc != secondConn {
		t.Fatal("Build() did not retain each connection's ClientConn")
	}
	if firstResolver.keyPrefix != "/mall/1.0.0/" || secondResolver.keyPrefix != "/credit/1.0.0/" {
		t.Fatalf("Build() key prefixes = %q and %q", firstResolver.keyPrefix, secondResolver.keyPrefix)
	}
	if firstResolver.cli == secondResolver.cli || len(clients) != 2 {
		t.Fatal("Build() shared the etcd client between connections")
	}

	firstResolver.EtcdAddrs[0] = "mutated"
	if builder.EtcdAddrs[0] == "mutated" || secondResolver.EtcdAddrs[0] == "mutated" {
		t.Fatal("Build() shared etcd endpoint state between connections")
	}
}

func TestResolverWatchRecreatesUnavailableChannelAfterSync(t *testing.T) {
	tests := []struct {
		name       string
		firstWatch func() chan clientv3.WatchResponse
	}{
		{
			name: "closed",
			firstWatch: func() chan clientv3.WatchResponse {
				watch := make(chan clientv3.WatchResponse)
				close(watch)
				return watch
			},
		},
		{
			name: "canceled",
			firstWatch: func() chan clientv3.WatchResponse {
				watch := make(chan clientv3.WatchResponse, 1)
				watch <- clientv3.WatchResponse{Canceled: true}
				return watch
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client := newFakeEtcdClient(tt.firstWatch(), make(chan clientv3.WatchResponse))
			builder := newTestResolver(func(clientv3.Config) (etcdClient, error) {
				return client, nil
			})
			builder.syncInterval = time.Millisecond

			built, err := builder.Build(resolver.Target{URL: url.URL{Path: "/mall/1.0.0"}}, &fakeClientConn{}, resolver.BuildOptions{})
			if err != nil {
				t.Fatalf("Build() error = %v", err)
			}
			t.Cleanup(built.Close)

			deadline := time.Now().Add(time.Second)
			for client.watchCallCount() < 2 {
				if time.Now().After(deadline) {
					t.Fatalf("watch was not recreated after becoming unavailable; calls = %d", client.watchCallCount())
				}
				time.Sleep(time.Millisecond)
			}
		})
	}
}

func TestResolverUpdateRefreshesSameAddressOnReregistration(t *testing.T) {
	cc := &fakeClientConn{}
	r := newTestResolver(func(clientv3.Config) (etcdClient, error) {
		return newFakeEtcdClient(), nil
	})
	r.cc = cc
	info := Server{Name: "bbs-mall-service", Addr: "192.168.31.164:9115", Weight: 1}
	r.srvAddrsList = []resolver.Address{serverAddress(info, 1)}
	value, err := json.Marshal(info)
	if err != nil {
		t.Fatalf("marshal server: %v", err)
	}

	r.update([]*clientv3.Event{{
		Type: mvccpb.PUT,
		Kv:   &mvccpb.KeyValue{Value: value, ModRevision: 2},
	}})

	cc.mu.Lock()
	defer cc.mu.Unlock()
	if len(cc.states) != 1 {
		t.Fatalf("UpdateState calls = %d, want 1", len(cc.states))
	}
	addresses := cc.states[0].Addresses
	if len(addresses) != 1 || addresses[0].Addr != info.Addr {
		t.Fatalf("addresses = %#v, want one refreshed address", addresses)
	}
	if got := addresses[0].Attributes.Value(etcdRevisionAttributeKey); got != int64(2) {
		t.Fatalf("address revision = %#v, want 2", got)
	}
}

func newTestResolver(factory etcdClientFactory) *Resolver {
	resolver := NewResolver([]string{"127.0.0.1:2379"}, logger.NewZapLogger(zap.NewNop()))
	resolver.newClient = factory
	return resolver
}

type fakeClientConn struct {
	resolver.ClientConn

	mu     sync.Mutex
	states []resolver.State
}

func (c *fakeClientConn) UpdateState(state resolver.State) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.states = append(c.states, state)
	return nil
}

type fakeEtcdClient struct {
	mu         sync.Mutex
	watchChans []clientv3.WatchChan
	watchCalls int
}

func newFakeEtcdClient(watchChans ...chan clientv3.WatchResponse) *fakeEtcdClient {
	channels := make([]clientv3.WatchChan, 0, len(watchChans))
	for _, watchCh := range watchChans {
		channels = append(channels, watchCh)
	}
	return &fakeEtcdClient{watchChans: channels}
}

func (c *fakeEtcdClient) Get(context.Context, string, ...clientv3.OpOption) (*clientv3.GetResponse, error) {
	return &clientv3.GetResponse{}, nil
}

func (c *fakeEtcdClient) Watch(context.Context, string, ...clientv3.OpOption) clientv3.WatchChan {
	c.mu.Lock()
	defer c.mu.Unlock()
	if len(c.watchChans) == 0 {
		return nil
	}
	index := c.watchCalls
	if index >= len(c.watchChans) {
		index = len(c.watchChans) - 1
	}
	c.watchCalls++
	return c.watchChans[index]
}

func (c *fakeEtcdClient) Close() error {
	return nil
}

func (c *fakeEtcdClient) watchCallCount() int {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.watchCalls
}
