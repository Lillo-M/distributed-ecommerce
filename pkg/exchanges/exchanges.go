package exchanges

type ExchangeInfo struct {
	Name string
	Type string
}

func GetEcommerceExchangeInfo() ExchangeInfo {
	return ExchangeInfo{
		Name: "eCommerce",
		Type: "direct",
	}
}

func GetSalesExchangeInfo() ExchangeInfo {
	return ExchangeInfo{
		Name: "sales",
		Type: "topic",
	}
}
