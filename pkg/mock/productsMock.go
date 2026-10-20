package mock

import (
	"encoding/json"
	"fmt"
	"os"
)

type Product struct {
	UUID     string  `json:"uuid"`
	Name     string  `json:"name"`
	Price    float64 `json:"price"`
	Quantity int     `json:"quantity"`
}

func GetProducts(filePath string) ([]Product, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to open file: %w", err)
	}
	defer file.Close()

	var products []Product
	if err := json.NewDecoder(file).Decode(&products); err != nil {
		return nil, fmt.Errorf("failed to decode json: %w", err)
	}

	return products, nil
}

func SaveProducts(filePath string, products []Product) error {
	data, err := json.MarshalIndent(products, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to encode json: %w", err)
	}

	if err := os.WriteFile(filePath, data, 0644); err != nil {
		return fmt.Errorf("failed to write file: %w", err)
	}

	return nil
}
