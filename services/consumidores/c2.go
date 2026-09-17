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
	helpers.FailOnError(err, "[C2] Erro ao carregar configurações")
	conn, err := amqp.Dial(cfg.RabbitMQURL)
	helpers.FailOnError(err, "[C2] Erro ao conectar ao RabbitMQ")
	defer conn.Close()
	ch, err := conn.Channel()
	helpers.FailOnError(err, "[C2] Erro ao abrir canal")
	defer ch.Close()

	salesEx := exchanges.GetSalesExchangeInfo()
	err = salesEx.Declare(ch)
	helpers.FailOnError(err, "[C2] Erro ao declarar exchange de promoções")
	q, err := ch.QueueDeclare("fila.c2", false, false, false, false, nil)
	helpers.FailOnError(err, "[C2] Erro ao declarar fila")

	err = ch.QueueBind(q.Name, "promocao.categoria.*", salesEx.Name, false, nil)
	helpers.FailOnError(err, "[C2] Erro ao associar categorias")

	msgs, err := ch.Consume(q.Name, "", true, false, false, false, nil)
	helpers.FailOnError(err, "[C2] Erro ao consumir promoções")
	log.Println("[Consumidor C2] Escutando TODAS as categorias (*)...")

	for message := range msgs {
		producerKey, err := helpers.GetProducerPublicKey(message)
		if err != nil {
			log.Printf("Erro ao ler chave publica do producer de %v: %v", message.RoutingKey, err)
			continue
		}

		err = helpers.VerifyMessage(message, producerKey)
		if err != nil {
			log.Printf("Falha ao validar assinatura do producer do evento %v: %v", message.RoutingKey, err)
			continue
		}
		var p events.PromoPayload
		if err := json.Unmarshal(message.Body, &p); err == nil {
			log.Printf("[C2] Promoção recebida: Cat %s | Desc: %.0f%%", p.Categoria, p.Desconto)
		}
	}
}
