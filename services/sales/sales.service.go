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
	"strings"
	"time"

	amqp "github.com/rabbitmq/amqp091-go"
)

type SalesService struct {
	interfaces.Service
}

func CreateSalesService() (*SalesService, error) {
	var sales *SalesService = &SalesService{}
	var err error

	err = sales.InitRMQChannel()
	if err != nil {
		helpers.LogErrorMessage(err, "Failed to init RabbitMQ Channel")
		return nil, err
	}

	var salesExchange = exchanges.GetSalesExchangeInfo()
	err = sales.Channel.ExchangeDeclare(
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
		sales.CloseRMQChannel()
		return nil, err
	}

	return sales, nil
}

func (this *SalesService) HandleMessages(message amqp.Delivery) {
	switch message.RoutingKey {
	default:
		log.Fatalf("Unknown routing key '%v'\n", message.RoutingKey)
	}
}

func (this *SalesService) generateSales() {
	for true {
		time.Sleep(time.Second)
		value, err := rand.Int(rand.Reader, big.NewInt(100))
		if err != nil {
			log.Fatalf("Failed to generate a random int '%v'\n", err)
			return
		}
		v := value.Int64()
		switch {
		case v < 33:
			this.sendNotification(
				notifications.SaleNotification{Category: strings.Split(events.SaleCategoryA.String(), ".")[2]},
				events.SaleCategoryA,
			)
		case v >= 33 && v < 67:
			this.sendNotification(
				notifications.SaleNotification{Category: strings.Split(events.SaleCategoryB.String(), ".")[2]},
				events.SaleCategoryB,
			)
		case v >= 67 && v < 100:
			this.sendNotification(
				notifications.SaleNotification{Category: strings.Split(events.SaleCategoryC.String(), ".")[2]},
				events.SaleCategoryC,
			)
		}
	}
}

func (this *SalesService) sendNotification(object any, event interfaces.Event) {
	notification, err := json.Marshal(object)
	if err != nil {
		log.Printf("failed to serialize request: %v\n", err)
		return
	}
	this.SendToSalesExchange(notification, event)
}

func (this *SalesService) SendToSalesExchange(body []byte, routingKey interfaces.Event) {
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
