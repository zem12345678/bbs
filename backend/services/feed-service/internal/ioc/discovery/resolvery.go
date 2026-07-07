package discovery

import (
	"feed-service/pkg/logger"
	"context"
	"fmt"
	"strings"
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

	closeCh      chan struct{}
	watchCh      clientv3.WatchChan
	cli          *clientv3.Client
	keyPrefix    string
	srvAddrsList []resolver.Address

	cc     resolver.ClientConn
	logger *zap.Logger
}

// NewResolver create a new resolver .Builder base on etcd
func NewResolver(etcdAddrs []string, l logger.Logger) *Resolver {
	return &Resolver{
		schema:      schema,
		EtcdAddrs:   etcdAddrs,
		DialTimeout: 3,
		logger:      l.GetZapLogger(),
	}
}

// Scheme returns the scheme supported by this resolver
func (r *Resolver) Scheme() string {
	return r.schema
}

// Build creates a new resolver.Resolver for the given target.
func (r *Resolver) Build(target resolver.Target, cc resolver.ClientConn, opts resolver.BuildOptions) (resolver.Resolver, error) {
	// 1) 保存 ClientConn
	r.cc = cc

	// 2) 从 target.URL.Path 中取出路径，去掉开头的 "/"
	//    假设 Dial 写法为：grpc.Dial("etcd:///user/1.0.0", ...)
	//    那么 target.URL.Path 可能是 "/user/1.0.0"
	path := strings.TrimPrefix(target.URL.Path, "/")

	// 3) 尝试按照 "<serviceName>/<version>" 的格式切分
	//    parts[0] = "user", parts[1] = "1.0.0"
	parts := strings.SplitN(path, "/", 2)

	var serviceName, version string
	switch len(parts) {
	case 2:
		// /user/1.0.0
		serviceName = parts[0]
		version = parts[1]
	case 1:
		// 只有 /user，没有 /version
		serviceName = parts[0]
		version = ""
	default:
		// 空路径或其他情况，可根据需要做容错
		serviceName = ""
		version = ""
	}

	// 4) 组合出和服务端注册时一致的 key 前缀。
	//    如果 etcd 里存的是 /user/1.0.0/172.28.217.156:8881
	//    那这里就要生成 "/user/1.0.0/"
	if serviceName != "" && version != "" {
		r.keyPrefix = fmt.Sprintf("/%s/%s/", serviceName, version)
	} else if serviceName != "" {
		r.keyPrefix = fmt.Sprintf("/%s/", serviceName)
	} else {
		// 兜底：默认前缀，也可直接 return error
		r.keyPrefix = "/"
	}

	// 5) 启动与 etcd 的 watch
	if _, err := r.start(); err != nil {
		return nil, err
	}

	// 6) 返回当前 resolver
	return r, nil
}

// ResolverNow resolver .Resolver interface
func (r *Resolver) ResolveNow(o resolver.ResolveNowOptions) {

}

func (r *Resolver) Close() {
	r.closeCh <- struct{}{}
}

// start
func (r *Resolver) start() (chan<- struct{}, error) {
	var err error
	r.cli, err = clientv3.New(clientv3.Config{
		Endpoints:   r.EtcdAddrs,
		DialTimeout: time.Duration(r.DialTimeout) * time.Second,
	})
	if err != nil {
		return nil, err
	}
	resolver.Register(r)

	r.closeCh = make(chan struct{})

	if err = r.sync(); err != nil {
		return nil, err
	}

	go r.watch()

	return r.closeCh, nil
}

func (r *Resolver) watch() {
	ticker := time.NewTicker(time.Minute)
	r.watchCh = r.cli.Watch(context.Background(), r.keyPrefix, clientv3.WithPrefix())

	for {
		select {
		case <-r.closeCh:
			return
		case res, ok := <-r.watchCh:
			if ok {
				r.update(res.Events)
			}
		case <-ticker.C:
			if err := r.sync(); err != nil {
				r.logger.Error("sync failed", zap.Error(err))
			}
		}
	}
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
	ctx, cancel := context.WithTimeout(context.Background(), time.Second*3)
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
