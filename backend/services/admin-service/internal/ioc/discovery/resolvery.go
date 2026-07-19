package discovery

import (
	"admin/pkg/logger"
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	"go.etcd.io/etcd/api/v3/mvccpb"
	clientv3 "go.etcd.io/etcd/client/v3"
	"go.uber.org/zap"
	"google.golang.org/grpc/resolver"
)

const (
	schema = "etcd"
)

// Resolver for grpc client
type Resolver struct {
	schema      string
	EtcdAddrs   []string
	DialTimeout int

	ctx          context.Context
	cancel       context.CancelFunc
	closeOnce    sync.Once
	watchCh      clientv3.WatchChan
	cli          etcdClient
	keyPrefix    string
	srvAddrsList []resolver.Address
	syncInterval time.Duration
	newClient    etcdClientFactory

	cc     resolver.ClientConn
	logger *zap.Logger
}

type etcdClient interface {
	Get(context.Context, string, ...clientv3.OpOption) (*clientv3.GetResponse, error)
	Watch(context.Context, string, ...clientv3.OpOption) clientv3.WatchChan
	Close() error
}

type etcdClientFactory func(clientv3.Config) (etcdClient, error)

// NewResolver create a new resolver .Builder base on etcd
func NewResolver(etcdAddrs []string, l logger.Logger) *Resolver {
	return &Resolver{
		schema:       schema,
		EtcdAddrs:    append([]string(nil), etcdAddrs...),
		DialTimeout:  3,
		logger:       l.GetZapLogger(),
		syncInterval: time.Minute,
		newClient:    newEtcdClient,
	}
}

// Scheme returns the scheme supported by this resolver
func (r *Resolver) Scheme() string {
	return r.schema
}

// Build creates a new resolver.Resolver for the given target.
func (r *Resolver) Build(target resolver.Target, cc resolver.ClientConn, opts resolver.BuildOptions) (resolver.Resolver, error) {
	resolved := r.newConnectionResolver(cc)
	resolved.keyPrefix = keyPrefix(target)

	if err := resolved.start(); err != nil {
		return nil, err
	}

	return resolved, nil
}

func (r *Resolver) newConnectionResolver(cc resolver.ClientConn) *Resolver {
	return &Resolver{
		schema:       r.schema,
		EtcdAddrs:    append([]string(nil), r.EtcdAddrs...),
		DialTimeout:  r.DialTimeout,
		syncInterval: r.syncInterval,
		newClient:    r.newClient,
		cc:           cc,
		logger:       r.logger,
	}
}

func keyPrefix(target resolver.Target) string {
	path := strings.TrimPrefix(target.URL.Path, "/")
	parts := strings.SplitN(path, "/", 2)

	var serviceName, version string
	switch len(parts) {
	case 2:
		serviceName = parts[0]
		version = parts[1]
	case 1:
		serviceName = parts[0]
		version = ""
	default:
		serviceName = ""
		version = ""
	}

	if serviceName != "" && version != "" {
		return fmt.Sprintf("/%s/%s/", serviceName, version)
	} else if serviceName != "" {
		return fmt.Sprintf("/%s/", serviceName)
	}
	return "/"
}

// ResolverNow resolver .Resolver interface
func (r *Resolver) ResolveNow(o resolver.ResolveNowOptions) {

}

func (r *Resolver) Close() {
	r.closeOnce.Do(func() {
		if r.cancel != nil {
			r.cancel()
		}
		if r.cli != nil {
			_ = r.cli.Close()
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

	cli, err := newClient(clientv3.Config{
		Endpoints:   r.EtcdAddrs,
		DialTimeout: time.Duration(r.DialTimeout) * time.Second,
	})
	if err != nil {
		return err
	}
	r.cli = cli
	r.ctx, r.cancel = context.WithCancel(context.Background())

	if err = r.sync(); err != nil {
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
		case res, ok := <-r.watchCh:
			if !ok || res.Canceled {
				r.watchCh = nil
				continue
			}
			r.update(res.Events)
		case <-ticker.C:
			if err := r.sync(); err != nil {
				if r.logger != nil {
					r.logger.Error("sync failed", zap.Error(err))
				}
				continue
			}
			if r.watchCh == nil {
				r.startWatch()
			}
		}
	}
}

func (r *Resolver) startWatch() {
	r.watchCh = r.cli.Watch(r.ctx, r.keyPrefix, clientv3.WithPrefix())
}

// update
func (r *Resolver) update(events []*clientv3.Event) {
	for _, ev := range events {
		var info Server
		var err error
		switch ev.Type {
		case mvccpb.PUT:
			info, err = ParseValue(ev.Kv.Value)
			if err != nil {
				continue
			}
			addr := resolver.Address{
				Addr:     info.Addr,
				Metadata: info.Weight,
			}
			if !Exist(r.srvAddrsList, addr) {
				r.srvAddrsList = append(r.srvAddrsList, addr)
				r.cc.UpdateState(resolver.State{Addresses: r.srvAddrsList})
			}
		case mvccpb.DELETE:
			info, err = SplitPath(string(ev.Kv.Key))
			if err != nil {
				continue
			}
			addr := resolver.Address{Addr: info.Addr}
			if s, ok := Remove(r.srvAddrsList, addr); ok {
				r.srvAddrsList = s
				r.cc.UpdateState(resolver.State{Addresses: r.srvAddrsList})
			}
		}
	}
}

// sync 同步获取所有地址信息
func (r *Resolver) sync() error {
	ctx, cancel := context.WithTimeout(r.ctx, time.Second*3)
	defer cancel()
	resp, err := r.cli.Get(ctx, r.keyPrefix, clientv3.WithPrefix())
	if err != nil {
		return err
	}
	r.srvAddrsList = []resolver.Address{}

	for _, v := range resp.Kvs {
		info, err := ParseValue(v.Value)
		if err != nil {
			continue
		}
		addr := resolver.Address{Addr: info.Addr, Metadata: info.Weight}
		r.srvAddrsList = append(r.srvAddrsList, addr)
	}
	r.cc.UpdateState(resolver.State{Addresses: r.srvAddrsList})
	return nil
}
