package mock

import ()

type Order struct {
	UUID     string
	Products []Product
}

type OrderStatus string

const (
	OrderStatusCreated         OrderStatus = "OrderCreated"
	OrderStatusPaymentDenied   OrderStatus = "PaymentDenied"
	OrderStatusPaymentApproved OrderStatus = "PaymentApproved"
	OrderStatusSent            OrderStatus = "OrderSent"
	OrderStatusStockReserved   OrderStatus = "StockReserved"
	OrderStatusOutOfStock      OrderStatus = "OutOfStock"
)

type OrderInfo struct {
	Order
	Status OrderStatus
}
