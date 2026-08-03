package app

import (
	"testing"

	"github.com/spf13/viper"
)

func TestProvideIDGeneratorUsesStatefulSetWorkerID(t *testing.T) {
	v := viper.New()
	v.Set("snowflake.workerId", 4)
	v.Set("snowflake.instanceName", "bbs-comment-service-7")
	v.Set("snowflake.workerIdRangeStart", 448)
	v.Set("snowflake.workerIdRangeSize", 192)

	ids, err := ProvideIDGenerator(v)
	if err != nil {
		t.Fatal(err)
	}
	if got := ids.Generate() >> 12 & 1023; got != 455 {
		t.Fatalf("worker bits = %d, want 455", got)
	}
}
