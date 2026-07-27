package config

import (
	"testing"

	"github.com/spf13/viper"
)

func TestSkipNacosRequiresAnExplicitTruthyEnvironmentValue(t *testing.T) {
	t.Setenv("BBS_GATEWAY_SKIP_NACOS", "true")
	if !skipNacos() {
		t.Fatal("expected explicit true to skip Nacos")
	}
	t.Setenv("BBS_GATEWAY_SKIP_NACOS", "false")
	if skipNacos() {
		t.Fatal("expected false not to skip Nacos")
	}
}

func TestApplyNacosEnvOverrides(t *testing.T) {
	t.Setenv("BBS_GATEWAY_NACOS_ADDR", "nacos.internal")
	t.Setenv("BBS_GATEWAY_NACOS_PORT", "18848")
	t.Setenv("BBS_GATEWAY_NACOS_NAMESPACE_ID", "production")
	t.Setenv("BBS_GATEWAY_NACOS_DATAID", "gateway-production.yaml")
	t.Setenv("BBS_GATEWAY_NACOS_GROUP_ID", "BBS_PRODUCTION")

	v := viper.New()
	v.Set("nacos.addr", "127.0.0.1")
	v.Set("nacos.port", 8848)
	v.Set("nacos.namespaceId", "local")
	v.Set("nacos.dataId", "gateway-local.yaml")
	v.Set("nacos.groupId", "BBS_LOCAL")
	if err := applyNacosEnvOverrides(v); err != nil {
		t.Fatalf("apply Nacos environment overrides: %v", err)
	}

	var options Options
	if err := v.UnmarshalKey("nacos", &options); err != nil {
		t.Fatalf("unmarshal Nacos options: %v", err)
	}
	if options.Addr != "nacos.internal" || options.Port != 18848 {
		t.Fatalf("Nacos endpoint = %s:%d", options.Addr, options.Port)
	}
	if options.NamespaceID != "production" || options.DataID != "gateway-production.yaml" || options.GroupID != "BBS_PRODUCTION" {
		t.Fatalf("Nacos metadata = %#v", options)
	}
}

func TestApplyNacosEnvOverridesRejectsInvalidPort(t *testing.T) {
	t.Setenv("BBS_GATEWAY_NACOS_PORT", "not-a-port")

	if err := applyNacosEnvOverrides(viper.New()); err == nil {
		t.Fatal("expected invalid Nacos port to fail")
	}
}
