package etcdresolver

import (
	"context"
	"net/url"
	"sync"
	"testing"
	"time"

	clientv3 "go.etcd.io/etcd/client/v3"
	"google.golang.org/grpc/resolver"
)

func TestBuildCreatesIndependentResolverState(t *testing.T) {
	clients := make([]*fakeEtcdClient, 0, 2)
	builder := NewBuilder("file-test-etcd", []string{"127.0.0.1:2379"})
	builder.newClient = func(clientv3.Config) (etcdClient, error) {
		client := newFakeEtcdClient(make(chan clientv3.WatchResponse))
		clients = append(clients, client)
		return client, nil
	}

	firstConn := &fakeClientConn{}
	first, err := builder.Build(resolver.Target{URL: url.URL{Path: "/mall"}}, firstConn, resolver.BuildOptions{})
	if err != nil {
		t.Fatalf("Build() first connection error = %v", err)
	}
	t.Cleanup(first.Close)

	secondConn := &fakeClientConn{}
	second, err := builder.Build(resolver.Target{URL: url.URL{Path: "/credit"}}, secondConn, resolver.BuildOptions{})
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
	if firstResolver.prefix != "/mall/" || secondResolver.prefix != "/credit/" {
		t.Fatalf("Build() prefixes = %q and %q", firstResolver.prefix, secondResolver.prefix)
	}
	if firstResolver.client == secondResolver.client || len(clients) != 2 {
		t.Fatal("Build() shared the etcd client between connections")
	}

	firstResolver.endpoints[0] = "mutated"
	if builder.endpoints[0] == "mutated" || secondResolver.endpoints[0] == "mutated" {
		t.Fatal("Build() shared endpoints between connections")
	}
}

func TestWatchRecreatesUnavailableChannelAfterSync(t *testing.T) {
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
			builder := NewBuilder("file-test-etcd", []string{"127.0.0.1:2379"})
			builder.syncInterval = time.Millisecond
			builder.newClient = func(clientv3.Config) (etcdClient, error) { return client, nil }

			built, err := builder.Build(resolver.Target{URL: url.URL{Path: "/mall"}}, &fakeClientConn{}, resolver.BuildOptions{})
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

type fakeClientConn struct {
	resolver.ClientConn

	mu     sync.Mutex
	states []resolver.State
	errors []error
}

func (c *fakeClientConn) UpdateState(state resolver.State) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.states = append(c.states, state)
	return nil
}

func (c *fakeClientConn) ReportError(err error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.errors = append(c.errors, err)
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
