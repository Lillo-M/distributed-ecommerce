package events

type PaymentEvent int

const (
	PaymentEventApproved PaymentEvent = iota
	PaymentEventRefused
	PaymentEventCheckoutCreated
)

var paymentEventName = map[PaymentEvent]string{
	PaymentEventApproved:        "Payment.Approved",
	PaymentEventRefused:         "Payment.Refused",
	PaymentEventCheckoutCreated: "Payment.CheckoutCreated",
}

func (payment PaymentEvent) String() string {
	return paymentEventName[payment]
}
