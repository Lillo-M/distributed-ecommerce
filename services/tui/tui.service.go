package main

import (
	"context"
	"eCommerce/pkg/config"
	"eCommerce/pkg/events"
	"eCommerce/pkg/exchanges"
	"eCommerce/pkg/helpers"
	"fmt"
	"log"
	"time"

	amqp "github.com/rabbitmq/amqp091-go"
)

type TerminalUserInterface struct {
	Config     *config.Config
	Connection *amqp.Connection
	Channel    *amqp.Channel
	Queue      amqp.Queue
	Messages   <-chan amqp.Delivery
}

func (tui *TerminalUserInterface) DestroyTUI() {
	if tui.Channel != nil {
		tui.Channel.Close()
	}
	if tui.Connection != nil {
		tui.Connection.Close()
	}
}

func CreateTUI() (*TerminalUserInterface, error) {
	var tui *TerminalUserInterface = &TerminalUserInterface{}
	var err error
	tui.Config, err = config.Load()
	if err != nil {
		helpers.LogErrorMessage(err, "Failed to load configuration")
		return nil, err
	}

	tui.Connection, err = amqp.Dial(tui.Config.RabbitMQURL)
	if err != nil {
		helpers.LogErrorMessage(err, "Failed to connect to RabbitMQ")
		return nil, err
	}

	tui.Channel, err = tui.Connection.Channel()
	if err != nil {
		helpers.LogErrorMessage(err, "Failed to open a channel")
		tui.Connection.Close()
		return nil, err
	}

	var ecommerceExchange = exchanges.GetEcommerceExchangeInfo()
	err = tui.Channel.ExchangeDeclare(
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
		tui.DestroyTUI()
		return nil, err
	}

	tui.Queue, err = tui.Channel.QueueDeclare(
		"",    // name
		false, // durability
		false, // delete when unused
		true,  // exclusive
		false, // no-wait
		nil,
	)
	if err != nil {
		helpers.LogErrorMessage(err, "Failed to declare a queue")
		tui.DestroyTUI()
		return nil, err
	}

	routingKeys := []string{
		events.PaymentEventApproved.String(),
		events.StockEventUnavailable.String(),
	}
	for _, key := range routingKeys {
		if err := tui.Channel.QueueBind(
			tui.Queue.Name,         // queue
			key,                    // routing
			ecommerceExchange.Name, // exchange
			false,                  // no-wait
			nil,                    // arguments
		); err != nil {
			helpers.LogErrorMessage(err, fmt.Sprintf("Failed to bind queue '%s' to key '%s'", tui.Queue.Name, key))
			tui.DestroyTUI()
			return nil, err
		}
	}

	tui.Messages, err = tui.Channel.Consume(
		tui.Queue.Name, // queue
		"",             // consumer
		true,           // auto-ack
		false,          // exclusive
		false,          // no-local
		false,          // no-wait
		nil,            // args
	)
	if err != nil {
		helpers.LogErrorMessage(err,
			fmt.Sprintf("Failed to register a consumer to queue '%v'",
				tui.Queue.Name))
		tui.DestroyTUI()
		return nil, err
	}

	go tui.handleMessage()

	return tui, err
}

func (tui *TerminalUserInterface) handleMessage() {
	for message := range tui.Messages {
		log.Printf("Received a message: %s", message.Body)
	}
}

func (tui *TerminalUserInterface) TestSend(body string) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	var err = tui.Channel.PublishWithContext(ctx,
		exchanges.GetEcommerceExchangeInfo().Name, // exchange
		events.OrderEventCreated.String(),         // routing key
		false,                                     // mandatory
		false,                                     // immediate
		amqp.Publishing{
			ContentType: "text/plain",
			Body:        []byte(body),
		})
	helpers.FailOnError(err, "Failed to publish a message")
	log.Printf(" [x] Sent %s\n", body)
}
