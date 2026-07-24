//go:build integration

package messaging

import (
	"context"
	"encoding/json"
	"io"
	"os"
	"strings"
	"testing"
	"time"

	domain "chat-service/internal/domain/chat"
	iockafka "chat-service/internal/ioc/kafka"

	"github.com/google/uuid"
	"github.com/segmentio/kafka-go"
)

func TestKafkaOutboxPublisherDeliversEnvelopeToBroker(t *testing.T) {
	brokers := kafkaTestBrokers(os.Getenv("BBS_CHAT_TEST_KAFKA_BROKERS"))
	topic := strings.TrimSpace(os.Getenv("BBS_CHAT_TEST_KAFKA_TOPIC"))
	if len(brokers) == 0 || topic == "" {
		t.Skip("set BBS_CHAT_TEST_KAFKA_BROKERS and BBS_CHAT_TEST_KAFKA_TOPIC to run the Kafka integration test")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	readers, err := openKafkaTestReaders(ctx, brokers[0], topic)
	if err != nil {
		t.Fatalf("open Kafka topic readers: %v", err)
	}
	for _, reader := range readers {
		defer reader.Close()
	}

	writer, err := iockafka.NewProducer(&iockafka.ProducerOptions{Brokers: brokers, Topic: topic})
	if err != nil {
		t.Fatalf("create Kafka producer: %v", err)
	}
	publisher := NewKafkaOutboxPublisher(writer)
	defer publisher.Close()

	eventID := "chat-kafka-integration-" + uuid.NewString()
	payload, err := json.Marshal(map[string]any{
		"eventId": eventID, "eventType": "chat.test.delivery.v1", "version": 1,
		"payload": map[string]any{"roomId": 1},
	})
	if err != nil {
		t.Fatalf("marshal test event: %v", err)
	}
	event := domain.OutboxEvent{
		EventID: eventID, EventType: "chat.test.delivery.v1", EventVersion: 1,
		PartitionKey: "chat-kafka-integration", Payload: payload,
	}
	if err := publisher.PublishOutboxEvent(ctx, event); err != nil {
		t.Fatalf("publish Kafka outbox event: %v", err)
	}

	results := make(chan kafkaTestReadResult, len(readers))
	deadline := time.Now().Add(10 * time.Second)
	for _, reader := range readers {
		go readKafkaTestEvent(reader, deadline, eventID, results)
	}
	var message kafka.Message
	for remaining := len(readers); remaining > 0; remaining-- {
		result := <-results
		if result.err != nil || !isKafkaTestEvent(result.message.Value, eventID) {
			continue
		}
		message = result.message
		break
	}
	if !isKafkaTestEvent(message.Value, eventID) {
		t.Fatalf("did not receive Kafka outbox event %q", eventID)
	}
	if string(message.Key) != event.PartitionKey {
		t.Fatalf("Kafka key = %q, want %q", message.Key, event.PartitionKey)
	}
	if string(message.Value) != string(event.Payload) {
		t.Fatalf("Kafka payload = %s, want %s", message.Value, event.Payload)
	}
	headers := make(map[string]string, len(message.Headers))
	for _, header := range message.Headers {
		headers[header.Key] = string(header.Value)
	}
	if headers["event_id"] != event.EventID || headers["event_type"] != event.EventType || headers["event_version"] != "1" || headers["producer"] != "chat-service" {
		t.Fatalf("Kafka headers = %#v", headers)
	}
}

type kafkaTestReadResult struct {
	message kafka.Message
	err     error
}

func openKafkaTestReaders(ctx context.Context, broker, topic string) ([]*kafka.Conn, error) {
	metadata, err := kafka.DialContext(ctx, "tcp", broker)
	if err != nil {
		return nil, err
	}
	partitions, err := metadata.ReadPartitions(topic)
	closeErr := metadata.Close()
	if err != nil {
		return nil, err
	}
	if closeErr != nil {
		return nil, closeErr
	}
	readers := make([]*kafka.Conn, 0, len(partitions))
	for _, partition := range partitions {
		if partition.Topic != topic || partition.Error != nil {
			continue
		}
		reader, err := kafka.DialLeader(ctx, "tcp", broker, topic, partition.ID)
		if err != nil {
			for _, opened := range readers {
				_ = opened.Close()
			}
			return nil, err
		}
		if _, err := reader.Seek(0, kafka.SeekEnd); err != nil {
			_ = reader.Close()
			for _, opened := range readers {
				_ = opened.Close()
			}
			return nil, err
		}
		readers = append(readers, reader)
	}
	if len(readers) == 0 {
		return nil, kafka.UnknownTopicOrPartition
	}
	return readers, nil
}

func readKafkaTestEvent(reader *kafka.Conn, deadline time.Time, eventID string, results chan<- kafkaTestReadResult) {
	for {
		if err := reader.SetReadDeadline(deadline); err != nil {
			results <- kafkaTestReadResult{err: err}
			return
		}
		batch := reader.ReadBatch(1, 1<<20)
		for {
			message, err := batch.ReadMessage()
			if err == nil {
				if isKafkaTestEvent(message.Value, eventID) {
					_ = batch.Close()
					results <- kafkaTestReadResult{message: message}
					return
				}
				continue
			}
			closeErr := batch.Close()
			if err == io.EOF && closeErr == nil {
				break
			}
			if err == io.EOF {
				err = closeErr
			}
			results <- kafkaTestReadResult{err: err}
			return
		}
	}
}

func isKafkaTestEvent(payload []byte, eventID string) bool {
	var envelope struct {
		EventID string `json:"eventId"`
	}
	return json.Unmarshal(payload, &envelope) == nil && envelope.EventID == eventID
}

func kafkaTestBrokers(value string) []string {
	parts := strings.Split(value, ",")
	brokers := make([]string, 0, len(parts))
	for _, part := range parts {
		if broker := strings.TrimSpace(part); broker != "" {
			brokers = append(brokers, broker)
		}
	}
	return brokers
}
