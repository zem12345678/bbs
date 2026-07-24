//go:build integration

package config

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"

	"github.com/nacos-group/nacos-sdk-go/v2/clients"
	"github.com/nacos-group/nacos-sdk-go/v2/common/constant"
	"github.com/nacos-group/nacos-sdk-go/v2/vo"
	"github.com/spf13/viper"
)

func TestNacosLoadsChatServiceConfiguration(t *testing.T) {
	if os.Getenv("BBS_CHAT_TEST_NACOS") == "" {
		t.Skip("set BBS_CHAT_TEST_NACOS to run the Nacos integration test")
	}
	configPath, err := filepath.Abs("../../../configs/config.yaml")
	if err != nil {
		t.Fatalf("resolve local chat config: %v", err)
	}
	bootstrap := viper.New()
	bootstrap.SetConfigFile(configPath)
	if err := bootstrap.ReadInConfig(); err != nil {
		t.Fatalf("read local chat config: %v", err)
	}
	var endpoint Options
	if err := bootstrap.UnmarshalKey("nacos", &endpoint); err != nil {
		t.Fatalf("unmarshal Nacos endpoint: %v", err)
	}
	client, err := clients.CreateConfigClient(map[string]interface{}{
		"serverConfigs": []constant.ServerConfig{{IpAddr: endpoint.Addr, Port: endpoint.Port}},
		"clientConfig": constant.ClientConfig{
			NamespaceId: endpoint.NamespaceID, TimeoutMs: 5000, NotLoadCacheAtStart: true,
			LogDir: filepath.Join(t.TempDir(), "log"), CacheDir: filepath.Join(t.TempDir(), "cache"), LogLevel: "error",
		},
	})
	if err != nil {
		t.Fatalf("create Nacos config client: %v", err)
	}
	content, err := client.GetConfig(vo.ConfigParam{DataId: endpoint.DataID, Group: stringDefault(endpoint.GroupID, "DEFAULT_GROUP")})
	client.CloseClient()
	if err != nil {
		t.Fatalf("load chat configuration through Nacos: %v", err)
	}
	v := viper.New()
	v.SetConfigType("yaml")
	if err := v.MergeConfig(bytes.NewBufferString(content)); err != nil {
		t.Fatalf("parse Nacos chat configuration: %v", err)
	}
	if got := v.GetString("service.name"); got != "bbs-chat-service" {
		t.Fatalf("service.name = %q, want bbs-chat-service", got)
	}
	if got := v.GetString("kafka.producerOptions.topic"); got != "chat.events" {
		t.Fatalf("kafka.producerOptions.topic = %q, want chat.events", got)
	}
	if got := v.GetString("kafka.consumerOptions.groupId"); got != "bbs-chat-realtime" {
		t.Fatalf("kafka.consumerOptions.groupId = %q, want bbs-chat-realtime", got)
	}
}
