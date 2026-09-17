package events

type PaymentEvent int

const (
	PaymentEventApproved PaymentEvent = iota
	PaymentEventRefused
)

var paymentEventName = map[PaymentEvent]string{
	PaymentEventApproved: RoutingPagamentoAprovado,
	PaymentEventRefused:  RoutingPagamentoRecusado,
}

func (payment PaymentEvent) String() string {
	return paymentEventName[payment]
}
