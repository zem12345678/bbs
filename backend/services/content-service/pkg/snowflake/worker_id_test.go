package snowflake

import (
	"strings"
	"testing"
)

func TestResolveWorkerID(t *testing.T) {
	tests := []struct {
		name         string
		workerID     int64
		rangeStart   int64
		rangeSize    int64
		instanceName string
		want         int64
		wantErr      string
	}{
		{name: "fixed local worker", workerID: 3, want: 3},
		{name: "statefulset ordinal", workerID: 3, rangeStart: 256, rangeSize: 192, instanceName: "bbs-content-service-7", want: 263},
		{name: "missing ordinal", rangeStart: 256, rangeSize: 192, instanceName: "bbs-content-service", wantErr: "StatefulSet ordinal"},
		{name: "ordinal outside range", rangeStart: 256, rangeSize: 2, instanceName: "bbs-content-service-2", wantErr: "exceeds"},
		{name: "range exceeds bits", rangeStart: 1000, rangeSize: 25, instanceName: "bbs-content-service-0", wantErr: "range size"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got, err := ResolveWorkerID(test.workerID, test.rangeStart, test.rangeSize, test.instanceName)
			if test.wantErr != "" {
				if err == nil || !strings.Contains(err.Error(), test.wantErr) {
					t.Fatalf("ResolveWorkerID error = %v, want %q", err, test.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if got != test.want {
				t.Fatalf("ResolveWorkerID = %d, want %d", got, test.want)
			}
		})
	}
}
