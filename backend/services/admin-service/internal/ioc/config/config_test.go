package config

import (
	"reflect"
	"testing"

	"github.com/spf13/viper"
)

func TestSetDefaultsFillsAdminAuthAndUpstreams(t *testing.T) {
	v := viper.New()
	configureEnv(v)

	setDefaults(v)

	assertString(t, v, "auth.jwtSecret", "bbs-admin-local-dev-secret")
	assertString(t, v, "auth.jwtTtl", "168h")
	assertString(t, v, "auth.defaultAdminPassword", "Admin123!")
	assertString(t, v, "auth.secretEncryptionKey", "bbs-admin-local-setting-secret")
	assertString(t, v, "upstreams.user", "bbs-user-service")
	assertString(t, v, "upstreams.reaction", "bbs-reaction-service")
	assertString(t, v, "upstreams.content", "bbs-content-service")
	assertString(t, v, "upstreams.comment", "bbs-comment-service")
}

func TestConfigureEnvBindsAdminAuthAndUpstreams(t *testing.T) {
	t.Setenv("BBS_ADMIN_AUTH_JWT_SECRET", "env-jwt-secret")
	t.Setenv("BBS_ADMIN_AUTH_JWT_TTL", "24h")
	t.Setenv("BBS_ADMIN_AUTH_DEFAULT_ADMIN_PASSWORD", "EnvAdmin123!")
	t.Setenv("BBS_ADMIN_AUTH_SECRET_ENCRYPTION_KEY", "env-setting-secret")
	t.Setenv("BBS_ADMIN_UPSTREAMS_USER", "file-user-service")
	t.Setenv("BBS_ADMIN_UPSTREAMS_REACTION", "file-reaction-service")
	t.Setenv("BBS_ADMIN_UPSTREAMS_CONTENT", "file-content-service")
	t.Setenv("BBS_ADMIN_UPSTREAMS_COMMENT", "file-comment-service")

	v := viper.New()
	configureEnv(v)
	setDefaults(v)

	assertString(t, v, "auth.jwtSecret", "env-jwt-secret")
	assertString(t, v, "auth.jwtTtl", "24h")
	assertString(t, v, "auth.defaultAdminPassword", "EnvAdmin123!")
	assertString(t, v, "auth.secretEncryptionKey", "env-setting-secret")
	assertString(t, v, "upstreams.user", "file-user-service")
	assertString(t, v, "upstreams.reaction", "file-reaction-service")
	assertString(t, v, "upstreams.content", "file-content-service")
	assertString(t, v, "upstreams.comment", "file-comment-service")
}

func TestApplyEnvOverridesSetsEtcdEndpoints(t *testing.T) {
	t.Setenv("BBS_ADMIN_GRPC_SERVER_ETCD_ADDR", "etcd-a:2379, etcd-b:2379")
	t.Setenv("BBS_ADMIN_GRPC_CLIENT_ETCD_ADDR", "etcd-client:2379")

	v := viper.New()
	applyEnvOverrides(v)

	if got, want := v.GetStringSlice("grpc.server.etcdAddr"), []string{"etcd-a:2379", "etcd-b:2379"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("server etcd endpoints = %#v, want %#v", got, want)
	}
	if got, want := v.GetStringSlice("grpc.client.etcdAddr"), []string{"etcd-client:2379"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("client etcd endpoints = %#v, want %#v", got, want)
	}
}

func assertString(t *testing.T, v *viper.Viper, key string, want string) {
	t.Helper()
	if got := v.GetString(key); got != want {
		t.Fatalf("%s = %q, want %q", key, got, want)
	}
}
