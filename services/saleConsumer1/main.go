package main

import (
	"eCommerce/pkg/helpers"
)

func main() {
	var forever chan struct{}

	consumer, err := CreateConsumerService()
	helpers.FailOnError(err, "Failed to create ConsumerService")
	defer consumer.CloseRMQChannel()
	<-forever
}
