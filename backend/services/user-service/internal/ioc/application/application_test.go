package application

import (
	"errors"
	"reflect"
	"testing"

	"go.uber.org/zap"
)

func TestStartStopsPreviouslyStartedComponentsWhenLaterComponentFails(t *testing.T) {
	calls := make([]string, 0, 3)
	first := &componentStub{name: "first", calls: &calls}
	second := &componentStub{name: "second", calls: &calls, startErr: errors.New("start failed")}
	app, err := New("user", zap.NewNop(), ComponentOptions(first, second))
	if err != nil {
		t.Fatal(err)
	}
	if err := app.Start(); err == nil || err.Error() != "application component start error: start failed" {
		t.Fatalf("Start() error = %v", err)
	}
	if got, want := calls, []string{"start:first", "start:second", "stop:first"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("calls = %#v, want %#v", got, want)
	}
}

func TestStartStartsComponentsInDeclaredOrder(t *testing.T) {
	calls := make([]string, 0, 2)
	first := &componentStub{name: "first", calls: &calls}
	second := &componentStub{name: "second", calls: &calls}
	app, err := New("user", zap.NewNop(), ComponentOptions(first, second))
	if err != nil {
		t.Fatal(err)
	}
	if err := app.Start(); err != nil {
		t.Fatal(err)
	}
	if got, want := calls, []string{"start:first", "start:second"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("calls = %#v, want %#v", got, want)
	}
}

type componentStub struct {
	name     string
	calls    *[]string
	startErr error
}

func (s *componentStub) Start() error {
	*s.calls = append(*s.calls, "start:"+s.name)
	return s.startErr
}

func (s *componentStub) Stop() error {
	*s.calls = append(*s.calls, "stop:"+s.name)
	return nil
}
