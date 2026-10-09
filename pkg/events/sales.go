package events

type SaleEvent int

const (
	SaleCategoryA SaleEvent = iota
	SaleCategoryB
	SaleCategoryC
)

var saleEventName = map[SaleEvent]string{
	SaleCategoryA: "Sale.Category.A",
	SaleCategoryB: "Sale.Category.B",
	SaleCategoryC: "Sale.Category.C",
}

func (sale SaleEvent) String() string {
	return saleEventName[sale]
}
