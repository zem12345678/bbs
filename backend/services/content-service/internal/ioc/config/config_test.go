package config

import (
	"testing"

	"github.com/spf13/viper"
)

func TestSetDefaultsFillsContentUpstreams(t *testing.T) {
	v := viper.New()
	configureEnv(v)

	setDefaults(v)

	if got := v.GetString("upstreams.comment"); got != "bbs-comment-service" {
		t.Fatalf("upstreams.comment = %q", got)
	}
	if got := v.GetString("upstreams.mall"); got != "bbs-mall-service" {
		t.Fatalf("upstreams.mall = %q", got)
	}
	if got := v.GetString("upstreams.credit"); got != "bbs-credit-service" {
		t.Fatalf("upstreams.credit = %q", got)
	}
}

func TestConfigureEnvBindsMallUpstream(t *testing.T) {
	t.Setenv("BBS_CONTENT_UPSTREAMS_MALL", "file-mall-service")
	v := viper.New()
	configureEnv(v)

	setDefaults(v)

	if got := v.GetString("upstreams.mall"); got != "file-mall-service" {
		t.Fatalf("upstreams.mall = %q", got)
	}
}
