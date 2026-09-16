package exchanges

type exchangeInfo struct {
	Name         string
	Type string
}

func GetEcommerceExchangeInfo() exchangeInfo {
	return exchangeInfo{
		Name:         "eCommerce",
		Type: "direct",
	}
}

func GetSalesExchangeInfo() exchangeInfo {
	return exchangeInfo{
		Name:         "sales",
		Type: "topic",
	}
}
