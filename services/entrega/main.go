package main

import (
	"encoding/json"
	"log"

	"eCommerce/pkg/config"
	"eCommerce/pkg/events"
	"eCommerce/pkg/exchanges"
	"eCommerce/pkg/helpers"

	amqp "github.com/rabbitmq/amqp091-go"
)

func main() {
	cfg, err := config.Load()
	helpers.FailOnError(err, "Erro nas configurações")

	conn, err := amqp.Dial(cfg.RabbitMQURL)
	helpers.FailOnError(err, "Erro no RabbitMQ")
	defer conn.Close()

	ch, err := conn.Channel()
	helpers.FailOnError(err, "Erro no canal")
	defer ch.Close()

	ecommerceEx := exchanges.GetEcommerceExchangeInfo()
	err = ecommerceEx.Declare(ch)
	helpers.FailOnError(err, "Erro ao declarar exchange eCommerce")

	q, err := ch.QueueDeclare("fila.entrega", false, false, false, false, nil)
	helpers.FailOnError(err, "Erro na fila")

	err = ch.QueueBind(q.Name, events.RoutingPagamentoAprovado, ecommerceEx.Name, false, nil)
	helpers.FailOnError(err, "Erro ao associar pagamento aprovado")

	msgs, err := ch.Consume(q.Name, "", true, false, false, false, nil)
	helpers.FailOnError(err, "Erro no consumer")

	log.Println("[Entrega] Serviço aguardando novos pagamentos...")

	for d := range msgs {
		var p events.PedidoPayload
		if err := json.Unmarshal(d.Body, &p); err != nil {
			continue
		}

		log.Printf("[Entrega] Emitindo NF e preparando despacho do Pedido %s...", p.ID)
		publishEvent(ch, ecommerceEx.Name, events.RoutingPedidoEnviado, p)
	}
}

func publishEvent(ch *amqp.Channel, exchange, rkey string, payload interface{}) {
	body, _ := json.Marshal(payload)
	_ = ch.Publish(exchange, rkey, false, false, amqp.Publishing{
		ContentType: "application/json",
		Body:        body,
	})
}
