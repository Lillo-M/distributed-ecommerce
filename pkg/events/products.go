package events

const (
	RoutingProdutosConsultar = "produtos.consultar"
	RoutingProdutosListados  = "produtos.listados"
)

type Product struct {
	UUID     string  `json:"uuid"`
	Name     string  `json:"name"`
	Price    float64 `json:"price"`
	Quantity int     `json:"quantity"`
}

type ProductsPayload struct {
	Products []Product `json:"produtos"`
}
