package config

import (
	"fmt"
	"os"

	"github.com/joho/godotenv"
)

type Config struct {
	RabbitMQURL string
}

func Load() (*Config, error) {
	_ = godotenv.Load()

	cfg := &Config{
		RabbitMQURL: os.Getenv("RABBITMQ_URL"),
	}

	if cfg.RabbitMQURL == "" {
		return nil, fmt.Errorf("RABBITMQ_URL is required")
	}

	return cfg, nil
}

func getEnv(key, defaultValue string) string {
	if val := os.Getenv(key); val != "" {
		return val
	}
	return defaultValue
}

