package kafka

import (
	"context"
	"encoding/json"
	"github.com/RaikyD/wb-orders-service/internal/domain"
	"github.com/segmentio/kafka-go"
	"strings"
)

type Producer struct {
	w *kafka.Writer
}

func NewProducer(brokersSTR, topic string) *Producer {
	brokers := strings.Split(brokersSTR, ",")

	return &Producer{
		w: &kafka.Writer{
			Addr:         kafka.TCP(brokers...),
			Topic:        topic,
			Balancer:     &kafka.LeastBytes{},
			RequiredAcks: kafka.RequireAll,
			Async:        false,
		},
	}
}

func (p *Producer) Close() error {
	return p.w.Close()
}

func (p *Producer) PublishOrder(ctx context.Context, o domain.Order) error {
	b, err := json.Marshal(o)
	if err != nil {
		return err
	}

	key := []byte(o.OrderUID)
	return p.w.WriteMessages(ctx, kafka.Message{
		Key:   key,
		Value: b,
		Headers: []kafka.Header{
			{Key: "content-type", Value: []byte("application/json")},
		},
	})
}
