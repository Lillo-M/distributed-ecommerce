package events

type SaleEvent int

const (
	SaleCategoryA SaleEvent = iota
	SaleCategoryB
	SaleCategoryC
	SaleInterest
)

var saleEventName = map[SaleEvent]string{
	SaleCategoryA: "Sale.Category.A",
	SaleCategoryB: "Sale.Category.B",
	SaleCategoryC: "Sale.Category.C",
	SaleInterest:  "Sale.Interest",
}

func (sale SaleEvent) String() string {
	return saleEventName[sale]
}
