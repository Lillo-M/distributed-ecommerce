package main

import (
	"crypto/rsa"
	"eCommerce/pkg/config"
	"eCommerce/pkg/events"
	"eCommerce/pkg/exchanges"
	"eCommerce/pkg/helpers"
	"encoding/json"
	"fmt"
	"log"
	"slices"
	"sync"

	amqp "github.com/rabbitmq/amqp091-go"
)

type TerminalUserInterface struct {
	Config     *config.Config
	Connection *amqp.Connection
	Channel    *amqp.Channel
	Queue      amqp.Queue
	Messages   <-chan amqp.Delivery
	publisher  messagePublisher
	ordersMu   sync.Mutex
	orders     map[string]sessionOrder
	privateKey *rsa.PrivateKey
}

// Os itens são mantidos para publicar a exclusão no contrato esperado pelo Estoque.
type sessionOrder struct {
	customerID string
	payload    events.PedidoPayload
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

	tui.privateKey, err = helpers.ReadPrivateKeyPEM("./pkg/private-keys/main.pem")
	if err != nil {
		return nil, fmt.Errorf("carregar chave privada: %w", err)
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
	err = ecommerceExchange.Declare(tui.Channel)
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
		events.PaymentEventRefused.String(),
		events.OrderEventSent.String(),
		events.OrderEventStockOk.String(),
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

		producerKey, err := helpers.GetProducerPublicKey(message)
		if err != nil {
			log.Printf("Erro ao ler chave publica do producer de %v: %v", message.RoutingKey, err)
			continue
		}

		err = helpers.VerifyMessage(message, producerKey)
		if err != nil {
			log.Printf("Falha ao validar assinatura do producer do evento %v: %v", message.RoutingKey, err)
			continue
		}

		var payload events.PedidoPayload
		if err := json.Unmarshal(message.Body, &payload); err != nil {
			log.Printf("Evento %s inválido: %v", message.RoutingKey, err)
			continue
		}

		if status, changed := tui.updateOrderStatus(payload.ID, message.RoutingKey); changed {
			log.Printf("Pedido %s atualizado: %s", payload.ID, status)
		}

		if slices.Contains([]string{events.PaymentEventRefused.String(), events.StockEventUnavailable.String()}, message.RoutingKey) {
			tui.publishOrderEvent(events.OrderEventDeleted, payload)
		}
	}
}
