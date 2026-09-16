package events

// OrderItem contém os dados informados no terminal para um produto do pedido.
type OrderItem struct {
	ProductID string `json:"product_id"`
	Quantity  int    `json:"quantity"`
}

type CreateOrderRequest struct {
	OrderID    string      `json:"order_id"`
	CustomerID string      `json:"customer_id"`
	Items      []OrderItem `json:"items"`
}

type DeleteOrderRequest struct {
	OrderID    string `json:"order_id"`
	CustomerID string `json:"customer_id"`
}

type OrderEvent int

const (
	OrderEventCreated OrderEvent = iota
	OrderEventDeleted
	OrderEventStockOk
	OrderEventSent
)

var orderEventName = map[OrderEvent]string{
	OrderEventCreated: "Order.Created",
	OrderEventDeleted: "Order.Deleted",
	OrderEventStockOk: "Order.StockOk",
	OrderEventSent:    "Order.Sent",
}

func (order OrderEvent) String() string {
	return orderEventName[order]
}
