package events

type OrderEvent int

const (
	OrderEventCreated OrderEvent = iota
	OrderEventDeleted
	OrderEventStoreOk
	OrderEventSent
)

var orderEventName = map[OrderEvent]string{
	OrderEventCreated: "Order.Created",
	OrderEventDeleted: "Order.Deleted",
	OrderEventStoreOk: "Order.StockOk",
	OrderEventSent:    "Order.Sent",
}

func (order OrderEvent) String() string {
	return orderEventName[order]
}
