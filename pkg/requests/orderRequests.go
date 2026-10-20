package requests

import "eCommerce/pkg/dtos"

type CreateOrderRequest struct {
	Products []dtos.ProductDto
}

type DeleteOrderRequest struct {
	OrderId string
}
