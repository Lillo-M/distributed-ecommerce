package config

import (
	"fmt"
	"os"

	"github.com/joho/godotenv"
)

type Config struct {
	RabbitMQURL    string
	StoreURL       string
	ApiURL         string
	EMailApiKey    string
	PaymentURL     string
	PaymentMockURL string
}

func Load() (*Config, error) {
	_ = godotenv.Load()

	cfg := &Config{
		RabbitMQURL:    os.Getenv("RABBITMQ_URL"),
		StoreURL:       os.Getenv("STORE_URL"),
		ApiURL:         os.Getenv("API_URL"),
		EMailApiKey:    os.Getenv("EMAIL_API_KEY"),
		PaymentURL:     os.Getenv("PAYMENT_URL"),
		PaymentMockURL: os.Getenv("PAYMENT_MOCK_URL"),
	}

	if cfg.RabbitMQURL == "" {
		return nil, fmt.Errorf("RABBITMQ_URL is required")
	}

	if cfg.StoreURL == "" {
		return nil, fmt.Errorf("STORE_URL is required")
	}

	if cfg.ApiURL == "" {
		return nil, fmt.Errorf("API_URL is required")
	}
	if cfg.PaymentURL == "" {
		return nil, fmt.Errorf("PAYMENT_URL is required")
	}
	if cfg.PaymentMockURL == "" {
		return nil, fmt.Errorf("PAYMENT_MOCK_URL is required")
	}

	if cfg.EMailApiKey == "" {
		return nil, fmt.Errorf("EMAIL_API_KEY is required")
	}

	return cfg, nil
}

func getEnv(key, defaultValue string) string {
	if val := os.Getenv(key); val != "" {
		return val
	}
	return defaultValue
}
