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
	"eCommerce/pkg/requests"
	"encoding/json"
	"log"
	"strconv"
	"time"

	amqp "github.com/rabbitmq/amqp091-go"
)

type Store struct {
	interfaces.Service
	Products      map[string]mock.Product
	OrderRequests map[string]requests.CreateOrderRequest
}

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

	store.OrderRequests = make(map[string]requests.CreateOrderRequest)

	store.StartConsuming(store.HandleMessages)

	return store, nil
}

func (this *Store) initProductsMap(products []mock.Product) {
	this.Products = make(map[string]mock.Product)
	for _, product := range products {
		this.Products[product.UUID] = product
	}
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
	request := requests.CreateOrderRequest{}
	err := json.Unmarshal(message.Body, &request)
	if err != nil {
		log.Fatalf("Failed to decode request '%v'\n", message.Body)
	}

	log.Printf("Received an order of:\n")
	for _, product := range request.Products {
		log.Printf("- %v %v\n", product.Quantity, strconv.Quote(this.Products[product.ProductId].Name))
	}

	var outOfStock bool = false

	for _, productOrdered := range request.Products {
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

	this.OrderRequests[request.Id] = request

	if outOfStock {
		this.sendNotification(
			notifications.StoreAvailability{OrderId: request.Id},
			events.StoreUnavailableEvent,
		)
		return
	}
	this.sendNotification(
		notifications.StoreAvailability{OrderId: request.Id},
		events.OrderStoreOkEvent,
	)
}

func (this *Store) handleOrderDeleted(message amqp.Delivery) {
	request := requests.DeleteOrderRequest{}
	err := json.Unmarshal(message.Body, &request)
	if err != nil {
		log.Fatalf("Failed to decode request '%v'\n", message.Body)
	}

	log.Printf("Received an delete request for the order '%v'\n", request.OrderId)

	order, ok := this.OrderRequests[request.OrderId]
	if !ok {
		log.Fatalf("invalid order id '%v'\n", order.Id)
		return
	}

	for _, product := range order.Products {
		p := this.Products[product.ProductId]
		p.Quantity += product.Quantity
	}

	delete(this.OrderRequests, request.OrderId)
}

func (this *Store) sendNotification(object any, event interfaces.Event) {
	notification, err := json.Marshal(object)
	if err != nil {
		log.Printf("failed to serialize request: %v\n", err)
		return
	}

	privateKey, err := helpers.ReadPrivateKeyPEM("./services/store/store.pem")
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
