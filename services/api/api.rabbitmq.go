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
	"time"

	amqp "github.com/rabbitmq/amqp091-go"
)

func (this *MainApiService) HandleMessages(message amqp.Delivery) {
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
	case events.PaymentEventApproved.String():
		this.handlePaymentApproved(message)
		log.Printf("Received a payment approved: %s", message.Body)
	case events.PaymentEventRefused.String():
		this.handlePaymentRefused(message)
		log.Printf("Received a payment refused: %s", message.Body)
	case events.OrderSentEvent.String():
		this.handleOrderSent(message)
		log.Printf("Received an order sent: %s", message.Body)
	case events.OrderStoreOkEvent.String():
		this.handleStoreOk(message)
		log.Printf("Received a store ok: %s", message.Body)
	case events.StoreUnavailableEvent.String():
		this.handleStoreUnavailable(message)
		log.Printf("Received a store unavailable: %s", message.Body)
	case events.PaymentEventCheckoutCreated.String():
		this.handleCheckoutCreated(message)
		log.Printf("Received a checkout created: %s", message.Body)
	default:
		log.Printf("routing key '%q' unkown\n", message.RoutingKey)
	}
}

func (this *MainApiService) handlePaymentApproved(message amqp.Delivery) {
	notification := notifications.StoreAvailability{}
	err := json.Unmarshal(message.Body, &notification)
	if err != nil {
		log.Fatalf("Failed to decode notification '%v'\n", message.Body)
	}

	this.ordersMutex.Lock()
	defer this.ordersMutex.Unlock()
	order, ok := this.Orders[notification.OrderId]
	if !ok {
		log.Printf("invalid order id '%v'\n", notification.OrderId)
		return
	}
	order.Status = mock.OrderStatusPaymentApproved
	this.Orders[notification.OrderId] = order
}

func (this *MainApiService) handleOrderSent(message amqp.Delivery) {
	notification := notifications.DeliveryStatus{}
	err := json.Unmarshal(message.Body, &notification)
	if err != nil {
		log.Fatalf("Failed to decode notification '%v'\n", message.Body)
	}

	this.ordersMutex.Lock()
	defer this.ordersMutex.Unlock()
	order, ok := this.Orders[notification.OrderId]
	if !ok {
		log.Printf("invalid order id '%v'\n", notification.OrderId)
		return
	}
	order.Status = mock.OrderStatusSent
	this.Orders[notification.OrderId] = order
}

func (this *MainApiService) handlePaymentRefused(message amqp.Delivery) {
	notification := notifications.StoreAvailability{}
	err := json.Unmarshal(message.Body, &notification)
	if err != nil {
		log.Fatalf("Failed to decode notification '%v'\n", message.Body)
	}

	notificationBytes, err := json.Marshal(notifications.DeleteOrderNotification{
		OrderId: notification.OrderId,
	})
	if err != nil {
		log.Printf("failed to serialize notification: %v\n", err)
		return
	}

	this.publishEvent(notificationBytes, events.OrderDeletedEvent)
	this.ordersMutex.Lock()
	defer this.ordersMutex.Unlock()
	order, ok := this.Orders[notification.OrderId]
	if !ok {
		log.Printf("invalid order id '%v'\n", notification.OrderId)
		return
	}
	order.Status = mock.OrderStatusPaymentDenied
	this.Orders[notification.OrderId] = order
}

func (this *MainApiService) handleStoreOk(message amqp.Delivery) {
	notification := notifications.StoreAvailability{}
	err := json.Unmarshal(message.Body, &notification)
	if err != nil {
		log.Fatalf("Failed to decode notification '%v'\n", message.Body)
	}

	this.ordersMutex.Lock()
	defer this.ordersMutex.Unlock()
	order, ok := this.Orders[notification.OrderId]
	if !ok {
		log.Printf("invalid order id '%v'\n", notification.OrderId)
		return
	}
	order.Status = mock.OrderStatusStockReserved
	this.Orders[notification.OrderId] = order
}

func (this *MainApiService) handleStoreUnavailable(message amqp.Delivery) {
	notification := notifications.StoreAvailability{}
	err := json.Unmarshal(message.Body, &notification)
	if err != nil {
		log.Fatalf("Failed to decode notification '%v'\n", message.Body)
	}

	notificationBytes, err := json.Marshal(notifications.DeleteOrderNotification{
		OrderId: notification.OrderId,
	})
	if err != nil {
		log.Printf("failed to serialize notification: %v\n", err)
		return
	}

	this.publishEvent(notificationBytes, events.OrderDeletedEvent)
	this.ordersMutex.Lock()
	defer this.ordersMutex.Unlock()
	order, ok := this.Orders[notification.OrderId]
	if !ok {
		log.Printf("invalid order id '%v'\n", notification.OrderId)
		return
	}
	order.Status = mock.OrderStatusOutOfStock
	this.Orders[notification.OrderId] = order
}

func (this *MainApiService) handleCheckoutCreated(message amqp.Delivery) {
	notification := notifications.PaymentStatus{}
	err := json.Unmarshal(message.Body, &notification)
	if err != nil {
		log.Printf("Failed to decode notification '%v'\n", message.Body)
		return
	}

	this.ordersMutex.Lock()
	defer this.ordersMutex.Unlock()
	order, ok := this.Orders[notification.OrderId]
	if !ok {
		log.Printf("invalid order id '%v'\n", notification.OrderId)
		return
	}
	order.CheckoutUrl = notification.CheckoutUrl
	this.Orders[notification.OrderId] = order
}

func (this *MainApiService) sendNotification(object any, event interfaces.Event) {
	notification, err := json.Marshal(object)
	if err != nil {
		log.Printf("failed to serialize notification: %v\n", err)
		return
	}

	privateKey, err := helpers.ReadPrivateKeyPEM("./services/api/main.pem")
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

func (this *MainApiService) SendToSalesExchange(publishing amqp.Publishing, routingKey interfaces.Event) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	var err = this.Channel.PublishWithContext(ctx,
		exchanges.GetSalesExchangeInfo().Name, // exchange
		routingKey.String(),                   // routing key
		false,                                 // mandatory
		false,                                 // immediate
		publishing,
	)
	helpers.FailOnError(err, "Failed to publish a message")
	log.Printf(" [x] Sent %s\n", publishing.Body)
}

func (this *MainApiService) SendToECommerceExchange(publishing amqp.Publishing, routingKey interfaces.Event) {
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

func (this *MainApiService) publishEvent(payload []byte, routingKey events.OrderEvent) {
	privateKey, err := helpers.ReadPrivateKeyPEM("./services/api/main.pem")
	if err != nil {
		log.Fatalf("Failed to read private key: %v", err)
	}

	checksum := sha256.Sum256(payload)

	signature, err := rsa.SignPSS(crand.Reader, privateKey, crypto.SHA256, checksum[:], nil)
	if err != nil {
		log.Fatalf("Failed to sign payload: %v", err)
	}

	this.SendToECommerceExchange(
		amqp.Publishing{
			ContentType: "application/json",
			Headers:     amqp.Table{"x-signature": signature},
			Body:        payload,
		},
		routingKey,
	)
}

func (this *MainApiService) publishSaleEvent(payload []byte, routingKey events.SaleEvent) {
	privateKey, err := helpers.ReadPrivateKeyPEM("./services/api/main.pem")
	if err != nil {
		log.Fatalf("Failed to read private key: %v", err)
	}

	checksum := sha256.Sum256(payload)

	signature, err := rsa.SignPSS(crand.Reader, privateKey, crypto.SHA256, checksum[:], nil)
	if err != nil {
		log.Fatalf("Failed to sign payload: %v", err)
	}

	this.SendToSalesExchange(
		amqp.Publishing{
			ContentType: "application/json",
			Headers:     amqp.Table{"x-signature": signature},
			Body:        payload,
		},
		routingKey,
	)
}
