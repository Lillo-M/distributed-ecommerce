package exchanges

type exchangeInfo struct {
	Name         string
	ExchangeType string
}

func GetEcommerceExchangeInfo() exchangeInfo {
	return exchangeInfo{
		Name:         "eCommerce",
		ExchangeType: "direct",
	}
}

func GetSalesExchangeInfo() exchangeInfo {
	return exchangeInfo{
		Name:         "sales",
		ExchangeType: "topic",
	}
}
