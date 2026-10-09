package main

import (
	"eCommerce/pkg/helpers"
)

func main() {
	var forever chan struct{}

	sales, err := CreateSalesService()
	helpers.FailOnError(err, "Failed to create DeliveryService")
	defer sales.CloseRMQChannel()
	sales.generateSales()
	<-forever
}
