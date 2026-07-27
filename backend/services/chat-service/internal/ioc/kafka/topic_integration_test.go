//go:build integration

package kafka

import (
	"context"
	"os"
	"strings"
	"testing"
)

func TestKafkaTopicExists(t *testing.T) {
	brokers := splitKafkaBrokers(os.Getenv("BBS_CHAT_TEST_KAFKA_BROKERS"))
	topic := strings.TrimSpace(os.Getenv("BBS_CHAT_TEST_KAFKA_TOPIC"))
	if len(brokers) == 0 || topic == "" {
		t.Skip("set BBS_CHAT_TEST_KAFKA_BROKERS and BBS_CHAT_TEST_KAFKA_TOPIC to run the Kafka topic check")
	}
	username := strings.TrimSpace(os.Getenv("BBS_CHAT_TEST_KAFKA_USERNAME"))
	password := strings.TrimSpace(os.Getenv("BBS_CHAT_TEST_KAFKA_PASSWORD"))
	algorithm := ScramAlgorithm(strings.ToUpper(strings.TrimSpace(os.Getenv("BBS_CHAT_TEST_KAFKA_SCRAM_ALGORITHM"))))
	if algorithm == "" {
		algorithm = SHA512
	}

	ctx, cancel := context.WithTimeout(context.Background(), topicVerificationTimeout)
	defer cancel()
	if err := VerifyTopic(ctx, brokers, topic, username, password, algorithm); err != nil {
		t.Fatalf("VerifyTopic(%q) error = %v", topic, err)
	}
}

func splitKafkaBrokers(value string) []string {
	parts := strings.Split(value, ",")
	brokers := make([]string, 0, len(parts))
	for _, part := range parts {
		if broker := strings.TrimSpace(part); broker != "" {
			brokers = append(brokers, broker)
		}
	}
	return brokers
}
