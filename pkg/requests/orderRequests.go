package requests

import "eCommerce/pkg/dtos"

// TODO: the request from the front shouldn't have a Id field
// since it's the back-end responsability to create the Id
type CreateOrderRequest struct {
	Id       string
	Products []dtos.ProductDto
}

type DeleteOrderRequest struct {
	OrderId string
}
