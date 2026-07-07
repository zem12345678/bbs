package kafka

import (
	"strings"
	"time"

	"api-gateway/pkg/logger"

	"github.com/pkg/errors"
	"github.com/segmentio/kafka-go"
	"github.com/segmentio/kafka-go/sasl/scram"
	"github.com/spf13/viper"
)

type ConsumerOptions struct {
	Brokers        []string       `toml:"brokers" json:"brokers" yaml:"brokers" mapstructure:"brokers" env:"KAFKA_BROKERS"`
	ScramAlgorithm ScramAlgorithm `toml:"scram_algorithm" json:"scram_algorithm" yaml:"scram_algorithm" mapstructure:"scram_algorithm" env:"KAFKA_SCRAM_ALGORITHM"`
	Topics         []string       `toml:"topics" json:"topics" yaml:"topics" mapstructure:"topics" env:"KAFKA_TOPICS"`
	GroupID        string         `toml:"groupId" json:"groupId" yaml:"groupId" mapstructure:"groupId" env:"KAFKA_GROUPID"`
	UserName       string         `toml:"username" json:"username" yaml:"username" mapstructure:"username" env:"KAFKA_USERNAME"`
	Password       string         `toml:"password" json:"password" yaml:"password" mapstructure:"password" env:"KAFKA_PASSWORD"`
	logger         logger.Logger
}

func NewConsumerOptions(v *viper.Viper, l logger.Logger) (*ConsumerOptions, error) {
	var (
		err error
		o   = new(ConsumerOptions)
	)
	if err = v.UnmarshalKey("kafka.consumerOptions", &o); err != nil {
		return nil, errors.Wrap(err, "unmarshal kafka consumerOptions option error")
	}
	if len(o.Brokers) == 0 {
		o.Brokers = v.GetStringSlice("kafka.brokers")
	}
	if len(o.Brokers) == 0 {
		o.Brokers = []string{"127.0.0.1:9092"}
	}
	if len(o.Topics) == 0 {
		o.Topics = v.GetStringSlice("kafka.topics")
	}
	if len(o.Topics) == 0 && strings.TrimSpace(v.GetString("kafka.topic")) != "" {
		o.Topics = []string{strings.TrimSpace(v.GetString("kafka.topic"))}
	}
	if strings.TrimSpace(o.GroupID) == "" {
		o.GroupID = strings.TrimSpace(v.GetString("kafka.groupId"))
	}
	if strings.TrimSpace(o.UserName) == "" {
		o.UserName = strings.TrimSpace(v.GetString("kafka.username"))
	}
	if strings.TrimSpace(o.Password) == "" {
		o.Password = strings.TrimSpace(v.GetString("kafka.password"))
	}
	if o.ScramAlgorithm == "" {
		o.ScramAlgorithm = ScramAlgorithm(strings.TrimSpace(v.GetString("kafka.scram_algorithm")))
	}
	l.Info("load kafka consumerOptions options success", logger.Any("kafka consumerOptions options", o))
	return o, err
}

func NewConsumer(o *ConsumerOptions) (*kafka.Reader, error) {
	dialer := &kafka.Dialer{
		Timeout:   10 * time.Second,
		DualStack: true,
	}
	if strings.TrimSpace(o.UserName) != "" || strings.TrimSpace(o.Password) != "" {
		mechanism, err := scram.Mechanism(scramAlgorithm(o.ScramAlgorithm), o.UserName, o.Password)
		if err != nil {
			return nil, err
		}
		dialer.SASLMechanism = mechanism
	}
	config := kafka.ReaderConfig{
		Brokers:        o.Brokers,
		Dialer:         dialer,
		GroupID:        o.GroupID,
		MinBytes:       1,
		MaxBytes:       10e6,
		StartOffset:    kafka.FirstOffset,
		CommitInterval: 0,
	}
	if len(o.Topics) == 1 {
		config.Topic = o.Topics[0]
	} else {
		config.GroupTopics = o.Topics
	}
	return kafka.NewReader(config), nil
}

func (o *ConsumerOptions) WithTopic(topic, groupID string) *ConsumerOptions {
	cp := *o
	cp.Topics = []string{strings.TrimSpace(topic)}
	cp.GroupID = strings.TrimSpace(groupID)
	return &cp
}
