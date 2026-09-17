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
