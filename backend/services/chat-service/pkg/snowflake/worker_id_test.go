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
		{name: "fixed local worker", workerID: 16, want: 16},
		{name: "statefulset ordinal", workerID: 16, rangeStart: 640, rangeSize: 192, instanceName: "bbs-chat-service-7", want: 647},
		{name: "missing ordinal", rangeStart: 640, rangeSize: 192, instanceName: "bbs-chat-service", wantErr: "StatefulSet ordinal"},
		{name: "ordinal outside range", rangeStart: 640, rangeSize: 2, instanceName: "bbs-chat-service-2", wantErr: "exceeds"},
		{name: "range exceeds bits", rangeStart: 1000, rangeSize: 25, instanceName: "bbs-chat-service-0", wantErr: "range size"},
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
