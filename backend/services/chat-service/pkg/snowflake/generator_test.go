package snowflake

import (
	"testing"
	"time"
)

func TestGeneratorsUseDifferentWorkerBits(t *testing.T) {
	first, err := New(16)
	if err != nil {
		t.Fatal(err)
	}
	second, err := New(17)
	if err != nil {
		t.Fatal(err)
	}
	fixed := time.UnixMilli(epochMillis + 123)
	first.now = func() time.Time { return fixed }
	second.now = func() time.Time { return fixed }

	firstID, err := first.Next()
	if err != nil {
		t.Fatal(err)
	}
	secondID, err := second.Next()
	if err != nil {
		t.Fatal(err)
	}
	if firstID == secondID {
		t.Fatalf("worker ids generated duplicate id %d", firstID)
	}
	if firstID>>workerShift&(maxWorkerID) != 16 || secondID>>workerShift&(maxWorkerID) != 17 {
		t.Fatalf("unexpected worker bits: %d, %d", firstID, secondID)
	}
}

func TestGeneratorRejectsInvalidWorkerID(t *testing.T) {
	if _, err := New(-1); err == nil {
		t.Fatal("expected negative worker id to fail")
	}
	if _, err := New(maxWorkerID + 1); err == nil {
		t.Fatal("expected oversized worker id to fail")
	}
}
