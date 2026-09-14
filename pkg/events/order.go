package events

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
