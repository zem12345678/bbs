package config

import "testing"

func TestLoadLocalConfigWhenNacosIsSkipped(t *testing.T) {
	t.Setenv("BBS_CHAT_SKIP_NACOS", "1")
	t.Setenv("BBS_CHAT_SNOWFLAKE_WORKER_ID", "19")
	t.Setenv("BBS_CHAT_ETCD_ADDR", "127.0.0.1:2379")

	cfg, err := Load("../../configs/config.yaml")
	if err != nil {
		t.Fatal(err)
	}
	if cfg.GRPC.Server.Port != 9116 || cfg.Snowflake.WorkerID != 19 {
		t.Fatalf("unexpected config: %#v", cfg)
	}
	if cfg.Postgres.DSN == "" || cfg.GRPC.Server.ServiceName != "bbs-chat-service" {
		t.Fatalf("incomplete config: %#v", cfg)
	}
}
