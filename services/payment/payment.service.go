package main

import (
	"context"
	"crypto/rand"
	"eCommerce/pkg/events"
	"eCommerce/pkg/exchanges"
	"eCommerce/pkg/helpers"
	"eCommerce/pkg/interfaces"
	"eCommerce/pkg/notifications"
	"encoding/json"
	"log"
	"math/big"
	"time"

	amqp "github.com/rabbitmq/amqp091-go"
)

type PaymentService struct {
	interfaces.Service
}

const (
	ApproveChancePercent = 50
)

func CreatePaymentService() (*PaymentService, error) {
	var payment *PaymentService = &PaymentService{}
	var err error

	err = payment.InitRMQChannel()
	if err != nil {
		helpers.LogErrorMessage(err, "Failed to init RabbitMQ Channel")
		return nil, err
	}

	var ecommerceExchange = exchanges.GetEcommerceExchangeInfo()
	err = payment.Channel.ExchangeDeclare(
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
		payment.CloseRMQChannel()
		return nil, err
	}

	routingKeys := []string{
		events.OrderStoreOkEvent.String(),
	}

	err = payment.BindRoutingKeys(routingKeys, ecommerceExchange)
	if err != nil {
		helpers.LogErrorMessage(err, "Failed to declare a exchange")
		payment.CloseRMQChannel()
		return nil, err
	}

	payment.StartConsuming(payment.HandleMessages)

	return payment, nil
}

func (this *PaymentService) HandleMessages(message amqp.Delivery) {
	switch message.RoutingKey {
	case events.OrderStoreOkEvent.String():
		this.handleStoreOk(message)
		log.Printf("Received a store ok: %s", message.Body)
	default:
		log.Fatalf("Unknown routing key '%v'\n", message.RoutingKey)
	}
}

func (this *PaymentService) handleStoreOk(message amqp.Delivery) {
	notification := notifications.StoreAvailability{}
	err := json.Unmarshal(message.Body, &notification)
	if err != nil {
		log.Fatalf("Failed to decode notification '%v'\n", message.Body)
	}
	value, err := rand.Int(rand.Reader, big.NewInt(100))
	log.Printf("got rand %v\n", value.Int64())
	if err != nil {
		log.Fatalf("Failed to generate a random integer: %v", err)
		return
	}
	if value.Int64() < ApproveChancePercent {
		this.sendNotification(
			notifications.PaymentStatus{OrderId: notification.OrderId},
			events.PaymentEventApproved,
		)
		return
	}
	this.sendNotification(
		notifications.PaymentStatus{OrderId: notification.OrderId},
		events.PaymentEventRefused,
	)
}

func (this *PaymentService) sendNotification(object any, event interfaces.Event) {
	notification, err := json.Marshal(object)
	if err != nil {
		log.Printf("failed to serialize request: %v\n", err)
		return
	}
	this.SendToECommerceExchange(notification, event)
}

func (this *PaymentService) SendToECommerceExchange(body []byte, routingKey interfaces.Event) {
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
