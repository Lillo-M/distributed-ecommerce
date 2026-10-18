package main

import (
	"eCommerce/pkg/helpers"

	"github.com/gin-gonic/gin"
)

func main() {
	payment, err := CreatePaymentService()
	helpers.FailOnError(err, "Failed to create PaymentService")
	defer payment.CloseRMQChannel()

	router := gin.Default()
	router.POST("/webhook", payment.PostWebhook)

	router.Run(payment.Config.PaymentURL)
}