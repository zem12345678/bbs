package chat

import (
	"context"
	"crypto/sha256"
	_ "embed"
	"encoding/hex"
	"errors"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
)

var (
	ErrConnectionRegistryUnavailable = errors.New("chat websocket connection registry is unavailable")
	ErrConnectionLeaseLost           = errors.New("chat websocket connection lease was lost")
)

const (
	connectionLeaseTTL              = 2 * time.Minute
	connectionRegistryOperationWait = 3 * time.Second
	connectionRegistryKeyTag        = "{chat-ws-admission}"
)

//go:embed connection_registry_acquire.lua
var connectionRegistryAcquireLua string

//go:embed connection_registry_refresh.lua
var connectionRegistryRefreshLua string

//go:embed connection_registry_check.lua
var connectionRegistryCheckLua string

//go:embed connection_registry_release.lua
var connectionRegistryReleaseLua string

// redisConnectionRegistry enforces shared connection limits across gateway
// instances. Both keys use the same Redis Cluster hash tag so the Lua scripts
// remain atomic when Redis Cluster is enabled.
type redisConnectionRegistry struct {
	cmd        redis.Cmdable
	maxUser    int
	maxIP      int
	leaseTTL   time.Duration
	now        func() time.Time
	newLeaseID func() (string, error)
}

type redisConnectionLease struct {
	registry  *redisConnectionRegistry
	id        string
	userKey   string
	ipKey     string
	trackUser bool
	trackIP   bool
}

func newRedisConnectionRegistry(cmd redis.Cmdable, maxUser, maxIP int) *redisConnectionRegistry {
	if maxUser <= 0 && maxIP <= 0 {
		return nil
	}
	return &redisConnectionRegistry{
		cmd:      cmd,
		maxUser:  maxUser,
		maxIP:    maxIP,
		leaseTTL: connectionLeaseTTL,
		now:      time.Now,
		newLeaseID: func() (string, error) {
			value, err := uuid.NewRandom()
			return value.String(), err
		},
	}
}

func (r *redisConnectionRegistry) acquire(ctx context.Context, userID int64, clientIP string) (*redisConnectionLease, error) {
	if r == nil || r.cmd == nil || userID <= 0 {
		return nil, ErrConnectionRegistryUnavailable
	}
	leaseID, err := r.newLeaseID()
	if err != nil {
		return nil, err
	}
	clientIP = strings.TrimSpace(clientIP)
	trackUser := r.maxUser > 0
	trackIP := r.maxIP > 0 && clientIP != ""
	if !trackUser && !trackIP {
		return nil, nil
	}
	now := r.now().UTC()
	expiresAt := now.Add(r.leaseTTL)
	userKey := connectionUserLeaseKey(userID)
	ipKey := connectionIPLeaseKey(clientIP)
	result, err := r.cmd.Eval(
		ctx,
		connectionRegistryAcquireLua,
		[]string{userKey, ipKey},
		now.UnixMilli(),
		expiresAt.UnixMilli(),
		r.leaseTTL.Milliseconds(),
		leaseID,
		r.maxUser,
		r.maxIP,
		boolToLuaNumber(trackUser),
		boolToLuaNumber(trackIP),
	).Int()
	if err != nil {
		return nil, ErrConnectionRegistryUnavailable
	}
	switch result {
	case 0:
		return &redisConnectionLease{
			registry: r, id: leaseID, userKey: userKey, ipKey: ipKey,
			trackUser: trackUser, trackIP: trackIP,
		}, nil
	case 1:
		return nil, ErrUserConnectionLimit
	case 2:
		return nil, ErrIPConnectionLimit
	default:
		return nil, ErrConnectionRegistryUnavailable
	}
}

func (r *redisConnectionRegistry) canAcquire(ctx context.Context, userID int64, clientIP string) error {
	if r == nil || r.cmd == nil || userID <= 0 {
		return ErrConnectionRegistryUnavailable
	}
	clientIP = strings.TrimSpace(clientIP)
	trackUser := r.maxUser > 0
	trackIP := r.maxIP > 0 && clientIP != ""
	if !trackUser && !trackIP {
		return nil
	}
	result, err := r.cmd.Eval(
		ctx,
		connectionRegistryCheckLua,
		[]string{connectionUserLeaseKey(userID), connectionIPLeaseKey(clientIP)},
		r.now().UTC().UnixMilli(),
		r.maxUser,
		r.maxIP,
		boolToLuaNumber(trackUser),
		boolToLuaNumber(trackIP),
	).Int()
	if err != nil {
		return ErrConnectionRegistryUnavailable
	}
	switch result {
	case 0:
		return nil
	case 1:
		return ErrUserConnectionLimit
	case 2:
		return ErrIPConnectionLimit
	default:
		return ErrConnectionRegistryUnavailable
	}
}

func (l *redisConnectionLease) Refresh(ctx context.Context) error {
	if l == nil || l.registry == nil || l.registry.cmd == nil {
		return ErrConnectionLeaseLost
	}
	now := l.registry.now().UTC()
	result, err := l.registry.cmd.Eval(
		ctx,
		connectionRegistryRefreshLua,
		[]string{l.userKey, l.ipKey},
		now.Add(l.registry.leaseTTL).UnixMilli(),
		l.registry.leaseTTL.Milliseconds(),
		l.id,
		boolToLuaNumber(l.trackUser),
		boolToLuaNumber(l.trackIP),
	).Int()
	if err != nil {
		return ErrConnectionRegistryUnavailable
	}
	if result != 0 {
		return ErrConnectionLeaseLost
	}
	return nil
}

func (l *redisConnectionLease) Release(ctx context.Context) error {
	if l == nil || l.registry == nil || l.registry.cmd == nil {
		return nil
	}
	_, err := l.registry.cmd.Eval(
		ctx,
		connectionRegistryReleaseLua,
		[]string{l.userKey, l.ipKey},
		l.id,
		boolToLuaNumber(l.trackUser),
		boolToLuaNumber(l.trackIP),
	).Result()
	if err != nil {
		return ErrConnectionRegistryUnavailable
	}
	return nil
}

func connectionUserLeaseKey(userID int64) string {
	return "chat:ws:connections:" + connectionRegistryKeyTag + ":user:" + strconv.FormatInt(userID, 10)
}

func connectionIPLeaseKey(clientIP string) string {
	if clientIP == "" {
		return "chat:ws:connections:" + connectionRegistryKeyTag + ":ip:none"
	}
	digest := sha256.Sum256([]byte(clientIP))
	return "chat:ws:connections:" + connectionRegistryKeyTag + ":ip:" + hex.EncodeToString(digest[:])
}

func boolToLuaNumber(value bool) int {
	if value {
		return 1
	}
	return 0
}
