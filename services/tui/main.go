package main

import (
	"context"
	"eCommerce/pkg/config"
	"eCommerce/pkg/events"
	"eCommerce/pkg/exchanges"
	"eCommerce/pkg/helpers"
	"log"
	"time"

	amqp "github.com/rabbitmq/amqp091-go"
)

func main() {
	cfg, err := config.Load()
	helpers.FailOnError(err, "Failed to load configuration")

	conn, err := amqp.Dial(cfg.RabbitMQURL)
	helpers.FailOnError(err, "Failed to connect to RabbitMQ")
	defer conn.Close()

	ch, err := conn.Channel()
	helpers.FailOnError(err, "Failed to open a channel")
	defer ch.Close()

	var ecommerceExchange = exchanges.GetEcommerceExchangeInfo()

	err = ch.ExchangeDeclare(
		ecommerceExchange.Name,         // name
		ecommerceExchange.ExchangeType, // type
		false,                          // durability
		false,                          // auto-deleted
		false,                          // internal
		false,                          // no-wait
		nil,                            // arguments
	)
	helpers.FailOnError(err, "Failed to declare a exchange")


	tuiQueue, err := ch.QueueDeclare(
		"",    // name
		false, // durability
		false, // delete when unused
		true,  // exclusive
		false, // no-wait
		nil,
	)
	helpers.FailOnError(err, "Failed to declare a queue")

	err = ch.QueueBind(
		tuiQueue.Name,      // queue
		events.PaymentEventApproved.String(), // routing
		ecommerceExchange.Name,     // exchange
		false,           // no-wait
		nil,             // arguments
	)
	helpers.FailOnError(err, "Failed to bind a queue")

	err = ch.QueueBind(
		tuiQueue.Name,      // queue
		events.StockEventUnavailable.String(), // routing
		ecommerceExchange.Name,     // exchange
		false,           // no-wait
		nil,             // arguments
	)
	helpers.FailOnError(err, "Failed to bind a queue")

	msgs, err := ch.Consume(
		tuiQueue.Name, // queue
		"",         // consumer
		true,       // auto-ack
		false,      // exclusive
		false,      // no-local
		false,      // no-wait
		nil,        // args
	)
	helpers.FailOnError(err, "Failed to register a consumer")

	var forever chan struct{}

	go func() {
		for d := range msgs {
			log.Printf("Received a message: %s", d.Body)
		}
	}()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	body := "Hello World!"
	err = ch.PublishWithContext(ctx,
		ecommerceExchange.Name,            // exchange
		events.OrderEventCreated.String(), // routing key
		false,                             // mandatory
		false,                             // immediate
		amqp.Publishing{
			ContentType: "text/plain",
			Body:        []byte(body),
		})
	helpers.FailOnError(err, "Failed to publish a message")
	log.Printf(" [x] Sent %s\n", body)
	<-forever
}
