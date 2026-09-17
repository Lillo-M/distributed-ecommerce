package main

import (
	"context"
	"eCommerce/pkg/events"
	"eCommerce/pkg/exchanges"
	"eCommerce/pkg/helpers"
	"log"
	"time"

	amqp "github.com/rabbitmq/amqp091-go"
)

func main() {
	conn, err := amqp.Dial("amqp://guest:guest@localhost:5672/")
	helpers.FailOnError(err, "Failed to connect to RabbitMQ")
	defer conn.Close()

	ch, err := conn.Channel()
	helpers.FailOnError(err, "Failed to open a channel")
	defer ch.Close()

	queue, err := ch.QueueDeclare(
		"",    // name
		false, // durability
		false, // delete when unused
		true,  // exclusive
		false, // no-wait
		nil,
	)
	helpers.FailOnError(err, "Failed to declare a queue")

	var ecommerceExchange = exchanges.GetEcommerceExchangeInfo()

	err = ch.QueueBind(
		queue.Name,                        // queue
		events.OrderEventCreated.String(), // routing
		ecommerceExchange.Name,            // exchange
		false,                             // no-wait
		nil,                               // arguments
	)
	helpers.FailOnError(err, "Failed to bind a queue")

	msgs, err := ch.Consume(
		queue.Name, // queue
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
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()

			body := "Stock Unavailable!"
			err = ch.PublishWithContext(ctx,
				ecommerceExchange.Name,                // exchange
				events.StoreEventUnavailable.String(), // routing key
				false,                                 // mandatory
				false,                                 // immediate
				amqp.Publishing{
					ContentType: "text/plain",
					Body:        []byte(body),
				})
			helpers.FailOnError(err, "Failed to publish a message")
			log.Printf(" [x] Sent %s\n", body)
		}
	}()

	log.Printf(" [*] Waiting for messages. To exit press CTRL+C")
	<-forever
}
