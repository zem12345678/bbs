package config

import (
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/spf13/viper"
)

// setNestedConfigValue must not regress in two ways that both broke services at
// runtime: writing one leaf used to wipe every sibling of that subtree (viper's
// override layer stores a partial map, so UnmarshalKey returned only that leaf),
// and routing the write through the config layer instead let AutomaticEnv/BindEnv
// outrank it, so a comma-split list collapsed back into one raw env string.
func TestSetNestedConfigValueKeepsSiblingsAndOutranksEnv(t *testing.T) {
	const yaml = "grpc:\n" +
		"  server:\n" +
		"    port: 9104\n" +
		"    serviceName: from-file\n" +
		"    timeout: 5s\n" +
		"    etcdAddr:\n" +
		"      - 127.0.0.1:2379\n" +
		"    rateLimit:\n" +
		"      interval: 1s\n" +
		"      rate: 100\n" +
		"  client:\n" +
		"    tag: from-file-tag\n"

	newViper := func(t *testing.T) *viper.Viper {
		t.Helper()
		v := viper.New()
		v.SetConfigType("yaml")
		if err := v.ReadConfig(strings.NewReader(yaml)); err != nil {
			t.Fatalf("seed config: %v", err)
		}
		return v
	}

	type serverOptions struct {
		Port        int           `mapstructure:"port"`
		ServiceName string        `mapstructure:"serviceName"`
		Timeout     time.Duration `mapstructure:"timeout"`
		EtcdAddr    []string      `mapstructure:"etcdAddr"`
		RateLimit   struct {
			Interval time.Duration `mapstructure:"interval"`
			Rate     int           `mapstructure:"rate"`
		} `mapstructure:"rateLimit"`
	}

	unmarshalServer := func(t *testing.T, v *viper.Viper) serverOptions {
		t.Helper()
		var o serverOptions
		if err := v.UnmarshalKey("grpc.server", &o); err != nil {
			t.Fatalf("unmarshal grpc.server: %v", err)
		}
		return o
	}

	t.Run("writing one leaf keeps every sibling", func(t *testing.T) {
		v := newViper(t)
		setNestedConfigValue(v, "grpc.server.internalAuthToken", "written-token")

		o := unmarshalServer(t, v)
		if o.Port != 9104 {
			t.Errorf("port = %d, want 9104", o.Port)
		}
		if o.ServiceName != "from-file" {
			t.Errorf("serviceName = %q, want %q", o.ServiceName, "from-file")
		}
		if o.Timeout != 5*time.Second {
			t.Errorf("timeout = %v, want 5s", o.Timeout)
		}
		if want := []string{"127.0.0.1:2379"}; !reflect.DeepEqual(o.EtcdAddr, want) {
			t.Errorf("etcdAddr = %#v, want %#v", o.EtcdAddr, want)
		}
		if o.RateLimit.Interval != time.Second || o.RateLimit.Rate != 100 {
			t.Errorf("rateLimit = {%v %d}, want {1s 100}", o.RateLimit.Interval, o.RateLimit.Rate)
		}
		if got := v.GetString("grpc.server.internalAuthToken"); got != "written-token" {
			t.Errorf("internalAuthToken = %q, want %q", got, "written-token")
		}
		if got := v.GetString("grpc.client.tag"); got != "from-file-tag" {
			t.Errorf("grpc.client.tag = %q, want %q", got, "from-file-tag")
		}
	})

	t.Run("repeated writes accumulate", func(t *testing.T) {
		v := newViper(t)
		setNestedConfigValue(v, "grpc.server.port", 29104)
		setNestedConfigValue(v, "grpc.server.rateLimit.rate", 999)
		setNestedConfigValue(v, "grpc.server.internalAuthToken", "written-token")
		setNestedConfigValue(v, "grpc.client.serverName", "written-server")

		o := unmarshalServer(t, v)
		if o.Port != 29104 {
			t.Errorf("port = %d, want 29104", o.Port)
		}
		if o.RateLimit.Rate != 999 {
			t.Errorf("rateLimit.rate = %d, want 999", o.RateLimit.Rate)
		}
		if o.RateLimit.Interval != time.Second {
			t.Errorf("rateLimit.interval = %v, want 1s", o.RateLimit.Interval)
		}
		if o.ServiceName != "from-file" {
			t.Errorf("serviceName = %q, want %q", o.ServiceName, "from-file")
		}
		if o.Timeout != 5*time.Second {
			t.Errorf("timeout = %v, want 5s", o.Timeout)
		}
		if got := v.GetString("grpc.server.internalAuthToken"); got != "written-token" {
			t.Errorf("internalAuthToken = %q, want %q", got, "written-token")
		}
		if got := v.GetString("grpc.client.tag"); got != "from-file-tag" {
			t.Errorf("grpc.client.tag = %q, want %q", got, "from-file-tag")
		}
		if got := v.GetString("grpc.client.serverName"); got != "written-server" {
			t.Errorf("grpc.client.serverName = %q, want %q", got, "written-server")
		}
	})

	t.Run("split list wins over the raw env string", func(t *testing.T) {
		v := newViper(t)
		t.Setenv("BBS_NESTED_WRITE_PROBE_ETCD_ADDR", "10.0.0.1:2379,10.0.0.2:2379")
		if err := v.BindEnv("grpc.server.etcdAddr", "BBS_NESTED_WRITE_PROBE_ETCD_ADDR"); err != nil {
			t.Fatalf("bind env: %v", err)
		}
		// Without the override layer the env binding outranks the write and the
		// flat read collapses to a single "a,b" element.
		setNestedConfigValue(v, "grpc.server.etcdAddr", []string{"10.0.0.1:2379", "10.0.0.2:2379"})

		want := []string{"10.0.0.1:2379", "10.0.0.2:2379"}
		if got := v.GetStringSlice("grpc.server.etcdAddr"); !reflect.DeepEqual(got, want) {
			t.Errorf("flat etcdAddr = %#v, want %#v", got, want)
		}
		o := unmarshalServer(t, v)
		if !reflect.DeepEqual(o.EtcdAddr, want) {
			t.Errorf("unmarshaled etcdAddr = %#v, want %#v", o.EtcdAddr, want)
		}
		if o.Port != 9104 || o.ServiceName != "from-file" {
			t.Errorf("siblings lost: port=%d serviceName=%q", o.Port, o.ServiceName)
		}
	})

	t.Run("env provided sibling keeps its env value", func(t *testing.T) {
		v := newViper(t)
		t.Setenv("BBS_NESTED_WRITE_PROBE_SERVICE_NAME", "from-env")
		if err := v.BindEnv("grpc.server.serviceName", "BBS_NESTED_WRITE_PROBE_SERVICE_NAME"); err != nil {
			t.Fatalf("bind env: %v", err)
		}
		setNestedConfigValue(v, "grpc.server.port", 29104)

		o := unmarshalServer(t, v)
		if o.ServiceName != "from-env" {
			t.Errorf("serviceName = %q, want %q", o.ServiceName, "from-env")
		}
		if o.Port != 29104 {
			t.Errorf("port = %d, want 29104", o.Port)
		}
	})
}
