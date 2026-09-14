package events

type StockEvent int

const (
	StockEventUnavailable StockEvent = iota
)

var stockEventName = map[StockEvent]string{
	StockEventUnavailable: "Stock.Unavailable",
}

func (stock StockEvent) String() string {
	return stockEventName[stock]
}
