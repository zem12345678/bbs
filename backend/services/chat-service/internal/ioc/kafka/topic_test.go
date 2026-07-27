package kafka

import (
	"context"
	"errors"
	"reflect"
	"strings"
	"testing"
	"time"

	segmentkafka "github.com/segmentio/kafka-go"
)

func TestVerifyTopicWithDialer(t *testing.T) {
	partitionErr := errors.New("unknown topic")
	tests := []struct {
		name       string
		brokers    []string
		results    map[string]topicDialResult
		wantErr    string
		wantDialed []string
		wantClosed int
	}{
		{
			name: "readable partition",
			results: map[string]topicDialResult{
				"broker-a": {conn: &topicConnectionStub{partitions: []segmentkafka.Partition{{Topic: "chat.events", ID: 0}}}},
			},
			brokers:    []string{"broker-a"},
			wantDialed: []string{"broker-a"},
			wantClosed: 1,
		},
		{
			name: "topic missing",
			results: map[string]topicDialResult{
				"broker-a": {conn: &topicConnectionStub{partitions: []segmentkafka.Partition{{Topic: "other.events", ID: 0}}}},
			},
			brokers:    []string{"broker-a"},
			wantErr:    "has no readable partitions",
			wantDialed: []string{"broker-a"},
			wantClosed: 1,
		},
		{
			name: "partition metadata error",
			results: map[string]topicDialResult{
				"broker-a": {conn: &topicConnectionStub{partitions: []segmentkafka.Partition{{Topic: "chat.events", Error: partitionErr}}}},
			},
			brokers:    []string{"broker-a"},
			wantErr:    partitionErr.Error(),
			wantDialed: []string{"broker-a"},
			wantClosed: 1,
		},
		{
			name: "next broker after dial error",
			results: map[string]topicDialResult{
				"broker-a": {err: errors.New("unavailable")},
				"broker-b": {conn: &topicConnectionStub{partitions: []segmentkafka.Partition{{Topic: "chat.events", ID: 1}}}},
			},
			brokers:    []string{"broker-a", "broker-b"},
			wantDialed: []string{"broker-a", "broker-b"},
			wantClosed: 1,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			dialer := &topicDialerStub{results: test.results}
			err := verifyTopicWithDialer(context.Background(), dialer, test.brokers, "chat.events")
			if test.wantErr == "" && err != nil {
				t.Fatalf("verifyTopicWithDialer() error = %v", err)
			}
			if test.wantErr != "" && (err == nil || !strings.Contains(err.Error(), test.wantErr)) {
				t.Fatalf("verifyTopicWithDialer() error = %v, want substring %q", err, test.wantErr)
			}
			if !reflect.DeepEqual(dialer.dialed, test.wantDialed) {
				t.Fatalf("dialed brokers = %#v, want %#v", dialer.dialed, test.wantDialed)
			}
			if got := topicConnectionCloses(test.results); got != test.wantClosed {
				t.Fatalf("closed connections = %d, want %d", got, test.wantClosed)
			}
		})
	}
}

func TestVerifyTopicRejectsIncompleteCredentials(t *testing.T) {
	err := VerifyTopic(context.Background(), []string{"127.0.0.1:9092"}, "chat.events", "only-user", "", SHA512)
	if err == nil || !strings.Contains(err.Error(), "username and password") {
		t.Fatalf("VerifyTopic() error = %v", err)
	}
}

type topicDialResult struct {
	conn *topicConnectionStub
	err  error
}

type topicDialerStub struct {
	results map[string]topicDialResult
	dialed  []string
}

func (d *topicDialerStub) DialContext(_ context.Context, _ string, address string) (topicConnection, error) {
	d.dialed = append(d.dialed, address)
	result, ok := d.results[address]
	if !ok {
		return nil, errors.New("unexpected broker")
	}
	return result.conn, result.err
}

type topicConnectionStub struct {
	partitions  []segmentkafka.Partition
	readErr     error
	deadlineErr error
	closeErr    error
	closed      int
	deadline    time.Time
}

func (c *topicConnectionStub) SetDeadline(deadline time.Time) error {
	c.deadline = deadline
	return c.deadlineErr
}

func (c *topicConnectionStub) ReadPartitions(...string) ([]segmentkafka.Partition, error) {
	return c.partitions, c.readErr
}

func (c *topicConnectionStub) Close() error {
	c.closed++
	return c.closeErr
}

func topicConnectionCloses(results map[string]topicDialResult) int {
	closes := 0
	for _, result := range results {
		if result.conn != nil {
			closes += result.conn.closed
		}
	}
	return closes
}
