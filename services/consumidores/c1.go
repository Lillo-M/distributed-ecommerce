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
	helpers.FailOnError(err, "[C1] Erro ao carregar configurações")
	conn, err := amqp.Dial(cfg.RabbitMQURL)
	helpers.FailOnError(err, "[C1] Erro ao conectar ao RabbitMQ")
	defer conn.Close()
	ch, err := conn.Channel()
	helpers.FailOnError(err, "[C1] Erro ao abrir canal")
	defer ch.Close()

	salesEx := exchanges.GetSalesExchangeInfo()
	err = salesEx.Declare(ch)
	helpers.FailOnError(err, "[C1] Erro ao declarar exchange de promoções")
	q, err := ch.QueueDeclare("fila.c1", false, false, false, false, nil)
	helpers.FailOnError(err, "[C1] Erro ao declarar fila")

	for _, key := range []string{"promocao.categoria.A", "promocao.categoria.B"} {
		err = ch.QueueBind(q.Name, key, salesEx.Name, false, nil)
		helpers.FailOnError(err, "[C1] Erro ao associar categoria "+key)
	}

	msgs, err := ch.Consume(q.Name, "", true, false, false, false, nil)
	helpers.FailOnError(err, "[C1] Erro ao consumir promoções")
	log.Println("[Consumidor C1] Escutando categorias A e B...")

	for d := range msgs {
		var p events.PromoPayload
		if err := json.Unmarshal(d.Body, &p); err == nil {
			log.Printf("[C1] Promoção recebida: Cat %s | Desc: %.0f%%", p.Categoria, p.Desconto)
		}
	}
}
