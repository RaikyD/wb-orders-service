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

	logger.Info("kafka consumer starting", "brokers", cfg.Brokers, "topic", cfg.Topic, "group", cfg.GroupID)

	go func() {
		defer r.Close()

		backoff := time.Millisecond * 300
		for {
			//m, err := r.FetchMessage(ctx)
			//if err != nil {
			//	if ctx.Err() != nil {
			//		return
			//	}
			//	logger.Warn("kafka fetch error", "err", err)
			//	time.Sleep(backoff)
			//	continue
			//}
			//logger.Info("order fetched", "partition", m.Partition, "offset", m.Offset)
			//var o domain.Order
			//if err = json.Unmarshal(m.Value, &o); err != nil {
			//	logger.Info("kafka invalid json. skip and commit")
			//
			//	_ = r.CommitMessages(ctx, m)
			//	continue
			//}
			//logger.Info("order structure is correct || passed Unmarshal check")
			//if err = svc.AddOrder(ctx, &o); err != nil {
			//	logger.Warn("kafka add order fail, will retry")
			//	time.Sleep(backoff)
			//	continue
			//}
			//logger.Info("Order successfully added", "uid", o.OrderUID)
			//
			//if err := r.CommitMessages(ctx, m); err != nil {
			//	logger.Warn("[kafka] commit failed", "err", err)
			//} else {
			//	logger.Info("[kafka] committed",
			//		"topic", m.Topic, "partition", m.Partition, "offset", m.Offset, "uid", o.OrderUID)
			//}
			m, err := r.FetchMessage(ctx)
			if err != nil {
				if ctx.Err() != nil {
					return
				}
				logger.Warn("kafka fetch error", "err", err) // было без err
				time.Sleep(backoff)
				continue
			}
			logger.Info("order fetched", "partition", m.Partition, "offset", m.Offset)

			var o domain.Order
			if err = json.Unmarshal(m.Value, &o); err != nil {
				logger.Warn("kafka invalid json. skip and commit", "err", err)
				_ = r.CommitMessages(ctx, m)
				continue
			}

			if err = svc.AddOrder(ctx, &o); err != nil {
				logger.Warn("kafka add order fail, will retry", "err", err)
				time.Sleep(backoff)
				continue
			}

			logger.Info("Order successfully added", "uid", o.OrderUID)

			if err := r.CommitMessages(ctx, m); err != nil {
				logger.Warn("[kafka] commit failed", "err", err)
			} else {
				logger.Info("[kafka] committed", "topic", m.Topic, "partition", m.Partition, "offset", m.Offset, "uid", o.OrderUID)
			}

		}

	}()
	return r, nil
}
