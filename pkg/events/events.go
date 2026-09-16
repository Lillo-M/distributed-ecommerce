package events

const (
	RoutingPedidoCriado      = "pedido.criado"
	RoutingPedidoEstoqueOk   = "pedido.estoque_ok"
	RoutingEstoqueIndisponivel = "estoque.indisponivel"
	RoutingPagamentoAprovado = "pagamento.aprovado"
	RoutingPagamentoRecusado = "pagamento.recusado"
	RoutingPedidoExcluido   = "pedido.excluido"
	RoutingPedidoEnviado     = "pedido.enviado"
)

type ItemPedido struct {
	ProductID string `json:"product_id"`
	Quantity  int    `json:"quantity"`
}

type PedidoPayload struct {
	ID     string       `json:"id"`
	Status string       `json:"status,omitempty"`
	Itens  []ItemPedido `json:"itens,omitempty"`
}

type PromoPayload struct {
	ID        string  `json:"id"`
	Categoria string  `json:"categoria"`
	Produto   string  `json:"produto"`
	Desconto  float64 `json:"desconto"`
}