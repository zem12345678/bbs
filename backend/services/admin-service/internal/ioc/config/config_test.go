package config

import (
	"reflect"
	"strings"
	"testing"

	"github.com/spf13/viper"
)

func TestSetDefaultsFillsAdminAuthAndUpstreams(t *testing.T) {
	v := viper.New()
	configureEnv(v)

	setDefaults(v)

	assertString(t, v, "auth.jwtSecret", "bbs-admin-local-dev-secret")
	assertString(t, v, "auth.jwtTtl", "168h")
	assertString(t, v, "auth.refreshTtl", "720h")
	assertString(t, v, "auth.defaultAdminPassword", "Admin123!")
	assertString(t, v, "auth.secretEncryptionKey", "bbs-admin-local-setting-secret")
	assertString(t, v, "grpc.server.internalAuthToken", "bbs-local-admin-internal-token")
	assertString(t, v, "upstreams.user", "bbs-user-service")
	assertString(t, v, "upstreams.userInternalAuthToken", "bbs-local-user-internal-token")
	assertString(t, v, "upstreams.reaction", "bbs-reaction-service")
	assertString(t, v, "upstreams.reactionInternalAuthToken", "bbs-local-reaction-internal-token")
	assertString(t, v, "upstreams.content", "bbs-content-service")
	assertString(t, v, "upstreams.contentInternalAuthToken", "bbs-local-content-internal-token")
	assertString(t, v, "upstreams.comment", "bbs-comment-service")
	assertString(t, v, "upstreams.commentInternalAuthToken", "bbs-local-comment-internal-token")
	assertString(t, v, "upstreams.notification", "bbs-notification-service")
	assertString(t, v, "upstreams.notificationInternalAuthToken", "bbs-local-notification-internal-token")
	assertString(t, v, "upstreams.search", "bbs-search-service")
	assertString(t, v, "upstreams.searchInternalAuthToken", "bbs-local-search-internal-token")
}

func TestConfigureEnvBindsAdminAuthAndUpstreams(t *testing.T) {
	t.Setenv("BBS_ADMIN_AUTH_JWT_SECRET", "env-jwt-secret")
	t.Setenv("BBS_ADMIN_AUTH_JWT_TTL", "24h")
	t.Setenv("BBS_ADMIN_AUTH_REFRESH_TTL", "720h")
	t.Setenv("BBS_ADMIN_AUTH_DEFAULT_ADMIN_PASSWORD", "EnvAdmin123!")
	t.Setenv("BBS_ADMIN_AUTH_SECRET_ENCRYPTION_KEY", "env-setting-secret")
	t.Setenv("BBS_ADMIN_GRPC_SERVER_INTERNAL_AUTH_TOKEN", "env-admin-internal-token")
	t.Setenv("BBS_ADMIN_TRACE_ENV", "production")
	t.Setenv("BBS_ADMIN_UPSTREAMS_USER", "file-user-service")
	t.Setenv("BBS_ADMIN_UPSTREAMS_USER_INTERNAL_AUTH_TOKEN", "env-user-internal-token")
	t.Setenv("BBS_ADMIN_UPSTREAMS_REACTION", "file-reaction-service")
	t.Setenv("BBS_ADMIN_UPSTREAMS_REACTION_INTERNAL_AUTH_TOKEN", "env-reaction-internal-token")
	t.Setenv("BBS_ADMIN_UPSTREAMS_CONTENT", "file-content-service")
	t.Setenv("BBS_ADMIN_UPSTREAMS_CONTENT_INTERNAL_AUTH_TOKEN", "env-content-internal-token")
	t.Setenv("BBS_ADMIN_UPSTREAMS_COMMENT", "file-comment-service")
	t.Setenv("BBS_ADMIN_UPSTREAMS_COMMENT_INTERNAL_AUTH_TOKEN", "env-comment-internal-token")
	t.Setenv("BBS_ADMIN_UPSTREAMS_NOTIFICATION", "file-notification-service")
	t.Setenv("BBS_ADMIN_UPSTREAMS_NOTIFICATION_INTERNAL_AUTH_TOKEN", "env-notification-internal-token")
	t.Setenv("BBS_ADMIN_UPSTREAMS_SEARCH", "file-search-service")
	t.Setenv("BBS_ADMIN_UPSTREAMS_SEARCH_INTERNAL_AUTH_TOKEN", "env-search-internal-token")

	v := viper.New()
	configureEnv(v)
	setDefaults(v)

	assertString(t, v, "auth.jwtSecret", "env-jwt-secret")
	assertString(t, v, "auth.jwtTtl", "24h")
	assertString(t, v, "auth.refreshTtl", "720h")
	assertString(t, v, "auth.defaultAdminPassword", "EnvAdmin123!")
	assertString(t, v, "auth.secretEncryptionKey", "env-setting-secret")
	assertString(t, v, "grpc.server.internalAuthToken", "env-admin-internal-token")
	assertString(t, v, "trace.env", "production")
	assertString(t, v, "upstreams.user", "file-user-service")
	assertString(t, v, "upstreams.userInternalAuthToken", "env-user-internal-token")
	assertString(t, v, "upstreams.reaction", "file-reaction-service")
	assertString(t, v, "upstreams.reactionInternalAuthToken", "env-reaction-internal-token")
	assertString(t, v, "upstreams.content", "file-content-service")
	assertString(t, v, "upstreams.contentInternalAuthToken", "env-content-internal-token")
	assertString(t, v, "upstreams.comment", "file-comment-service")
	assertString(t, v, "upstreams.commentInternalAuthToken", "env-comment-internal-token")
	assertString(t, v, "upstreams.notification", "file-notification-service")
	assertString(t, v, "upstreams.notificationInternalAuthToken", "env-notification-internal-token")
	assertString(t, v, "upstreams.search", "file-search-service")
	assertString(t, v, "upstreams.searchInternalAuthToken", "env-search-internal-token")
}

func TestConfigureEnvBindsPostgresSettings(t *testing.T) {
	t.Setenv("BBS_ADMIN_POSTGRES_DSN", "postgres://user:password@127.0.0.1:25432/bbs?sslmode=disable&search_path=bbs_admin")
	t.Setenv("BBS_ADMIN_POSTGRES_DEBUG", "true")
	v := viper.New()
	configureEnv(v)

	assertString(t, v, "postgres.dsn", "postgres://user:password@127.0.0.1:25432/bbs?sslmode=disable&search_path=bbs_admin")
	if !v.GetBool("postgres.debug") {
		t.Fatal("postgres.debug should be true")
	}
}

func TestApplyEnvOverridesSetsPostgresSettings(t *testing.T) {
	t.Setenv("BBS_ADMIN_POSTGRES_DSN", "postgres://user:password@127.0.0.1:25432/bbs?sslmode=disable&search_path=bbs_admin")
	t.Setenv("BBS_ADMIN_POSTGRES_DEBUG", "true")
	v := viper.New()
	applyEnvOverrides(v)

	assertString(t, v, "postgres.dsn", "postgres://user:password@127.0.0.1:25432/bbs?sslmode=disable&search_path=bbs_admin")
	if !v.GetBool("postgres.debug") {
		t.Fatal("postgres.debug should be true")
	}
}

func TestSkipNacosRequiresAnExplicitTruthyEnvironmentValue(t *testing.T) {
	t.Setenv("BBS_ADMIN_SKIP_NACOS", "true")
	if !skipNacos() {
		t.Fatal("expected explicit true to skip Nacos")
	}
	t.Setenv("BBS_ADMIN_SKIP_NACOS", "false")
	if skipNacos() {
		t.Fatal("expected false not to skip Nacos")
	}
}

func TestValidateProductionSecurityConfigRejectsUnsafeValues(t *testing.T) {
	tests := []struct {
		name      string
		configure func(*viper.Viper)
		want      string
	}{
		{
			name: "local defaults",
			configure: func(v *viper.Viper) {
				setDefaults(v)
			},
			want: "auth.jwtSecret",
		},
		{
			name: "short jwt secret",
			configure: func(v *viper.Viper) {
				setSecureProductionAuth(v)
				v.Set("auth.jwtSecret", "too-short")
			},
			want: "auth.jwtSecret",
		},
		{
			name: "missing encryption key",
			configure: func(v *viper.Viper) {
				setSecureProductionAuth(v)
				v.Set("auth.secretEncryptionKey", " ")
			},
			want: "auth.secretEncryptionKey",
		},
		{
			name: "shared secrets",
			configure: func(v *viper.Viper) {
				setSecureProductionAuth(v)
				v.Set("auth.secretEncryptionKey", v.GetString("auth.jwtSecret"))
			},
			want: "must differ",
		},
		{
			name: "default bootstrap password",
			configure: func(v *viper.Viper) {
				setSecureProductionAuth(v)
				v.Set("auth.defaultAdminPassword", localDevDefaultAdminPassword)
			},
			want: "auth.defaultAdminPassword",
		},
		{
			name: "weak bootstrap password",
			configure: func(v *viper.Viper) {
				setSecureProductionAuth(v)
				v.Set("auth.defaultAdminPassword", "weak")
			},
			want: "auth.defaultAdminPassword",
		},
		{
			name: "default grpc internal auth token",
			configure: func(v *viper.Viper) {
				setSecureProductionAuth(v)
				v.Set("grpc.server.internalAuthToken", localDevInternalAuthToken)
			},
			want: "grpc.server.internalAuthToken",
		},
		{
			name: "short upstream user internal auth token",
			configure: func(v *viper.Viper) {
				setSecureProductionAuth(v)
				v.Set("upstreams.userInternalAuthToken", "too-short")
			},
			want: "upstreams.userInternalAuthToken",
		},
		{
			name: "default upstream reaction internal auth token",
			configure: func(v *viper.Viper) {
				setSecureProductionAuth(v)
				v.Set("upstreams.reactionInternalAuthToken", localDevReactionInternalAuthToken)
			},
			want: "upstreams.reactionInternalAuthToken",
		},
		{
			name: "missing upstream reaction internal auth token",
			configure: func(v *viper.Viper) {
				setSecureProductionAuth(v)
				v.Set("upstreams.reactionInternalAuthToken", " ")
			},
			want: "upstreams.reactionInternalAuthToken",
		},
		{
			name: "short upstream reaction internal auth token",
			configure: func(v *viper.Viper) {
				setSecureProductionAuth(v)
				v.Set("upstreams.reactionInternalAuthToken", "too-short")
			},
			want: "upstreams.reactionInternalAuthToken",
		},
		{
			name: "default upstream notification internal auth token",
			configure: func(v *viper.Viper) {
				setSecureProductionAuth(v)
				v.Set("upstreams.notificationInternalAuthToken", localDevNotificationInternalAuthToken)
			},
			want: "upstreams.notificationInternalAuthToken",
		},
		{
			name: "short upstream notification internal auth token",
			configure: func(v *viper.Viper) {
				setSecureProductionAuth(v)
				v.Set("upstreams.notificationInternalAuthToken", "too-short")
			},
			want: "upstreams.notificationInternalAuthToken",
		},
		{
			name: "default upstream search internal auth token",
			configure: func(v *viper.Viper) {
				setSecureProductionAuth(v)
				v.Set("upstreams.searchInternalAuthToken", localDevSearchInternalAuthToken)
			},
			want: "upstreams.searchInternalAuthToken",
		},
		{
			name: "missing upstream search internal auth token",
			configure: func(v *viper.Viper) {
				setSecureProductionAuth(v)
				v.Set("upstreams.searchInternalAuthToken", " ")
			},
			want: "upstreams.searchInternalAuthToken",
		},
		{
			name: "short upstream search internal auth token",
			configure: func(v *viper.Viper) {
				setSecureProductionAuth(v)
				v.Set("upstreams.searchInternalAuthToken", "too-short")
			},
			want: "upstreams.searchInternalAuthToken",
		},
		{
			name: "default upstream comment internal auth token",
			configure: func(v *viper.Viper) {
				setSecureProductionAuth(v)
				v.Set("upstreams.commentInternalAuthToken", localDevCommentInternalAuthToken)
			},
			want: "upstreams.commentInternalAuthToken",
		},
		{
			name: "missing upstream comment internal auth token",
			configure: func(v *viper.Viper) {
				setSecureProductionAuth(v)
				v.Set("upstreams.commentInternalAuthToken", " ")
			},
			want: "upstreams.commentInternalAuthToken",
		},
		{
			name: "short upstream comment internal auth token",
			configure: func(v *viper.Viper) {
				setSecureProductionAuth(v)
				v.Set("upstreams.commentInternalAuthToken", "too-short")
			},
			want: "upstreams.commentInternalAuthToken",
		},
		{
			name: "default upstream content internal auth token",
			configure: func(v *viper.Viper) {
				setSecureProductionAuth(v)
				v.Set("upstreams.contentInternalAuthToken", localDevContentInternalAuthToken)
			},
			want: "upstreams.contentInternalAuthToken",
		},
		{
			name: "missing upstream content internal auth token",
			configure: func(v *viper.Viper) {
				setSecureProductionAuth(v)
				v.Set("upstreams.contentInternalAuthToken", " ")
			},
			want: "upstreams.contentInternalAuthToken",
		},
		{
			name: "short upstream content internal auth token",
			configure: func(v *viper.Viper) {
				setSecureProductionAuth(v)
				v.Set("upstreams.contentInternalAuthToken", "too-short")
			},
			want: "upstreams.contentInternalAuthToken",
		},
		{
			name: "plaintext grpc server",
			configure: func(v *viper.Viper) {
				setSecureProductionAuth(v)
				v.Set("grpc.server.tls.enabled", false)
			},
			want: "tls.enabled",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			v := viper.New()
			v.Set("trace.env", "production")
			tt.configure(v)
			err := validateProductionSecurityConfig(v)
			if err == nil || !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("validateProductionSecurityConfig() error = %v, want %q", err, tt.want)
			}
		})
	}
}

func TestValidateProductionSecurityConfigAllowsSecureValuesAndLocalDefaults(t *testing.T) {
	production := viper.New()
	production.Set("trace.env", "prod")
	setSecureProductionAuth(production)
	if err := validateProductionSecurityConfig(production); err != nil {
		t.Fatalf("validateProductionSecurityConfig() secure production error = %v", err)
	}

	local := viper.New()
	local.Set("trace.env", "local")
	setDefaults(local)
	if err := validateProductionSecurityConfig(local); err != nil {
		t.Fatalf("validateProductionSecurityConfig() local defaults error = %v", err)
	}
}

func setSecureProductionAuth(v *viper.Viper) {
	v.Set("auth.jwtSecret", "0123456789abcdef0123456789abcdef")
	v.Set("auth.secretEncryptionKey", "abcdef0123456789abcdef0123456789")
	v.Set("auth.defaultAdminPassword", "Bootstrap-Admin-2026!")
	v.Set("grpc.server.internalAuthToken", "admin-internal-token-with-at-least-32-bytes")
	v.Set("upstreams.userInternalAuthToken", "user-internal-token-with-at-least-32-bytes")
	v.Set("upstreams.reactionInternalAuthToken", "reaction-internal-token-with-at-least-32-bytes")
	v.Set("upstreams.notificationInternalAuthToken", "notification-internal-token-with-at-least-32-bytes")
	v.Set("upstreams.searchInternalAuthToken", "search-internal-token-with-at-least-32-bytes")
	v.Set("upstreams.commentInternalAuthToken", "comment-internal-token-with-at-least-32-bytes")
	v.Set("upstreams.contentInternalAuthToken", "content-internal-token-with-at-least-32-bytes")
	v.Set("grpc.server.tls.enabled", true)
	v.Set("grpc.server.tls.certFile", "/var/run/secrets/bbs/tls.crt")
	v.Set("grpc.server.tls.keyFile", "/var/run/secrets/bbs/tls.key")
	v.Set("grpc.server.tls.clientCAFile", "/var/run/secrets/bbs/ca.crt")
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

func TestApplyGRPCPortEnvOverrideUsesServerPortAndValidatesIt(t *testing.T) {
	t.Setenv("BBS_ADMIN_GRPC_SERVER_PORT", "19114")
	t.Setenv("BBS_ADMIN_SERVICE_GRPC_PORT", "19115")

	v := viper.New()
	if err := applyGRPCPortEnvOverride(v,
		"BBS_ADMIN_GRPC_SERVER_PORT",
		"BBS_ADMIN_SERVICE_GRPC_PORT",
	); err != nil {
		t.Fatalf("applyGRPCPortEnvOverride() error = %v", err)
	}
	if got := v.GetInt("service.grpcPort"); got != 19114 {
		t.Fatalf("service.grpcPort = %d, want 19114", got)
	}
	if got := v.GetInt("grpc.server.port"); got != 19114 {
		t.Fatalf("grpc.server.port = %d, want 19114", got)
	}

	t.Setenv("BBS_ADMIN_GRPC_SERVER_PORT", "invalid")
	if err := applyGRPCPortEnvOverride(v, "BBS_ADMIN_GRPC_SERVER_PORT"); err == nil {
		t.Fatal("applyGRPCPortEnvOverride() accepted an invalid port")
	}
}

func assertString(t *testing.T, v *viper.Viper, key string, want string) {
	t.Helper()
	if got := v.GetString(key); got != want {
		t.Fatalf("%s = %q, want %q", key, got, want)
	}
}
