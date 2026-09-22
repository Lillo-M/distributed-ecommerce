package events

type OrderEvent int

const (
	OrderCreatedEvent OrderEvent = iota
	OrderDeletedEvent
	OrderStoreOkEvent
	OrderSentEvent
)

var orderEventName = map[OrderEvent]string{
	OrderCreatedEvent: "Order.Created",
	OrderDeletedEvent: "Order.Deleted",
	OrderStoreOkEvent: "Order.StockOk",
	OrderSentEvent:    "Order.Sent",
}

func (order OrderEvent) String() string {
	return orderEventName[order]
}
