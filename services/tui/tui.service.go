package main

import (
	"eCommerce/pkg/config"
	"eCommerce/pkg/events"
	"eCommerce/pkg/exchanges"
	"fmt"
	"log"

	amqp "github.com/rabbitmq/amqp091-go"
)

type TerminalUserInterface struct {
	Config     *config.Config
	Connection *amqp.Connection
	Channel    *amqp.Channel
	Queue      amqp.Queue
	Messages   <-chan amqp.Delivery
	publisher  messagePublisher
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
		return nil, fmt.Errorf("carregar configuração: %w", err)
	}

	tui.Connection, err = amqp.Dial(tui.Config.RabbitMQURL)
	if err != nil {
		return nil, fmt.Errorf("conectar ao RabbitMQ: %w", err)
	}

	tui.Channel, err = tui.Connection.Channel()
	if err != nil {
		tui.Connection.Close()
		return nil, fmt.Errorf("abrir canal: %w", err)
	}
	tui.publisher = tui.Channel

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
		tui.DestroyTUI()
		return nil, fmt.Errorf("declarar exchange: %w", err)
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
		tui.DestroyTUI()
		return nil, fmt.Errorf("declarar fila: %w", err)
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
			tui.DestroyTUI()
			return nil, fmt.Errorf("associar fila %q ao evento %q: %w", tui.Queue.Name, key, err)
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
		tui.DestroyTUI()
		return nil, fmt.Errorf("registrar consumidor na fila %q: %w", tui.Queue.Name, err)
	}

	go tui.handleMessage()

	return tui, nil
}

func (tui *TerminalUserInterface) handleMessage() {
	for message := range tui.Messages {
		log.Printf("Received a message: %s", message.Body)
	}
}
