package main

import (
	"context"
	"crypto"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"eCommerce/pkg/events"
	"eCommerce/pkg/exchanges"
	"encoding/json"
	"fmt"
	"os"
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
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	products, err := tui.listProducts(ctx)
	if err != nil {
		return err
	}
	return displayProducts(os.Stdout, products)
}

func (tui *TerminalUserInterface) OnListOrders(customerID string) error {
	return displayOrders(os.Stdout, tui.ordersForCustomer(customerID))
}

func (tui *TerminalUserInterface) OnCreateOrder(request events.CreateOrderRequest) error {
	payload := events.PedidoPayload{
		ID:     request.OrderID,
		Status: statusCreated,
		Itens:  make([]events.ItemPedido, len(request.Items)),
	}
	for i, item := range request.Items {
		payload.Itens[i] = events.ItemPedido{ProductID: item.ProductID, Quantity: item.Quantity}
	}

	tui.ordersMu.Lock()
	defer tui.ordersMu.Unlock()
	if _, exists := tui.orders[request.OrderID]; exists {
		return fmt.Errorf("pedido %s já foi enviado nesta sessão", request.OrderID)
	}
	if err := tui.publishOrderEvent(events.OrderEventCreated, payload); err != nil {
		return err
	}
	if tui.orders == nil {
		tui.orders = make(map[string]sessionOrder)
	}
	tui.orders[request.OrderID] = sessionOrder{customerID: request.CustomerID, payload: payload}
	return nil
}

func (tui *TerminalUserInterface) OnDeleteOrder(request events.DeleteOrderRequest) error {
	tui.ordersMu.Lock()
	defer tui.ordersMu.Unlock()
	order, exists := tui.orders[request.OrderID]
	if !exists || order.customerID != request.CustomerID {
		return fmt.Errorf("pedido %s não encontrado para este usuário nesta sessão", request.OrderID)
	}
	if !order.pending() {
		return fmt.Errorf("pedido %s já foi cancelado, recusado ou teve o pagamento aprovado", request.OrderID)
	}

	payload := order.payload
	payload.Status = statusCancelled
	if err := tui.publishOrderEvent(events.OrderEventDeleted, payload); err != nil {
		return err
	}
	order.payload = payload
	tui.orders[request.OrderID] = order
	return nil
}

func (tui *TerminalUserInterface) publishOrderEvent(event events.OrderEvent, payload events.PedidoPayload) error {
	if tui.publisher == nil {
		return fmt.Errorf("conexão de publicação indisponível")
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("preparar solicitação: %w", err)
	}

	checksum := sha256.Sum256(body)

	signature, err := rsa.SignPSS(rand.Reader, tui.privateKey, crypto.SHA256, checksum[:], nil)
	if err != nil {
		return fmt.Errorf("preparar assinatura: %w", err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	err = tui.publisher.PublishWithContext(ctx,
		exchanges.GetEcommerceExchangeInfo().Name,
		event.String(),
		false,
		false,
		amqp.Publishing{
			ContentType: "application/json",
			Headers:     amqp.Table{"x-signature": signature},
			Body:        body,
		},
	)
	if err != nil {
		return fmt.Errorf("enviar solicitação: %w", err)
	}
	return nil
}
