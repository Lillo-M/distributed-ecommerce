package events

type StoreEvent int

const (
	StoreEventUnavailable StoreEvent = iota
)

var storeEventName = map[StoreEvent]string{
	StoreEventUnavailable: "Store.Unavailable",
}

func (stock StoreEvent) String() string {
	return storeEventName[stock]
}
