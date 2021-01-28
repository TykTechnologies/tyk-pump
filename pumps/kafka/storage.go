package kafka

import (
	"context"

	"github.com/segmentio/kafka-go"
)

type KafkaClient interface {
	WriteMessages(ctx context.Context, msgs ...kafka.Message) error
}