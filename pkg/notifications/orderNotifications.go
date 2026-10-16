package notifications

import "eCommerce/pkg/dtos"

type CreateOrderNotification struct {
	Id       string
	Products []dtos.ProductDto
}

type DeleteOrderNotification struct {
	OrderId string
}
