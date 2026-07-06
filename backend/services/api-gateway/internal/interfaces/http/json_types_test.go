package http

import (
	"encoding/json"
	"testing"
)

func TestJsonInt64Unmarshal(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		body string
		want int64
	}{
		{name: "number", body: `{"parent_id":123}`, want: 123},
		{name: "large string", body: `{"parent_id":"332596060432121856"}`, want: 332596060432121856},
		{name: "empty string", body: `{"parent_id":""}`, want: 0},
		{name: "null", body: `{"parent_id":null}`, want: 0},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			var req createCommentRequest
			if err := json.Unmarshal([]byte(tt.body), &req); err != nil {
				t.Fatalf("unmarshal: %v", err)
			}
			if got := req.ParentID.Int64(); got != tt.want {
				t.Fatalf("parent_id = %d, want %d", got, tt.want)
			}
		})
	}
}

func TestJsonInt64UnmarshalRejectsInvalidValue(t *testing.T) {
	t.Parallel()

	var req createCommentRequest
	if err := json.Unmarshal([]byte(`{"parent_id":"abc"}`), &req); err == nil {
		t.Fatal("expected invalid parent_id to fail")
	}
}
