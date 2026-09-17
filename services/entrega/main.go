package main

import (
	"crypto"
	crand "crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
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
		var p events.PedidoPayload
		if err := json.Unmarshal(message.Body, &p); err != nil {
			continue
		}

		log.Printf("[Entrega] Emitindo NF e preparando despacho do Pedido %s...", p.ID)
		publishEvent(ch, ecommerceEx.Name, events.RoutingPedidoEnviado, p)
	}
}

func publishEvent(ch *amqp.Channel, exchange, rkey string, payload interface{}) {
	body, _ := json.Marshal(payload)
	privateKey, err := helpers.ReadPrivateKeyPEM("./pkg/private-keys/delivery.pem")
	if err != nil {
		log.Fatalf("Erro ao ler chave privada: %v", err)
	}

	checksum := sha256.Sum256(body)

	signature, err := rsa.SignPSS(crand.Reader, privateKey, crypto.SHA256, checksum[:], nil)
	if err != nil {
		log.Fatalf("preparar assinatura: %v", err)
	}

	_ = ch.Publish(exchange, rkey, false, false, amqp.Publishing{
		ContentType: "application/json",
		Headers:     amqp.Table{"x-signature": signature},
		Body:        body,
	})
}
