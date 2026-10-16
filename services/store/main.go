package main

import (
	"eCommerce/pkg/helpers"

	gin "github.com/gin-gonic/gin"
)

func main() {
	store, err := CreateStore()
	helpers.FailOnError(err, "Failed to create Store")
	defer store.CloseRMQChannel()
	router := gin.Default()
	router.GET("/products", store.GetProductsStore)

	router.RunTLS("localhost:8080",
		"./services/store/certificate.pem",
		"./services/store/httpPrivate.key")
}
