package main

import (
	"fmt"
	"io"
	"sort"

	"eCommerce/pkg/events"
)

const (
	statusCreated          = "Criado"
	statusStockReserved    = "Estoque Reservado / Pagamento Pendente"
	statusPaymentApproved  = "Pagamento Aprovado"
	statusSent             = "Enviado"
	statusStockUnavailable = "Cancelado (Estoque Indisponível)"
	statusPaymentRefused   = "Cancelado (Pagamento Recusado)"
	statusCancelled        = "Cancelado Manualmente"
)

func (order sessionOrder) pending() bool {
	return order.payload.Status == statusCreated || order.payload.Status == statusStockReserved
}

func (tui *TerminalUserInterface) ordersForCustomer(customerID string) []events.PedidoPayload {
	tui.ordersMu.Lock()
	defer tui.ordersMu.Unlock()
	var orders []events.PedidoPayload
	for _, order := range tui.orders {
		if order.customerID == customerID {
			payload := order.payload
			payload.Itens = append([]events.ItemPedido(nil), payload.Itens...)
			orders = append(orders, payload)
		}
	}
	sort.Slice(orders, func(i, j int) bool { return orders[i].ID < orders[j].ID })
	return orders
}

func (tui *TerminalUserInterface) updateOrderStatus(orderID, routingKey string) (string, bool) {
	tui.ordersMu.Lock()
	defer tui.ordersMu.Unlock()
	order, exists := tui.orders[orderID]
	if !exists {
		// Os eventos de outros usuários não adicionam pedidos à sessão atual.
		return "", false
	}

	status := order.payload.Status
	switch routingKey {
	case events.RoutingPedidoEstoqueOk:
		if status == statusCreated {
			status = statusStockReserved
		}
	case events.RoutingPagamentoAprovado:
		if order.pending() {
			status = statusPaymentApproved
		}
	case events.RoutingPedidoEnviado:
		// O envio pode chegar antes da aprovação porque os produtores são distintos.
		if order.pending() || status == statusPaymentApproved {
			status = statusSent
		}
	case events.RoutingEstoqueIndisponivel:
		if order.pending() {
			status = statusStockUnavailable
		}
	case events.RoutingPagamentoRecusado:
		if order.pending() {
			status = statusPaymentRefused
		}
	}
	if status == order.payload.Status {
		return status, false
	}
	order.payload.Status = status
	tui.orders[orderID] = order
	return status, true
}

func displayOrders(output io.Writer, orders []events.PedidoPayload) error {
	if len(orders) == 0 {
		_, err := fmt.Fprintln(output, "Nenhum pedido encontrado para este usuário nesta sessão.")
		return err
	}
	if _, err := fmt.Fprintln(output, "\n--- Meus pedidos ---"); err != nil {
		return err
	}
	for _, order := range orders {
		if _, err := fmt.Fprintf(output, "Pedido: %s\nStatus: %s\n", order.ID, order.Status); err != nil {
			return err
		}
		for _, item := range order.Itens {
			if _, err := fmt.Fprintf(output, "  Produto %s: %d unidade(s)\n", item.ProductID, item.Quantity); err != nil {
				return err
			}
		}
	}
	return nil
}
