package config

import (
	"testing"

	"github.com/spf13/viper"
)

func TestSkipNacos(t *testing.T) {
	t.Setenv("BBS_FILE_SKIP_NACOS", "")
	if skipNacos() {
		t.Fatal("skipNacos() = true without override")
	}
	t.Setenv("BBS_FILE_SKIP_NACOS", "true")
	if !skipNacos() {
		t.Fatal("skipNacos() = false with BBS_FILE_SKIP_NACOS=true")
	}
}

func TestSetDefaultsFillsMallInternalAuthToken(t *testing.T) {
	v := viper.New()
	configureEnv(v)

	setDefaults(v)

	if got := v.GetString("upstreams.mallInternalAuthToken"); got != localDevMallInternalAuthToken {
		t.Fatalf("upstreams.mallInternalAuthToken = %q", got)
	}
	if got := v.GetString("upstreams.creditInternalAuthToken"); got != localDevCreditInternalAuthToken {
		t.Fatalf("upstreams.creditInternalAuthToken = %q", got)
	}
	if got := v.GetString("upstreams.contentInternalAuthToken"); got != localDevContentInternalAuthToken {
		t.Fatalf("upstreams.contentInternalAuthToken = %q", got)
	}
	if got := v.GetString("grpc.server.internalAuthToken"); got != localDevFileInternalAuthToken {
		t.Fatalf("grpc.server.internalAuthToken = %q", got)
	}
	if got := v.GetInt64("files.capacityBytes"); got != defaultFileCapacityBytes {
		t.Fatalf("files.capacityBytes = %d, want %d", got, defaultFileCapacityBytes)
	}
}

func TestConfigureEnvBindsFileCapacity(t *testing.T) {
	t.Setenv("BBS_FILE_FILES_CAPACITY_BYTES", "209715200")
	v := viper.New()
	configureEnv(v)
	setDefaults(v)

	if got := v.GetInt64("files.capacityBytes"); got != 209715200 {
		t.Fatalf("files.capacityBytes = %d", got)
	}
}

func TestConfigureEnvBindsStorageSettings(t *testing.T) {
	t.Setenv("BBS_FILE_STORAGE_ENDPOINT", "http://127.0.0.1:19000")
	t.Setenv("BBS_FILE_STORAGE_BUCKET", "bbs-local")
	t.Setenv("BBS_FILE_STORAGE_ACCESS_KEY", "access")
	t.Setenv("BBS_FILE_STORAGE_SECRET_KEY", "secret")
	v := viper.New()
	configureEnv(v)

	if got := v.GetString("storage.endpoint"); got != "http://127.0.0.1:19000" {
		t.Fatalf("storage.endpoint = %q", got)
	}
	if got := v.GetString("storage.bucket"); got != "bbs-local" {
		t.Fatalf("storage.bucket = %q", got)
	}
	if got := v.GetString("storage.accessKey"); got != "access" {
		t.Fatalf("storage.accessKey = %q", got)
	}
	if got := v.GetString("storage.secretKey"); got != "secret" {
		t.Fatalf("storage.secretKey = %q", got)
	}
}

func TestConfigureEnvBindsMallInternalAuthToken(t *testing.T) {
	t.Setenv("BBS_FILE_UPSTREAMS_MALL_INTERNAL_AUTH_TOKEN", "configured-mall-token")
	v := viper.New()
	configureEnv(v)

	setDefaults(v)

	if got := v.GetString("upstreams.mallInternalAuthToken"); got != "configured-mall-token" {
		t.Fatalf("upstreams.mallInternalAuthToken = %q", got)
	}
}

func TestConfigureEnvBindsCreditInternalAuthToken(t *testing.T) {
	t.Setenv("BBS_FILE_UPSTREAMS_CREDIT_INTERNAL_AUTH_TOKEN", "configured-credit-token")
	v := viper.New()
	configureEnv(v)

	setDefaults(v)

	if got := v.GetString("upstreams.creditInternalAuthToken"); got != "configured-credit-token" {
		t.Fatalf("upstreams.creditInternalAuthToken = %q", got)
	}
}

func TestConfigureEnvBindsContentInternalAuthToken(t *testing.T) {
	t.Setenv("BBS_FILE_UPSTREAMS_CONTENT_INTERNAL_AUTH_TOKEN", "configured-content-token")
	v := viper.New()
	configureEnv(v)

	setDefaults(v)

	if got := v.GetString("upstreams.contentInternalAuthToken"); got != "configured-content-token" {
		t.Fatalf("upstreams.contentInternalAuthToken = %q", got)
	}
}

func TestConfigureEnvBindsFileInternalAuthToken(t *testing.T) {
	t.Setenv("BBS_FILE_GRPC_SERVER_INTERNAL_AUTH_TOKEN", "configured-file-token")
	v := viper.New()
	configureEnv(v)

	setDefaults(v)

	if got := v.GetString("grpc.server.internalAuthToken"); got != "configured-file-token" {
		t.Fatalf("grpc.server.internalAuthToken = %q", got)
	}
}

func TestValidateRejectsDefaultMallInternalAuthTokenInProduction(t *testing.T) {
	v := viper.New()
	setDefaults(v)
	v.Set("trace.env", "production")

	if err := validate(v); err == nil {
		t.Fatal("validate() error = nil, want default mall token rejection")
	}
}

func TestValidateRejectsDefaultCreditInternalAuthTokenInProduction(t *testing.T) {
	v := viper.New()
	setDefaults(v)
	v.Set("trace.env", "production")
	v.Set("upstreams.mallInternalAuthToken", "production-mall-internal-token-with-32-bytes")

	if err := validate(v); err == nil {
		t.Fatal("validate() error = nil, want default credit token rejection")
	}
}

func TestValidateRejectsDefaultFileInternalAuthTokenInProduction(t *testing.T) {
	v := viper.New()
	setDefaults(v)
	v.Set("trace.env", "production")
	v.Set("upstreams.mallInternalAuthToken", "production-mall-internal-token-with-32-bytes")
	v.Set("upstreams.creditInternalAuthToken", "production-credit-internal-token-with-32-bytes")

	if err := validate(v); err == nil {
		t.Fatal("validate() error = nil, want default file token rejection")
	}
}

func TestValidateAcceptsConfiguredInternalAuthTokensInProduction(t *testing.T) {
	v := viper.New()
	setDefaults(v)
	v.Set("trace.env", "production")
	v.Set("upstreams.mallInternalAuthToken", "production-mall-internal-token-with-32-bytes")
	v.Set("upstreams.creditInternalAuthToken", "production-credit-internal-token-with-32-bytes")
	v.Set("grpc.server.internalAuthToken", "production-file-internal-token-with-32-bytes")
	v.Set("upstreams.contentInternalAuthToken", "production-content-internal-token-with-32-bytes")
	v.Set("storage.bucket", "bbs-production")
	v.Set("storage.accessKey", "production-access-key")
	v.Set("storage.secretKey", "production-secret-key")

	if err := validate(v); err != nil {
		t.Fatalf("validate() error = %v", err)
	}
}

func TestValidateRejectsMissingStorageCredentialsInProduction(t *testing.T) {
	v := viper.New()
	setDefaults(v)
	v.Set("trace.env", "production")
	v.Set("upstreams.mallInternalAuthToken", "production-mall-internal-token-with-32-bytes")
	v.Set("upstreams.creditInternalAuthToken", "production-credit-internal-token-with-32-bytes")
	v.Set("grpc.server.internalAuthToken", "production-file-internal-token-with-32-bytes")
	v.Set("upstreams.contentInternalAuthToken", "production-content-internal-token-with-32-bytes")

	if err := validate(v); err == nil {
		t.Fatal("validate() error = nil, want missing storage credentials rejection")
	}
}

func TestValidateRejectsDefaultContentInternalAuthTokenInProduction(t *testing.T) {
	v := viper.New()
	setDefaults(v)
	v.Set("trace.env", "production")
	v.Set("upstreams.mallInternalAuthToken", "production-mall-internal-token-with-32-bytes")
	v.Set("upstreams.creditInternalAuthToken", "production-credit-internal-token-with-32-bytes")
	v.Set("grpc.server.internalAuthToken", "production-file-internal-token-with-32-bytes")

	if err := validate(v); err == nil {
		t.Fatal("validate() error = nil, want default content token rejection")
	}
}

func TestValidateRejectsShortContentInternalAuthTokenInProduction(t *testing.T) {
	v := viper.New()
	setDefaults(v)
	v.Set("trace.env", "production")
	v.Set("upstreams.mallInternalAuthToken", "production-mall-internal-token-with-32-bytes")
	v.Set("upstreams.creditInternalAuthToken", "production-credit-internal-token-with-32-bytes")
	v.Set("grpc.server.internalAuthToken", "production-file-internal-token-with-32-bytes")
	v.Set("upstreams.contentInternalAuthToken", "too-short")

	if err := validate(v); err == nil {
		t.Fatal("validate() error = nil, want short content token rejection")
	}
}
