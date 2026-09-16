package main

import (
	"context"
	"eCommerce/pkg/events"
	"eCommerce/pkg/exchanges"
	"encoding/json"
	"fmt"
	"time"

	amqp "github.com/rabbitmq/amqp091-go"
)

type messagePublisher interface {
	PublishWithContext(context.Context, string, string, bool, bool, amqp.Publishing) error
}

func (tui *TerminalUserInterface) Callbacks() MenuCallbacks {
	return MenuCallbacks{
		ListProducts: tui.OnListProducts,
		CreateOrder:  tui.OnCreateOrder,
		DeleteOrder:  tui.OnDeleteOrder,
		ListOrders:   tui.OnListOrders,
	}
}

func (tui *TerminalUserInterface) OnListProducts() error {
	// TODO: conectar a consulta quando houver um contrato para consultar produtos.
	return ErrActionUnavailable
}

func (tui *TerminalUserInterface) OnListOrders(customerID string) error {
	// TODO: consultar pedidos e status do usuário quando essa operação existir.
	return ErrActionUnavailable
}

func (tui *TerminalUserInterface) OnCreateOrder(request events.CreateOrderRequest) error {
	return tui.publishOrderEvent(events.OrderEventCreated, request)
}

func (tui *TerminalUserInterface) OnDeleteOrder(request events.DeleteOrderRequest) error {
	return tui.publishOrderEvent(events.OrderEventDeleted, request)
}

func (tui *TerminalUserInterface) publishOrderEvent(event events.OrderEvent, payload any) error {
	if tui.publisher == nil {
		return fmt.Errorf("conexão de publicação indisponível")
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("preparar solicitação: %w", err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	err = tui.publisher.PublishWithContext(ctx,
		exchanges.GetEcommerceExchangeInfo().Name,
		event.String(),
		false,
		false,
		amqp.Publishing{ContentType: "application/json", Body: body},
	)
	if err != nil {
		return fmt.Errorf("enviar solicitação: %w", err)
	}
	return nil
}
