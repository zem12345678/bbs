package kafka

import (
	"strings"
	"time"

	"credit-service/pkg/logger"

	"github.com/pkg/errors"
	"github.com/segmentio/kafka-go"
	"github.com/segmentio/kafka-go/sasl/scram"
	"github.com/spf13/viper"
)

type ProducerOptions struct {
	Brokers        []string       `toml:"brokers" json:"brokers" yaml:"brokers" mapstructure:"brokers" env:"KAFKA_BROKERS"`
	ScramAlgorithm ScramAlgorithm `toml:"scram_algorithm" json:"scram_algorithm" yaml:"scram_algorithm" mapstructure:"scram_algorithm" env:"KAFKA_SCRAM_ALGORITHM"`
	Topic          string         `toml:"topic" json:"topic" yaml:"topic" mapstructure:"topic" env:"KAFKA_TOPIC"`
	UserName       string         `toml:"username" json:"username" yaml:"username" mapstructure:"username" env:"KAFKA_USERNAME"`
	Password       string         `toml:"password" json:"password" yaml:"password" mapstructure:"password" env:"KAFKA_PASSWORD"`
	logger         logger.Logger
}

func NewProducerOptions(v *viper.Viper, l logger.Logger) (*ProducerOptions, error) {
	var (
		err error
		o   = new(ProducerOptions)
	)
	if err = v.UnmarshalKey("kafka.producerOptions", &o); err != nil {
		return nil, errors.Wrap(err, "unmarshal kafka producerOptions option error")
	}
	if len(o.Brokers) == 0 {
		o.Brokers = v.GetStringSlice("kafka.brokers")
	}
	if len(o.Brokers) == 0 {
		o.Brokers = []string{"127.0.0.1:9092"}
	}
	if strings.TrimSpace(o.Topic) == "" {
		o.Topic = strings.TrimSpace(v.GetString("kafka.topic"))
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
	l.Info("load kafka producerOptions options success", logger.Any("kafka producerOptions options", o))
	return o, err
}

func NewProducer(o *ProducerOptions) (*kafka.Writer, error) {
	var transport *kafka.Transport
	if strings.TrimSpace(o.UserName) != "" || strings.TrimSpace(o.Password) != "" {
		mechanism, err := scram.Mechanism(scramAlgorithm(o.ScramAlgorithm), o.UserName, o.Password)
		if err != nil {
			return nil, err
		}
		transport = &kafka.Transport{SASL: mechanism}
	}
	w := &kafka.Writer{
		Addr:                   kafka.TCP(o.Brokers...),
		Topic:                  o.Topic,
		Balancer:               &kafka.Hash{},
		RequiredAcks:           kafka.RequireAll,
		MaxAttempts:            3,
		WriteTimeout:           10 * time.Second,
		BatchTimeout:           time.Millisecond,
		AllowAutoTopicCreation: false,
	}
	if transport != nil {
		w.Transport = transport
	}
	return w, nil
}
