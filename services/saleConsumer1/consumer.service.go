package main

import (
	"context"
	"eCommerce/pkg/events"
	"eCommerce/pkg/exchanges"
	"eCommerce/pkg/helpers"
	"eCommerce/pkg/interfaces"
	"encoding/json"
	"log"
	"strings"
	"time"

	amqp "github.com/rabbitmq/amqp091-go"
)

type ConsumerService struct {
	interfaces.Service
}

func CreateConsumerService() (*ConsumerService, error) {
	var consumer *ConsumerService = &ConsumerService{}
	var err error

	err = consumer.InitRMQChannel()
	if err != nil {
		helpers.LogErrorMessage(err, "Failed to init RabbitMQ Channel")
		return nil, err
	}

	var salesExchange = exchanges.GetSalesExchangeInfo()
	err = consumer.Channel.ExchangeDeclare(
		salesExchange.Name, // name
		salesExchange.Type, // type
		false,                  // durability
		false,                  // auto-deleted
		false,                  // internal
		false,                  // no-wait
		nil,                    // arguments
	)

	if err != nil {
		helpers.LogErrorMessage(err, "Failed to declare a exchange")
		consumer.CloseRMQChannel()
		return nil, err
	}

	routingKeys := []string{
		events.SaleCategoryA.String(),
		events.SaleCategoryB.String(),
	}

	err = consumer.BindRoutingKeys(routingKeys, salesExchange)
	if err != nil {
		helpers.LogErrorMessage(err, "Failed to declare a exchange")
		consumer.CloseRMQChannel()
		return nil, err
	}

	consumer.StartConsuming(consumer.HandleMessages)

	return consumer, nil
}

func (this *ConsumerService) HandleMessages(message amqp.Delivery) {
	switch message.RoutingKey {
	case events.SaleCategoryA.String():
		fallthrough
	case events.SaleCategoryB.String():
		log.Printf("Received sale on category %v\n", strings.Split(message.RoutingKey, ".")[2])
	default:
		log.Fatalf("Unknown routing key '%v'\n", message.RoutingKey)
	}
}

func (this *ConsumerService) sendNotification(object any, event interfaces.Event) {
	notification, err := json.Marshal(object)
	if err != nil {
		log.Printf("failed to serialize request: %v\n", err)
		return
	}
	this.SendToSalesExchange(notification, event)
}

func (this *ConsumerService) SendToSalesExchange(body []byte, routingKey interfaces.Event) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	var err = this.Channel.PublishWithContext(ctx,
		exchanges.GetSalesExchangeInfo().Name, // exchange
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
