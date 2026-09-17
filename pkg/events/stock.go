package events

type StockEvent int

const (
	StockEventUnavailable StockEvent = iota
)

var stockEventName = map[StockEvent]string{
	StockEventUnavailable: RoutingEstoqueIndisponivel,
}

func (stock StockEvent) String() string {
	return stockEventName[stock]
}
