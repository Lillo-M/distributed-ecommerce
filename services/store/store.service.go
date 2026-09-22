package main

import (
	"eCommerce/pkg/events"
	"eCommerce/pkg/exchanges"
	"eCommerce/pkg/helpers"
	"eCommerce/pkg/interfaces"
	"eCommerce/pkg/mock"
	"eCommerce/pkg/requests"
	"encoding/json"
	"log"
	"strconv"

	amqp "github.com/rabbitmq/amqp091-go"
)

type Store struct {
	interfaces.Service
	Products map[string]mock.Product
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
		request := requests.CreateOrderRequest{}
		err := json.Unmarshal(message.Body, &request)
		if err != nil {
			log.Fatalf("Failed to decode request '%v'\n", message.Body)
		}
		log.Printf("Received an order of:\n")
		for _, product := range request.Products {
			log.Printf("- %v %v\n", product.Quantity, strconv.Quote(this.Products[product.ProductId].Name))
		}
	case events.OrderDeletedEvent.String():
		log.Fatalf("Routing key not handled '%v'\n", message.RoutingKey)
	default:
		log.Fatalf("Unknown routing key '%v'\n", message.RoutingKey)
	}
}
