package main

import (
	"eCommerce/pkg/events"
	"eCommerce/pkg/exchanges"
	"eCommerce/pkg/helpers"
	"eCommerce/pkg/interfaces"
	"eCommerce/pkg/mock"
	"eCommerce/pkg/notifications"
	"eCommerce/pkg/requests"
	"encoding/json"
	"io"
	"net/http"
	"uuid"

	"github.com/gin-gonic/gin"
)

type MainApiService struct {
	interfaces.Service
	httpClient *http.Client
	Orders     map[string]mock.OrderInfo
}

const (
	ApproveChancePercent = 50
	StoreCertFilePath    = "pkg/certificates/storeCertificate.pem"
)

func CreateMainApiService() (*MainApiService, error) {
	var mainApi *MainApiService = &MainApiService{}
	var err error

	err = mainApi.InitRMQChannel()
	if err != nil {
		helpers.LogErrorMessage(err, "Failed to init RabbitMQ Channel")
		return nil, err
	}

	var ecommerceExchange = exchanges.GetEcommerceExchangeInfo()
	err = mainApi.Channel.ExchangeDeclare(
		ecommerceExchange.Name, // name
		ecommerceExchange.Type, // type
		false,                  // durability
		false,                  // auto-deleted
		false,                  // internal
		false,                  // no-wait
		nil,                    // arguments
	)

	if err != nil {
		helpers.LogErrorMessage(err, "Failed to declare a exchange")
		mainApi.CloseRMQChannel()
		return nil, err
	}

	routingKeys := []string{
		events.PaymentEventApproved.String(),
	}

	err = mainApi.BindRoutingKeys(routingKeys, ecommerceExchange)
	if err != nil {
		helpers.LogErrorMessage(err, "Failed to declare a exchange")
		mainApi.CloseRMQChannel()
		return nil, err
	}

	mainApi.httpClient, err = CreateHttpClient()
	if err != nil {
		helpers.LogErrorMessage(err, "Failed to create a http client")
		mainApi.CloseRMQChannel()
		return nil, err
	}

	mainApi.StartConsuming(mainApi.HandleMessages)

	return mainApi, nil
}

func CreateHttpClient() (*http.Client, error) {
	client := &http.Client{}
	return client, nil
}

func (this *MainApiService) HttpGet(path string) ([]byte, error) {
	resp, err := this.httpClient.Get(path)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	return body, err
}

// (0,1) listar produtos disponíveis em estoque;
func (this *MainApiService) GetProducts(context *gin.Context) {
	var productsList []mock.Product
	data, err := this.HttpGet("http://" + this.Config.StoreURL + "/products")
	if err != nil {
		context.IndentedJSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"message": "oh shi...",
			"error":   err.Error(),
		})
		return
	}

	err = json.Unmarshal(data, &productsList)
	if err != nil {
		context.IndentedJSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"message": "oh fu...",
			"error":   err.Error(),
		})
		return
	}

	context.IndentedJSON(http.StatusOK, productsList)
}

// (0,1) criar pedidos;

func (this *MainApiService) PostOrder(context *gin.Context) {
	var request requests.CreateOrderRequest

	if err := context.BindJSON(&request); err != nil {
		context.IndentedJSON(http.StatusBadRequest, request)
		return
	}

	notification := notifications.CreateOrderNotification{
		Id:       uuid.New().String(),
		Products: request.Products,
	}

	notificationBytes, err := json.Marshal(notification)
	if err != nil {
		context.IndentedJSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"message": "couldn't serialize notification...",
			"error":   err.Error(),
		})
		return
	}

	// TODO: hold orders state

	this.publishEvent(notificationBytes, events.OrderCreatedEvent)

	context.IndentedJSON(http.StatusCreated, gin.H{})
}

// (0,1) registrar interesse, informando o e-mail, em receber notificação sobre promoções de categorias;

// (0,1) cancelar interesse em receber e-mail sobre.
