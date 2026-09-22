package interfaces

import (
	"eCommerce/pkg/config"
	"eCommerce/pkg/exchanges"
	"eCommerce/pkg/helpers"
	"fmt"

	amqp "github.com/rabbitmq/amqp091-go"
)

type Service struct {
	Config     *config.Config
	Connection *amqp.Connection
	Channel    *amqp.Channel
	Queue      amqp.Queue
	Messages   <-chan amqp.Delivery
}

func (this *Service) InitRMQChannel() error {
	var err error
	if this.Config == nil {
		this.Config, err = config.Load()
		if err != nil {
			helpers.LogErrorMessage(err, "Failed to load configuration")
			return err
		}
	}

	this.Connection, err = amqp.Dial(this.Config.RabbitMQURL)
	if err != nil {
		helpers.LogErrorMessage(err, "Failed to connect to RabbitMQ")
		return err
	}

	this.Channel, err = this.Connection.Channel()
	if err != nil {
		helpers.LogErrorMessage(err, "Failed to open a channel")
		this.Connection.Close()
		return err
	}

	this.Queue, err = this.Channel.QueueDeclare(
		"",    // name
		false, // durability
		false, // delete when unused
		true,  // exclusive
		false, // no-wait
		nil,
	)
	if err != nil {
		helpers.LogErrorMessage(err, "Failed to declare a queue")
		this.CloseRMQChannel()
		return err
	}

	return nil
}

func (this *Service) CloseRMQChannel() {
	if this.Channel != nil &&
		!this.Channel.IsClosed() {
		this.Channel.Close()
	}
	if this.Connection != nil &&
		!this.Connection.IsClosed() {
		this.Connection.Close()
	}
}

func (this *Service) BindRoutingKeys(routingKeys []string, ecommerceExchange exchanges.ExchangeInfo) error {
	for _, key := range routingKeys {
		err := this.Channel.QueueBind(
			this.Queue.Name,        // queue
			key,                    // routing
			ecommerceExchange.Name, // exchange
			false,                  // no-wait
			nil,                    // arguments
		)
		if err != nil {
			err = fmt.Errorf("Failed to bind queue '%s' to key '%s': %v", this.Queue.Name, key, err)
			return err
		}
	}
	return nil
}

func (s *Service) StartConsuming(handler func(msg amqp.Delivery)) error {
	var err error
	s.Messages, err = s.Channel.Consume(
		s.Queue.Name, // queue
		"",           // consumer
		true,         // auto-ack
		false,        // exclusive
		false,        // no-local
		false,        // no-wait
		nil,          // args
	)
	if err != nil {
		return fmt.Errorf("failed to register consumer to queue '%s': %w", s.Queue.Name, err)
	}

	go func() {
		for message := range s.Messages {
			handler(message)
		}
	}()

	return nil
}
