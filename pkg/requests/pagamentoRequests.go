package requests

// POST do /cobrancas
type CreateCobrancaRequest struct {
	OrderId    string `json:"orderId"`
	WebhookUrl string `json:"webhookUrl"`
}

// Response do POST /cobrancas
type CreateCobrancaResponse struct {
	OrderId     string `json:"orderId"`
	CheckoutUrl string `json:"checkoutUrl"`
}

type PagamentoWebhookRequest struct {
	OrderId string `json:"orderId"`
	Status  string `json:"status"`
}
