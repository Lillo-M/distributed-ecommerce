package events

type PaymentEvent int

const (
	PaymentEventApproved PaymentEvent = iota
	PaymentEventRefused
)

var paymentEventName = map[PaymentEvent]string{
	PaymentEventApproved: "Payment.Approved",
	PaymentEventRefused:  "Payment.Refused",
}

func (payment PaymentEvent) String() string {
	return paymentEventName[payment]
}
