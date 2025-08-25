package config

import (
	"os"
)

type Config struct {
	HTTP_PORT      string
	DB_STRING      string
	KAFKA_BROKERS  string // "localhost:9092,localhost:9093"
	KAFKA_TOPIC    string // "orders"
	KAFKA_GROUP_ID string // "orders-service"
	KAFKA_DLT      string // "orders.dlq" (опционально)
}

func LoadConfig() (*Config, error) {
	cfg := &Config{
		HTTP_PORT:      os.Getenv("HTTP_PORT"),
		DB_STRING:      os.Getenv("DB_STRING"),
		KAFKA_BROKERS:  os.Getenv("KAFKA_BROKERS"),
		KAFKA_TOPIC:    os.Getenv("KAFKA_TOPIC"),
		KAFKA_GROUP_ID: os.Getenv("KAFKA_GROUP_ID"),
		KAFKA_DLT:      os.Getenv("KAFKA_DLT"),
	}

	// дефолты на случай, если .env пустой
	if cfg.HTTP_PORT == "" {
		cfg.HTTP_PORT = "8080"
	}
	if cfg.KAFKA_BROKERS == "" {
		cfg.KAFKA_BROKERS = "localhost:9092"
	}
	if cfg.KAFKA_TOPIC == "" {
		cfg.KAFKA_TOPIC = "orders"
	}
	if cfg.KAFKA_GROUP_ID == "" {
		cfg.KAFKA_GROUP_ID = "orders-service"
	}
	if cfg.KAFKA_DLT == "" {
		cfg.KAFKA_DLT = "orders.dlq"
	}
	return cfg, nil
}
