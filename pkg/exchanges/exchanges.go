package exchanges

import amqp "github.com/rabbitmq/amqp091-go"

type exchangeInfo struct {
	Name string
	Type string
}

// Todos os serviços declaram a mesma configuração antes de usar a exchange.
func (exchange exchangeInfo) Declare(channel *amqp.Channel) error {
	return channel.ExchangeDeclare(exchange.Name, exchange.Type, false, false, false, false, nil)
}

func GetEcommerceExchangeInfo() exchangeInfo {
	return exchangeInfo{
		Name: "eCommerce",
		Type: "direct",
	}
}

func GetSalesExchangeInfo() exchangeInfo {
	return exchangeInfo{
		Name: "Promoções",
		Type: "topic",
	}
}
