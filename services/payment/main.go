package main

import (
	"eCommerce/pkg/helpers"
)

func main() {
	var forever chan struct{}

	payment, err := CreatePaymentService()
	helpers.FailOnError(err, "Failed to create PaymentService")
	defer payment.CloseRMQChannel()
	<-forever
}
