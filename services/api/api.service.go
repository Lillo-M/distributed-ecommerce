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
	"sync"
	"uuid"

	"github.com/gin-gonic/gin"
)

type MainApiService struct {
	interfaces.Service
	httpClient *http.Client
	Orders     map[string]mock.OrderInfo
	Products   []mock.Product
	// protege Orders: acessado pelos handlers do Gin e pela goroutine consumidora do RabbitMQ
	ordersMutex sync.RWMutex
	// clientes SSE conectados (ver api.sse.go)
	sseClients map[chan mock.OrderInfo]struct{}
	sseMutex   sync.Mutex
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

	// a API publica o interesse em promoções na exchange sales; publicar numa exchange
	// inexistente (MS Promoções ainda não subiu) faz o RabbitMQ fechar o canal da API
	var salesExchange = exchanges.GetSalesExchangeInfo()
	err = mainApi.Channel.ExchangeDeclare(
		salesExchange.Name, // name
		salesExchange.Type, // type
		false,              // durability
		false,              // auto-deleted
		false,              // internal
		false,              // no-wait
		nil,                // arguments
	)

	if err != nil {
		helpers.LogErrorMessage(err, "Failed to declare a exchange")
		mainApi.CloseRMQChannel()
		return nil, err
	}

	routingKeys := []string{
		events.PaymentEventApproved.String(),
		events.PaymentEventRefused.String(),
		events.OrderSentEvent.String(),
		events.OrderStoreOkEvent.String(),
		events.StoreUnavailableEvent.String(),
		events.PaymentEventCheckoutCreated.String(),
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

	mainApi.Orders = make(map[string]mock.OrderInfo)

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

func (this *MainApiService) GetOrders(context *gin.Context) {
	var orderList []mock.OrderInfo
	this.ordersMutex.RLock()
	for _, order := range this.Orders {
		orderList = append(orderList, order)
	}
	this.ordersMutex.RUnlock()
	context.IndentedJSON(http.StatusOK, orderList)
}

func (this *MainApiService) GetOrder(context *gin.Context) {
	this.ordersMutex.RLock()
	order, ok := this.Orders[context.Param("id")]
	this.ordersMutex.RUnlock()
	if !ok {
		context.IndentedJSON(http.StatusNotFound, gin.H{
			"success": false,
			"message": "order not found",
		})
		return
	}
	context.IndentedJSON(http.StatusOK, order)
}

// (0,1) listar produtos disponíveis em estoque;
func (this *MainApiService) GetProducts(context *gin.Context) {
	productsList, err := this.getStoreProducts()
	if err != nil {
		context.IndentedJSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"message": "oh shi...",
			"error":   err.Error(),
		})
		return
	}

	context.IndentedJSON(http.StatusOK, productsList)
}

func (this *MainApiService) getStoreProducts() ([]mock.Product, error) {
	var productsList []mock.Product
	data, err := this.HttpGet("http://" + this.Config.StoreURL + "/products")
	if err != nil {
		return nil, err
	}

	err = json.Unmarshal(data, &productsList)
	if err != nil {
		return nil, err
	}

	return productsList, nil
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

	if this.Products == nil {
		productsList, err := this.getStoreProducts()
		if err != nil {
			context.IndentedJSON(http.StatusInternalServerError, gin.H{
				"success": false,
				"message": "oh shi...",
				"error":   err.Error(),
			})
			return
		}
		this.Products = productsList
	}

	// 1. Initialize a slice to hold the combined products
	orderProducts := make([]mock.Product, 0, len(request.Products))

	// 2. Loop through the incoming DTO items
	for _, dtoProduct := range request.Products {

		// 3. Find the matching original product in your catalog/list
		for _, originalProduct := range this.Products { // Replace with your actual products list variable
			if originalProduct.UUID == dtoProduct.ProductId {

				// 4. Create the new instance combining DTO (Quantity) + Original (Name, Price, Id)
				orderProducts = append(orderProducts, mock.Product{
					UUID:     originalProduct.UUID,
					Name:     originalProduct.Name,
					Price:    originalProduct.Price,
					Quantity: dtoProduct.Quantity,
				})

				break // Found the match, stop searching the catalog for this item
			}
		}
	}

	// 5. Assign the finalized slice to your OrderInfo
	order := mock.OrderInfo{
		UUID:     notification.Id,
		Products: orderProducts,
		Status:   mock.OrderStatusCreated,
	}

	this.ordersMutex.Lock()
	this.Orders[order.UUID] = order
	this.ordersMutex.Unlock()
	this.broadcastOrder(order)

	this.publishEvent(notificationBytes, events.OrderCreatedEvent)

	// retorna o pedido criado (com o UUID) para o frontend acompanhar o status
	context.IndentedJSON(http.StatusCreated, order)
}

// (0,1) registrar interesse, informando o e-mail, em receber notificação sobre promoções de categorias;
func (this *MainApiService) PostRegisterEMail(context *gin.Context) {
	var request requests.TrackSalesRequest

	if err := context.BindJSON(&request); err != nil {
		context.IndentedJSON(http.StatusBadRequest, request)
		return
	}

	notification := notifications.TrackSaleNotification{
		IsRegisting: true,
		Category:    request.Category,
		Email:       request.Email,
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

	this.publishSaleEvent(notificationBytes, events.SaleInterest)

	context.IndentedJSON(http.StatusCreated, gin.H{})
}

// (0,1) cancelar interesse em receber e-mail sobre.
func (this *MainApiService) DeleteRegisterEMail(context *gin.Context) {
	var request requests.TrackSalesRequest

	if err := context.BindJSON(&request); err != nil {
		context.IndentedJSON(http.StatusBadRequest, request)
		return
	}

	notification := notifications.TrackSaleNotification{
		IsRegisting: false,
		Category:    request.Category,
		Email:       request.Email,
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

	this.publishSaleEvent(notificationBytes, events.SaleInterest)

	context.IndentedJSON(http.StatusOK, gin.H{})
}
