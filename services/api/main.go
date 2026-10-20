package main

import (
	"eCommerce/pkg/helpers"

	"github.com/gin-gonic/gin"
)

func main() {
	api, err := CreateMainApiService()
	helpers.FailOnError(err, "Failed to create API")
	defer api.CloseRMQChannel()
	router := gin.Default()
	router.GET("/products", api.GetProducts)
	router.POST("/orders", api.PostOrder)
	router.GET("/orders", api.GetOrders)
	router.GET("/orders/:id", api.GetOrder)
	router.POST("/sales/register", api.PostRegisterEMail)
	router.DELETE("/sales/register", api.DeleteRegisterEMail)

	router.Run(api.Config.ApiURL)
}
