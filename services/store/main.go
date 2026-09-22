package main

import (
	"eCommerce/pkg/helpers"
)

func main() {
	var forever chan struct{}

	store, err := CreateStore()
	helpers.FailOnError(err, "Failed to create Store")
	defer store.CloseRMQChannel()
	<-forever
}
