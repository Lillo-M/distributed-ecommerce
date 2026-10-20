package main

import (
	"eCommerce/pkg/helpers"
	"log"

	"github.com/gin-gonic/gin"
)

func main() {
	paymentMock, err := CreatePaymentMockService()
	helpers.FailOnError(err, "Failed to create PaymentMockService")

	router := gin.Default()
	router.POST("/charges", paymentMock.PostCharge)
	router.GET("/checkout/:orderId", paymentMock.GetCheckout)

	router.POST("/checkout/:orderId/approve", paymentMock.PostApprove)
	router.POST("/checkout/:orderId/refuse", paymentMock.PostRefuse)

	log.Printf("Mock de Pagamento tá ouvindo em %s", paymentMock.ListenURL)
	router.Run(paymentMock.ListenURL)
}