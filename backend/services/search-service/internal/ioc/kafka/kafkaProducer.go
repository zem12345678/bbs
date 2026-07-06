package kafka

import (
	"search-service/pkg/logger"

	"github.com/pkg/errors"
	"github.com/segmentio/kafka-go"
	"github.com/segmentio/kafka-go/sasl/scram"
	"github.com/spf13/viper"
)

type ProducerOptions struct {
	Brokers        []string       `toml:"brokers" json:"brokers" yaml:"brokers"  env:"KAFKA_BROKERS"`
	ScramAlgorithm ScramAlgorithm `toml:"scram_algorithm" json:"scram_algorithm" yaml:"scram_algorithm"  env:"KAFKA_SCRAM_ALGORITHM"`
	Topic          string         `toml:"topic" json:"topic" yaml:"topic"  env:"KAFKA_TOPIC"`
	UserName       string         `toml:"username" json:"username" yaml:"username"  env:"KAFKA_USERNAME"`
	Password       string         `toml:"password" json:"password" yaml:"password"  env:"KAFKA_PASSWORD"`
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
	l.Info("load kafka producerOptions options success", logger.Any("kafka producerOptions options", o))
	return o, err
}

func NewProducer(o *ProducerOptions) (*kafka.Writer, error) {
	mechanism, err := scram.Mechanism(scramAlgorithm(o.ScramAlgorithm), o.UserName, o.Password)
	if err != nil {
		return nil, err
	}
	w := &kafka.Writer{
		Addr:                   kafka.TCP(o.Brokers...),
		Topic:                  o.Topic,
		Balancer:               &kafka.LeastBytes{},
		AllowAutoTopicCreation: true,
		Transport:              &kafka.Transport{SASL: mechanism},
	}
	return w, nil
}
