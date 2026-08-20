package config

import (
	"bytes"
	"crypto/elliptic"
	"encoding/base64"
	"reflect"
	"testing"

	"github.com/spf13/viper"
)

func TestSkipNacos(t *testing.T) {
	t.Setenv("BBS_NOTIFICATION_SKIP_NACOS", "")
	if skipNacos() {
		t.Fatal("skipNacos() = true without override")
	}
	t.Setenv("BBS_NOTIFICATION_SKIP_NACOS", "true")
	if !skipNacos() {
		t.Fatal("skipNacos() = false with BBS_NOTIFICATION_SKIP_NACOS=true")
	}
}

func TestValidateWebPushRequiresCompleteValidVAPIDConfig(t *testing.T) {
	v := viper.New()
	v.Set("webPush.enabled", true)
	if err := validateWebPush(v); err == nil {
		t.Fatal("enabled incomplete web push config was accepted")
	}
	privateBytes := make([]byte, 32)
	privateBytes[31] = 2
	publicX, publicY := elliptic.P256().ScalarBaseMult(privateBytes)
	publicBytes := elliptic.Marshal(elliptic.P256(), publicX, publicY)
	v.Set("webPush.subject", "mailto:admin@example.com")
	v.Set("webPush.publicKey", base64.RawURLEncoding.EncodeToString(publicBytes))
	v.Set("webPush.privateKey", base64.RawURLEncoding.EncodeToString(privateBytes))
	if err := validateWebPush(v); err != nil {
		t.Fatalf("valid web push config: %v", err)
	}
	v.Set("webPush.subject", "http://example.com")
	if err := validateWebPush(v); err == nil {
		t.Fatal("invalid web push subject was accepted")
	}
}

func TestValidateWebhookRequiresPublicProductionConfiguration(t *testing.T) {
	local := viper.New()
	local.Set("webhook.enabled", true)
	local.Set("webhook.serverURL", "http://127.0.0.1:18080")
	local.Set("webhook.allowPrivateEndpoints", true)
	if err := validateWebhook(local, false); err != nil {
		t.Fatalf("valid local webhook config: %v", err)
	}
	if err := validateWebhook(local, true); err == nil {
		t.Fatal("unsafe production webhook config was accepted")
	}
	local.Set("webhook.serverURL", "https://bbs.example.test")
	local.Set("webhook.allowPrivateEndpoints", false)
	if err := validateWebhook(local, true); err != nil {
		t.Fatalf("valid production webhook config: %v", err)
	}
}

func TestValidateVAPIDKeyPairRejectsInvalidAndMismatchedKeys(t *testing.T) {
	privateKey := make([]byte, 32)
	privateKey[31] = 3
	publicX, publicY := elliptic.P256().ScalarBaseMult(privateKey)
	publicKey := elliptic.Marshal(elliptic.P256(), publicX, publicY)
	if err := validateVAPIDKeyPair(publicKey, privateKey); err != nil {
		t.Fatalf("valid key pair: %v", err)
	}
	if err := validateVAPIDKeyPair(append([]byte{4}, bytes.Repeat([]byte{1}, 64)...), privateKey); err == nil {
		t.Fatal("invalid public curve point was accepted")
	}
	if err := validateVAPIDKeyPair(publicKey, make([]byte, 32)); err == nil {
		t.Fatal("zero private scalar was accepted")
	}
	mismatchedPrivateKey := make([]byte, 32)
	mismatchedPrivateKey[31] = 4
	if err := validateVAPIDKeyPair(publicKey, mismatchedPrivateKey); err == nil {
		t.Fatal("mismatched key pair was accepted")
	}
}

func TestApplyEnvOverridesSetsWebPushConfig(t *testing.T) {
	t.Setenv("BBS_NOTIFICATION_WEB_PUSH_ENABLED", "true")
	t.Setenv("BBS_NOTIFICATION_WEB_PUSH_SUBJECT", "mailto:push@example.com")
	t.Setenv("BBS_NOTIFICATION_WEB_PUSH_PUBLIC_KEY", "public")
	t.Setenv("BBS_NOTIFICATION_WEB_PUSH_PRIVATE_KEY", "private")
	v := viper.New()
	applyEnvOverrides(v)
	if !v.GetBool("webPush.enabled") || v.GetString("webPush.subject") != "mailto:push@example.com" ||
		v.GetString("webPush.publicKey") != "public" || v.GetString("webPush.privateKey") != "private" {
		t.Fatalf("web push env config = %#v", v.GetStringMap("webPush"))
	}
}

func TestApplyGRPCPortEnvOverride(t *testing.T) {
	t.Setenv("BBS_NOTIFICATION_GRPC_SERVER_PORT", "19108")

	v := viper.New()
	if err := applyGRPCPortEnvOverride(v,
		"BBS_NOTIFICATION_GRPC_SERVER_PORT",
		"BBS_NOTIFICATION_SERVICE_GRPC_PORT",
	); err != nil {
		t.Fatalf("applyGRPCPortEnvOverride() error = %v", err)
	}
	if got := v.GetInt("service.grpcPort"); got != 19108 {
		t.Fatalf("service.grpcPort = %d, want 19108", got)
	}
	if got := v.GetInt("grpc.server.port"); got != 19108 {
		t.Fatalf("grpc.server.port = %d, want 19108", got)
	}

	t.Setenv("BBS_NOTIFICATION_GRPC_SERVER_PORT", "70000")
	if err := applyGRPCPortEnvOverride(v, "BBS_NOTIFICATION_GRPC_SERVER_PORT"); err == nil {
		t.Fatal("applyGRPCPortEnvOverride() accepted an invalid port")
	}
}

func TestApplyEnvOverridesSetsRuntimeConfig(t *testing.T) {
	t.Setenv("BBS_NOTIFICATION_POSTGRES_DSN", "postgres://notification")
	t.Setenv("BBS_NOTIFICATION_POSTGRES_DEBUG", "true")
	t.Setenv("BBS_NOTIFICATION_POSTGRES_MAX_OPEN_CONNS", "8")
	t.Setenv("BBS_NOTIFICATION_KAFKA_BROKERS", "kafka-a:9092, kafka-b:9092")
	t.Setenv("BBS_NOTIFICATION_KAFKA_ARTICLE_TOPIC", "env.article.events")
	t.Setenv("BBS_NOTIFICATION_KAFKA_USER_TOPIC", "env.user.events")
	t.Setenv("BBS_NOTIFICATION_KAFKA_MALL_TOPIC", "env.mall.events")
	t.Setenv("BBS_NOTIFICATION_KAFKA_ARTICLE_GROUP_ID", "env-article-group")
	t.Setenv("BBS_NOTIFICATION_KAFKA_USER_GROUP_ID", "env-user-group")
	t.Setenv("BBS_NOTIFICATION_KAFKA_MALL_GROUP_ID", "env-mall-group")
	t.Setenv("BBS_NOTIFICATION_GRPC_SERVER_ETCD_ADDR", "etcd-a:2379, etcd-b:2379")
	t.Setenv("BBS_NOTIFICATION_GRPC_SERVER_INTERNAL_AUTH_TOKEN", "env-notification-internal-token")
	t.Setenv("BBS_NOTIFICATION_USER_INTERNAL_AUTH_TOKEN", "env-user-internal-token")
	t.Setenv("BBS_NOTIFICATION_TRACE_ENV", "production")

	v := viper.New()
	applyEnvOverrides(v)

	if got := v.GetString("postgres.dsn"); got != "postgres://notification" {
		t.Fatalf("postgres.dsn = %q", got)
	}
	if !v.GetBool("postgres.debug") {
		t.Fatal("postgres.debug = false, want true")
	}
	if got := v.GetInt("postgres.max_open_conns"); got != 8 {
		t.Fatalf("postgres.max_open_conns = %d", got)
	}
	if got, want := v.GetStringSlice("kafka.brokers"), []string{"kafka-a:9092", "kafka-b:9092"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("kafka.brokers = %#v, want %#v", got, want)
	}
	if got := v.GetString("kafka.userTopic"); got != "env.user.events" {
		t.Fatalf("kafka.userTopic = %q", got)
	}
	if got := v.GetString("kafka.mallGroupId"); got != "env-mall-group" {
		t.Fatalf("kafka.mallGroupId = %q", got)
	}
	if got, want := v.GetStringSlice("grpc.server.etcdAddr"), []string{"etcd-a:2379", "etcd-b:2379"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("grpc.server.etcdAddr = %#v, want %#v", got, want)
	}
	if got := v.GetString("grpc.server.internalAuthToken"); got != "env-notification-internal-token" {
		t.Fatalf("grpc.server.internalAuthToken = %q", got)
	}
	if got := v.GetString("upstreams.userInternalAuthToken"); got != "env-user-internal-token" {
		t.Fatalf("upstreams.userInternalAuthToken = %q", got)
	}
	if got := v.GetString("trace.env"); got != "production" {
		t.Fatalf("trace.env = %q", got)
	}
}

func TestInternalAuthDefaultAndProductionValidation(t *testing.T) {
	local := viper.New()
	local.Set("trace.env", "local")
	setInternalAuthDefault(local)
	if got := local.GetString("grpc.server.internalAuthToken"); got != localDevInternalAuthToken {
		t.Fatalf("local internal auth token = %q", got)
	}
	if err := validate(local); err != nil {
		t.Fatalf("validate local config: %v", err)
	}

	for _, tc := range []struct {
		name  string
		token string
	}{
		{name: "empty", token: ""},
		{name: "default", token: localDevInternalAuthToken},
		{name: "short", token: "too-short"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			v := viper.New()
			v.Set("trace.env", "production")
			v.Set("grpc.server.internalAuthToken", tc.token)
			if err := validate(v); err == nil {
				t.Fatal("validate production config error = nil")
			}
		})
	}

	production := viper.New()
	production.Set("trace.env", "prod")
	production.Set("grpc.server.internalAuthToken", "production-notification-token-with-at-least-32-bytes")
	production.Set("upstreams.userInternalAuthToken", "production-user-token-with-at-least-32-bytes")
	if err := validate(production); err != nil {
		t.Fatalf("validate production config: %v", err)
	}
}
