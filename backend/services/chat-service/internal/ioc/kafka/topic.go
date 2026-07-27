package kafka

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/segmentio/kafka-go"
	"github.com/segmentio/kafka-go/sasl/scram"
)

const topicVerificationTimeout = 10 * time.Second

type topicConnection interface {
	SetDeadline(time.Time) error
	ReadPartitions(...string) ([]kafka.Partition, error)
	Close() error
}

type topicDialer interface {
	DialContext(context.Context, string, string) (topicConnection, error)
}

type kafkaTopicDialer struct {
	dialer *kafka.Dialer
}

func (d kafkaTopicDialer) DialContext(ctx context.Context, network, address string) (topicConnection, error) {
	return d.dialer.DialContext(ctx, network, address)
}

// VerifyTopic confirms that a Kafka topic has at least one readable partition.
// Topic creation remains a deployment responsibility.
func VerifyTopic(ctx context.Context, brokers []string, topic, username, password string, algorithm ScramAlgorithm) error {
	if ctx == nil {
		return errors.New("kafka topic verification context is required")
	}
	topic = strings.TrimSpace(topic)
	if topic == "" {
		return errors.New("kafka topic is required")
	}

	validBrokers := make([]string, 0, len(brokers))
	for _, broker := range brokers {
		if broker = strings.TrimSpace(broker); broker != "" {
			validBrokers = append(validBrokers, broker)
		}
	}
	if len(validBrokers) == 0 {
		return errors.New("kafka brokers are required")
	}
	if (strings.TrimSpace(username) == "") != (strings.TrimSpace(password) == "") {
		return errors.New("kafka username and password must be configured together")
	}

	dialer := &kafka.Dialer{Timeout: topicVerificationTimeout, DualStack: true}
	if strings.TrimSpace(username) != "" {
		mechanism, err := scram.Mechanism(scramAlgorithm(algorithm), username, password)
		if err != nil {
			return err
		}
		dialer.SASLMechanism = mechanism
	}

	verifyCtx, cancel := context.WithTimeout(ctx, topicVerificationTimeout)
	defer cancel()
	return verifyTopicWithDialer(verifyCtx, kafkaTopicDialer{dialer: dialer}, validBrokers, topic)
}

func (o *ProducerOptions) VerifyTopic(ctx context.Context) error {
	if o == nil {
		return errors.New("kafka producer options are required")
	}
	return VerifyTopic(ctx, o.Brokers, o.Topic, o.UserName, o.Password, o.ScramAlgorithm)
}

func (o *ConsumerOptions) VerifyTopic(ctx context.Context, topic string) error {
	if o == nil {
		return errors.New("kafka consumer options are required")
	}
	return VerifyTopic(ctx, o.Brokers, topic, o.UserName, o.Password, o.ScramAlgorithm)
}

func verifyTopicWithDialer(ctx context.Context, dialer topicDialer, brokers []string, topic string) error {
	if dialer == nil {
		return errors.New("kafka topic dialer is required")
	}
	deadline, ok := ctx.Deadline()
	if !ok {
		deadline = time.Now().Add(topicVerificationTimeout)
	}

	failures := make([]error, 0, len(brokers))
	for _, broker := range brokers {
		conn, err := dialer.DialContext(ctx, "tcp", broker)
		if err != nil {
			failures = append(failures, fmt.Errorf("dial broker %s: %w", broker, err))
			continue
		}

		if err := conn.SetDeadline(deadline); err != nil {
			_ = conn.Close()
			failures = append(failures, fmt.Errorf("set broker %s deadline: %w", broker, err))
			continue
		}
		partitions, readErr := conn.ReadPartitions(topic)
		closeErr := conn.Close()
		if readErr != nil {
			failures = append(failures, fmt.Errorf("read topic %s from broker %s: %w", topic, broker, readErr))
			continue
		}
		if closeErr != nil {
			failures = append(failures, fmt.Errorf("close broker %s connection: %w", broker, closeErr))
			continue
		}
		for _, partition := range partitions {
			if partition.Topic == topic && partition.Error == nil {
				return nil
			}
			if partition.Topic == topic && partition.Error != nil {
				failures = append(failures, fmt.Errorf("read topic %s from broker %s: %w", topic, broker, partition.Error))
			}
		}
		failures = append(failures, fmt.Errorf("topic %s has no readable partitions on broker %s", topic, broker))
	}

	return fmt.Errorf("verify kafka topic %s: %w", topic, errors.Join(failures...))
}
