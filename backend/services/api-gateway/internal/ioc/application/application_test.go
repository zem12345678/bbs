package application

import (
	"errors"
	"reflect"
	"testing"

	"go.uber.org/zap"
)

func TestStartRollsBackStartedComponentsInReverseOrder(t *testing.T) {
	var calls []string
	app, err := New("test", zap.NewNop(), ComponentOptions(
		&componentStub{name: "first", calls: &calls},
		&componentStub{name: "second", calls: &calls},
		&componentStub{name: "failed", calls: &calls, startErr: errors.New("boom")},
	))
	if err != nil {
		t.Fatal(err)
	}

	if err := app.Start(); err == nil {
		t.Fatal("expected component start error")
	}
	want := []string{"start:first", "start:second", "start:failed", "stop:second", "stop:first"}
	if !reflect.DeepEqual(calls, want) {
		t.Fatalf("calls = %#v, want %#v", calls, want)
	}
}

type componentStub struct {
	name     string
	calls    *[]string
	startErr error
}

func (c *componentStub) Start() error {
	*c.calls = append(*c.calls, "start:"+c.name)
	return c.startErr
}

func (c *componentStub) Stop() error {
	*c.calls = append(*c.calls, "stop:"+c.name)
	return nil
}
