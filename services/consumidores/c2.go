package main

import (
	"encoding/json"
	"log"

	"eCommerce/pkg/config"
	"eCommerce/pkg/events"
	"eCommerce/pkg/exchanges"

	amqp "github.com/rabbitmq/amqp091-go"
)

func main() {
	cfg, _ := config.Load()
	conn, _ := amqp.Dial(cfg.RabbitMQURL)
	ch, _ := conn.Channel()

	salesEx := exchanges.GetSalesExchangeInfo()
	q, _ := ch.QueueDeclare("fila.c2", false, false, false, false, nil)

	_ = ch.QueueBind(q.Name, "promocao.categoria.*", salesEx.Name, false, nil)

	msgs, _ := ch.Consume(q.Name, "", true, false, false, false, nil)
	log.Println("[Consumidor C2] Escutando TODAS as categorias (*)...")

	for d := range msgs {
		var p events.PromoPayload
		if err := json.Unmarshal(d.Body, &p); err == nil {
			log.Printf("[C2] Promoção recebida: Cat %s | Desc: %.0f%%", p.Categoria, p.Desconto)
		}
	}
}