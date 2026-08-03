package app

import (
	"testing"

	"github.com/spf13/viper"
)

func TestProvideSnowflakeGeneratorUsesStatefulSetWorkerID(t *testing.T) {
	v := viper.New()
	v.Set("snowflake.workerId", 16)
	v.Set("snowflake.instanceName", "bbs-chat-service-7")
	v.Set("snowflake.workerIdRangeStart", 640)
	v.Set("snowflake.workerIdRangeSize", 192)

	ids, err := ProvideSnowflakeGenerator(v)
	if err != nil {
		t.Fatal(err)
	}
	id, err := ids.Next()
	if err != nil {
		t.Fatal(err)
	}
	if got := id >> 12 & 1023; got != 647 {
		t.Fatalf("worker bits = %d, want 647", got)
	}
}
