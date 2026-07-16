package config

import (
	"testing"

	"github.com/spf13/viper"
)

func TestSetDefaultsFillsMallUpstream(t *testing.T) {
	v := viper.New()
	configureEnv(v)

	setDefaults(v)

	if got := v.GetString("upstreams.mall"); got != "bbs-mall-service" {
		t.Fatalf("upstreams.mall = %q", got)
	}
}

func TestConfigureEnvBindsMallUpstream(t *testing.T) {
	t.Setenv("BBS_USER_UPSTREAMS_MALL", "file-mall-service")
	v := viper.New()
	configureEnv(v)

	setDefaults(v)

	if got := v.GetString("upstreams.mall"); got != "file-mall-service" {
		t.Fatalf("upstreams.mall = %q", got)
	}
}
