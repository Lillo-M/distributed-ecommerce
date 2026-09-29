package main

import (
	"context"
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

	store.StartConsuming(store.HandleMessages)

	return store, nil
}

func (store *Store) initProductsMap(products []mock.Product) {
	store.Products = make(map[string]mock.Product)
	for _, product := range products {
		store.Products[product.UUID] = product
	}
}

func (this *Store) HandleMessages(message amqp.Delivery) {
	switch message.RoutingKey {
	case events.OrderCreatedEvent.String():
		this.handleOrderCreated(message)
	case events.OrderDeletedEvent.String():
		log.Fatalf("Routing key not handled '%v'\n", message.RoutingKey)
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

	for _, productOrdered := range request.Products {
		storeProduct, ok := this.Products[productOrdered.ProductId]
		if !ok {
			log.Fatalf("invalid product id '%v'\n", productOrdered.ProductId)
			return
		}
		if storeProduct.Quantity < productOrdered.Quantity {
			this.sendNotification(
				notifications.StoreAvailability{OrderId: request.Id},
				events.StoreUnavailableEvent,
			)
			return
		}
	}

	for _, product := range request.Products {
		p := this.Products[product.ProductId]
		p.Quantity -= product.Quantity
	}

	this.OrderRequests[request.Id] = request

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
	this.SendToECommerceExchange(notification, event)
}

func (this *Store) SendToECommerceExchange(body []byte, routingKey interfaces.Event) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	var err = this.Channel.PublishWithContext(ctx,
		exchanges.GetEcommerceExchangeInfo().Name, // exchange
		routingKey.String(),                       // routing key
		false,                                     // mandatory
		false,                                     // immediate
		amqp.Publishing{
			ContentType: "application/json",
			Body:        []byte(body),
		})
	helpers.FailOnError(err, "Failed to publish a message")
	log.Printf(" [x] Sent %s\n", body)
}
