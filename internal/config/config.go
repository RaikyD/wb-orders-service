package config

import (
	"os"
)

type Config struct {
	HTTP_PORT string `env:"HTTP_PORT"`
	DB_STRING string `env:"DB_STRING"`
}

func LoadConfig() (*Config, error) {
	cfg := &Config{
		HTTP_PORT: os.Getenv("HTTP_PORT"),
		DB_STRING: os.Getenv("DB_STRING"),
	}

	if cfg.HTTP_PORT == "" {
		cfg.HTTP_PORT = "8080"
	}

	return cfg, nil
}
