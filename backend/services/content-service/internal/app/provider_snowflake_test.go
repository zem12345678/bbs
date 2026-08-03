package app

import (
	"testing"

	"github.com/spf13/viper"
)

func TestProvideSnowflakeNodeUsesStatefulSetWorkerID(t *testing.T) {
	v := viper.New()
	v.Set("snowflake.workerId", 3)
	v.Set("snowflake.instanceName", "bbs-content-service-7")
	v.Set("snowflake.workerIdRangeStart", 256)
	v.Set("snowflake.workerIdRangeSize", 192)

	ids, err := ProvideSnowflakeNode(v)
	if err != nil {
		t.Fatal(err)
	}
	if got := ids.Generate() >> 12 & 1023; got != 263 {
		t.Fatalf("worker bits = %d, want 263", got)
	}
}
