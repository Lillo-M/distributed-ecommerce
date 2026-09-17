package main

import (
	"encoding/json"
	"log"
	"math/rand"
	"time"

	"eCommerce/pkg/config"
	"eCommerce/pkg/events"
	"eCommerce/pkg/exchanges"
	"eCommerce/pkg/helpers"

	amqp "github.com/rabbitmq/amqp091-go"
)

func main() {
	rand.Seed(time.Now().UnixNano())
	cfg, err := config.Load()
	helpers.FailOnError(err, "Erro ao carregar configurações")

	conn, err := amqp.Dial(cfg.RabbitMQURL)
	helpers.FailOnError(err, "Erro ao conectar")
	defer conn.Close()

	ch, err := conn.Channel()
	helpers.FailOnError(err, "Erro no canal")
	defer ch.Close()

	ecommerceEx := exchanges.GetEcommerceExchangeInfo()
	err = ecommerceEx.Declare(ch)
	helpers.FailOnError(err, "Erro ao declarar exchange eCommerce")

	q, err := ch.QueueDeclare("fila.pagamento", false, false, false, false, nil)
	helpers.FailOnError(err, "Erro na fila")

	err = ch.QueueBind(q.Name, events.RoutingPedidoEstoqueOk, ecommerceEx.Name, false, nil)
	helpers.FailOnError(err, "Erro ao associar confirmação de estoque")

	msgs, err := ch.Consume(q.Name, "", true, false, false, false, nil)
	helpers.FailOnError(err, "Erro ao consumir")

	log.Println("[Pagamento] Serviço iniciado...")

	for d := range msgs {
		var p events.PedidoPayload
		if err := json.Unmarshal(d.Body, &p); err != nil {
			continue
		}

		if rand.Float32() < 0.8 {
			log.Printf("[Pagamento] Pedido %s - APROVADO", p.ID)
			publishEvent(ch, ecommerceEx.Name, events.RoutingPagamentoAprovado, p)
		} else {
			log.Printf("[Pagamento] Pedido %s - RECUSADO", p.ID)
			publishEvent(ch, ecommerceEx.Name, events.RoutingPagamentoRecusado, p)
		}
	}
}

func publishEvent(ch *amqp.Channel, exchange, rkey string, payload interface{}) {
	body, _ := json.Marshal(payload)
	_ = ch.Publish(exchange, rkey, false, false, amqp.Publishing{
		ContentType: "application/json",
		Body:        body,
	})
}
