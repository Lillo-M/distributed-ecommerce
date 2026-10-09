package main

import (
	"context"
	"eCommerce/pkg/events"
	"eCommerce/pkg/exchanges"
	"eCommerce/pkg/helpers"
	"eCommerce/pkg/interfaces"
	"eCommerce/pkg/notifications"
	"encoding/json"
	"log"
	"time"

	amqp "github.com/rabbitmq/amqp091-go"
)

type DeliveryService struct {
	interfaces.Service
}

const (
	ApproveChancePercent = 50
)

func CreateDeliveryService() (*DeliveryService, error) {
	var delivery *DeliveryService = &DeliveryService{}
	var err error

	err = delivery.InitRMQChannel()
	if err != nil {
		helpers.LogErrorMessage(err, "Failed to init RabbitMQ Channel")
		return nil, err
	}

	var ecommerceExchange = exchanges.GetEcommerceExchangeInfo()
	err = delivery.Channel.ExchangeDeclare(
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
		delivery.CloseRMQChannel()
		return nil, err
	}

	routingKeys := []string{
		events.PaymentEventApproved.String(),
	}

	err = delivery.BindRoutingKeys(routingKeys, ecommerceExchange)
	if err != nil {
		helpers.LogErrorMessage(err, "Failed to declare a exchange")
		delivery.CloseRMQChannel()
		return nil, err
	}

	delivery.StartConsuming(delivery.HandleMessages)

	return delivery, nil
}

func (this *DeliveryService) HandleMessages(message amqp.Delivery) {
	switch message.RoutingKey {
	case events.PaymentEventApproved.String():
		this.handlePaymentApproved(message)
		log.Printf("Received a payment approved: %s", message.Body)
	default:
		log.Fatalf("Unknown routing key '%v'\n", message.RoutingKey)
	}
}

func (this *DeliveryService) handlePaymentApproved(message amqp.Delivery) {
	notification := notifications.PaymentStatus{}
	err := json.Unmarshal(message.Body, &notification)
	if err != nil {
		log.Fatalf("Failed to decode notification '%v'\n", message.Body)
	}
	this.sendNotification(
		notifications.DeliveryStatus{OrderId: notification.OrderId},
		events.OrderSentEvent,
	)
}

func (this *DeliveryService) sendNotification(object any, event interfaces.Event) {
	notification, err := json.Marshal(object)
	if err != nil {
		log.Printf("failed to serialize request: %v\n", err)
		return
	}
	this.SendToECommerceExchange(notification, event)
}

func (this *DeliveryService) SendToECommerceExchange(body []byte, routingKey interfaces.Event) {
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
