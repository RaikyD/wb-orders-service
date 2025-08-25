package kafka

import (
	"context"
	"encoding/json"
	"github.com/RaikyD/wb-orders-service/internal/application"
	"github.com/RaikyD/wb-orders-service/internal/domain"
	"github.com/RaikyD/wb-orders-service/internal/logger"
	"github.com/segmentio/kafka-go"
	"strings"
	"time"
)

type ConsumerConfig struct {
	Brokers string
	Topic   string
	GroupID string
}

func StartConsumer(ctx context.Context, svc *application.OrdersService, cfg ConsumerConfig) (*kafka.Reader, error) {
	brokers := strings.Split(cfg.Brokers, ",")

	r := kafka.NewReader(kafka.ReaderConfig{
		Brokers:         brokers,
		GroupID:         cfg.GroupID,
		Topic:           cfg.Topic,
		MinBytes:        1,
		MaxBytes:        10e6,
		CommitInterval:  0,
		StartOffset:     kafka.FirstOffset,
		ReadLagInterval: -1,
	})

	go func() {
		defer r.Close()

		backoff := time.Millisecond * 300
		for {
			m, err := r.FetchMessage(ctx)
			if err != nil {
				if ctx.Err() != nil {
					return
				}
				logger.Info("kafka fetch error")
				time.Sleep(backoff)
				continue
			}

			var o domain.Order
			if err = json.Unmarshal(m.Value, &o); err != nil {
				logger.Info("kafka invalid json. skip and commit")

				_ = r.CommitMessages(ctx, m)
				continue
			}

			if err = svc.AddOrder(ctx, &o); err != nil {
				logger.Warn("kafka add order fail, will retry")
				time.Sleep(backoff)
				continue
			}

			if err := r.CommitMessages(ctx, m); err != nil {
				logger.Info("[kafka] commit failed:", err)
			} else {
				logger.Info("[kafka] committed: topic=%s partition=%d offset=%d uid=%s\n",
					m.Topic, m.Partition, m.Offset, o.OrderUID)
			}
		}

	}()
	return r, nil
}
