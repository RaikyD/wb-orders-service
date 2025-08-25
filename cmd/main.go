package main

import (
	"context"
	"github.com/RaikyD/wb-orders-service/internal/kafka"
	"github.com/RaikyD/wb-orders-service/internal/migrate"
	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/jackc/pgx/v5/pgxpool"
	kfk "github.com/segmentio/kafka-go"
	//"github.com/joho/godotenv"
	"net/http"
	"os"
	"time"

	"github.com/RaikyD/wb-orders-service/internal/application"
	"github.com/RaikyD/wb-orders-service/internal/config"
	"github.com/RaikyD/wb-orders-service/internal/logger"
	"github.com/RaikyD/wb-orders-service/internal/presentation"
	"github.com/RaikyD/wb-orders-service/internal/repository"
)

func main() {
	//_ = godotenv.Load()
	logger.Init()
	cfg, err := config.LoadConfig()
	logger.Info("kafka config", "brokers", cfg.KAFKA_BROKERS, "topic", cfg.KAFKA_TOPIC, "group", cfg.KAFKA_GROUP_ID)

	if err != nil {
		logger.Warn("config load failed", "err", err)
		os.Exit(1)
	}

	// DB pool
	pool, err := pgxpool.New(context.Background(), cfg.DB_STRING)
	if err != nil {
		logger.Warn("pgxpool new failed", "err", err)
		os.Exit(1)
	}
	defer pool.Close()

	if err := migrate.Up(cfg.DB_STRING); err != nil {
		logger.Warn("goose up failed", "err", err)
		os.Exit(1)
	}
	logger.Info("migrations applied")

	if err := pool.Ping(context.Background()); err != nil {
		logger.Warn("db ping failed", "err", err)
		os.Exit(1)
	}
	logger.Info("db connected")

	repo := repository.NewOrderRepository(pool)
	svc := application.NewOrdersService(repo)

	if err := svc.RestoreCache(context.Background(), 1000); err != nil {
		logger.Warn("restore cache failed", "err", err)
	}

	d := &kfk.Dialer{Timeout: 10 * time.Second}
	conn, err := d.DialContext(context.Background(), "tcp", "wb-kafka:9092") // "wb-kafka:9092"
	if err == nil {
		defer conn.Close()
		_ = conn.CreateTopics(kfk.TopicConfig{
			Topic: "orders", NumPartitions: 1, ReplicationFactor: 1,
		})
		_ = conn.CreateTopics(kfk.TopicConfig{
			Topic: "orders.dlq", NumPartitions: 1, ReplicationFactor: 1,
		})
	} else {
		logger.Warn("kafka admin dial failed", "err", err)
	}

	var prod *kafka.Producer
	prod = kafka.NewProducer(cfg.KAFKA_BROKERS, cfg.KAFKA_TOPIC)
	defer prod.Close()

	_, _ = kafka.StartConsumer(
		context.Background(),
		svc,
		kafka.ConsumerConfig{
			Brokers: cfg.KAFKA_BROKERS,
			Topic:   cfg.KAFKA_TOPIC,
			GroupID: cfg.KAFKA_GROUP_ID,
		},
	)

	r := chi.NewRouter()
	r.Use(middleware.RequestID)
	r.Use(middleware.RealIP)
	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)
	r.Use(middleware.Timeout(60 * time.Second))

	h := presentation.NewOrdersHandler(*svc, prod)
	h.Register(r)

	presentation.MountStatic(r)

	addr := ":" + cfg.HTTP_PORT
	logger.Info("starting http", "addr", addr)
	if err := http.ListenAndServe(addr, r); err != nil {
		logger.Warn("http server crashed", "err", err)
		os.Exit(1)
	}
}
