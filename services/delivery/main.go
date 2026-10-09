package main

import (
	"eCommerce/pkg/helpers"
)

func main() {
	var forever chan struct{}

	delivery, err := CreateDeliveryService()
	helpers.FailOnError(err, "Failed to create DeliveryService")
	defer delivery.CloseRMQChannel()
	<-forever
}
