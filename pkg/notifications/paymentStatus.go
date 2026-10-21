package notifications

type PaymentStatus struct {
	OrderId     string `json:"orderId"`
	CheckoutUrl string `json:"checkoutUrl,omitempty"`
}
