package events

type StoreEvent int

const (
	StoreUnavailableEvent StoreEvent = iota
)

var storeEventName = map[StoreEvent]string{
	StoreUnavailableEvent: "Store.Unavailable",
}

func (stock StoreEvent) String() string {
	return storeEventName[stock]
}
