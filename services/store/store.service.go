package main

import (
	"context"
	"crypto"
	crand "crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"eCommerce/pkg/events"
	"eCommerce/pkg/exchanges"
	"eCommerce/pkg/helpers"
	"eCommerce/pkg/interfaces"
	"eCommerce/pkg/mock"
	"eCommerce/pkg/notifications"
	"encoding/json"
	"log"
	"net/http"
	"strconv"
	"time"

	gin "github.com/gin-gonic/gin"
	amqp "github.com/rabbitmq/amqp091-go"
)

type Store struct {
	interfaces.Service
	Products      map[string]mock.Product
	OrderNotifications map[string]notifications.CreateOrderNotification
}

const (
	storePrivateKeyDirectory = "./services/store/store.pem"
)

func CreateStore() (*Store, error) {
	var store *Store = &Store{}
	var err error

	err = store.InitRMQChannel()
	if err != nil {
		helpers.LogErrorMessage(err, "Failed to init RabbitMQ Channel")
		return nil, err
	}

	products, err := mock.GetProducts("store.json")
	if err != nil {
		helpers.LogErrorMessage(err, "Failed to load products Mock")
		return nil, err
	}

	store.initProductsMap(products)

	var ecommerceExchange = exchanges.GetEcommerceExchangeInfo()
	err = store.Channel.ExchangeDeclare(
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
		store.CloseRMQChannel()
		return nil, err
	}

	routingKeys := []string{
		events.OrderCreatedEvent.String(),
		events.OrderDeletedEvent.String(),
	}

	err = store.BindRoutingKeys(routingKeys, ecommerceExchange)
	if err != nil {
		helpers.LogErrorMessage(err, "Failed to declare a exchange")
		store.CloseRMQChannel()
		return nil, err
	}

	store.OrderNotifications = make(map[string]notifications.CreateOrderNotification)

	store.StartConsuming(store.HandleMessages)

	return store, nil
}

func (this *Store) initProductsMap(products []mock.Product) {
	this.Products = make(map[string]mock.Product)
	for _, product := range products {
		this.Products[product.UUID] = product
	}
}

func (this *Store) GetProductsStore(context *gin.Context) {
	var productsList []mock.Product
	for _, product := range this.Products {
		productsList = append(productsList, product)
	}
	context.IndentedJSON(http.StatusOK, productsList)
}

func (this *Store) HandleMessages(message amqp.Delivery) {
	producerKey, err := helpers.GetProducerPublicKey(message)
	if err != nil {
		log.Printf("Failed to read the producer's public key routing key '%v': %v", message.RoutingKey, err)
		return
	}
	err = helpers.VerifyMessage(message, producerKey)
	if err != nil {
		log.Printf("Failed to validate the producer's public key of the event '%v': %v", message.RoutingKey, err)
		return
	}

	switch message.RoutingKey {
	case events.OrderCreatedEvent.String():
		this.handleOrderCreated(message)
	case events.OrderDeletedEvent.String():
		this.handleOrderDeleted(message)
	default:
		log.Fatalf("Unknown routing key '%v'\n", message.RoutingKey)
	}
}

func (this *Store) handleOrderCreated(message amqp.Delivery) {
	notification := notifications.CreateOrderNotification{}
	err := json.Unmarshal(message.Body, &notification)
	if err != nil {
		log.Fatalf("Failed to decode notification '%v'\n", message.Body)
	}

	log.Printf("Received an order of:\n")
	for _, product := range notification.Products {
		log.Printf("- %v %v\n", product.Quantity, strconv.Quote(this.Products[product.ProductId].Name))
	}

	var outOfStock bool = false

	for _, productOrdered := range notification.Products {
		storeProduct, ok := this.Products[productOrdered.ProductId]
		if !ok {
			log.Fatalf("invalid product id '%v'\n", productOrdered.ProductId)
			return
		}
		p := this.Products[productOrdered.ProductId]
		p.Quantity -= productOrdered.Quantity
		this.Products[productOrdered.ProductId] = p
		if storeProduct.Quantity < productOrdered.Quantity {
			outOfStock = true
		}
	}

	this.OrderNotifications[notification.Id] = notification

	if outOfStock {
		this.sendNotification(
			notifications.StoreAvailability{OrderId: notification.Id},
			events.StoreUnavailableEvent,
		)
		return
	}
	this.sendNotification(
		notifications.StoreAvailability{OrderId: notification.Id},
		events.OrderStoreOkEvent,
	)
}

func (this *Store) handleOrderDeleted(message amqp.Delivery) {
	notification := notifications.DeleteOrderNotification{}
	err := json.Unmarshal(message.Body, &notification)
	if err != nil {
		log.Fatalf("Failed to decode notification '%v'\n", message.Body)
	}

	log.Printf("Received an delete notification for the order '%v'\n", notification.OrderId)

	order, ok := this.OrderNotifications[notification.OrderId]
	if !ok {
		log.Fatalf("invalid order id '%v'\n", order.Id)
		return
	}

	for _, product := range order.Products {
		p := this.Products[product.ProductId]
		p.Quantity += product.Quantity
	}

	delete(this.OrderNotifications, notification.OrderId)
}

func (this *Store) sendNotification(object any, event interfaces.Event) {
	notification, err := json.Marshal(object)
	if err != nil {
		log.Printf("failed to serialize notification: %v\n", err)
		return
	}

	privateKey, err := helpers.ReadPrivateKeyPEM(storePrivateKeyDirectory)
	if err != nil {
		log.Fatalf("Failed to read private key: %v", err)
	}

	checksum := sha256.Sum256(notification)

	signature, err := rsa.SignPSS(crand.Reader, privateKey, crypto.SHA256, checksum[:], nil)
	if err != nil {
		log.Fatalf("Failed to sign payload: %v", err)
	}
	this.SendToECommerceExchange(
		amqp.Publishing{
			ContentType: "application/json",
			Headers:     amqp.Table{"x-signature": signature},
			Body:        notification,
		},
		event,
	)
}

func (this *Store) SendToECommerceExchange(publishing amqp.Publishing, routingKey interfaces.Event) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	var err = this.Channel.PublishWithContext(ctx,
		exchanges.GetEcommerceExchangeInfo().Name, // exchange
		routingKey.String(),                       // routing key
		false,                                     // mandatory
		false,                                     // immediate
		publishing,
	)
	helpers.FailOnError(err, "Failed to publish a message")
	log.Printf(" [x] Sent %s\n", publishing.Body)
}
