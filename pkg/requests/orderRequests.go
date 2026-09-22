package requests

import "eCommerce/pkg/dtos"

type CreateOrderRequest struct {
	Id string
	Products []dtos.ProductDto
}
