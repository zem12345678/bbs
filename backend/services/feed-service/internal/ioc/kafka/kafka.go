package kafka

import (
	"strings"

	"github.com/google/wire"
	"github.com/segmentio/kafka-go/sasl/scram"
)

type ScramAlgorithm string

const (
	SHA256 ScramAlgorithm = "SHA256"
	SHA512 ScramAlgorithm = "SHA512"
)

func scramAlgorithm(scramAlgorithm ScramAlgorithm) scram.Algorithm {
	switch ScramAlgorithm(strings.ToUpper(strings.TrimSpace(string(scramAlgorithm)))) {
	case SHA256:
		return scram.SHA256
	default:
		return scram.SHA512
	}
}

var ProviderSet = wire.NewSet(NewProducer, NewProducerOptions, NewConsumer, NewConsumerOptions)
