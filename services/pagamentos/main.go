package main

import (
	"crypto"
	crand "crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"encoding/json"
	"log"
	"math/rand"

	"eCommerce/pkg/config"
	"eCommerce/pkg/events"
	"eCommerce/pkg/exchanges"
	"eCommerce/pkg/helpers"

	amqp "github.com/rabbitmq/amqp091-go"
)

func main() {
	rand.NewSource(0)
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

		if rand.Float32() < 0.5 {
			log.Printf("[Pagamento] Pedido %s - APROVADO", p.ID)
			publishEvent(ch, ecommerceEx.Name, events.RoutingPagamentoAprovado, p)
		} else {
			log.Printf("[Pagamento] Pedido %s - RECUSADO", p.ID)
			publishEvent(ch, ecommerceEx.Name, events.RoutingPagamentoRecusado, p)
		}
	}
}

func publishEvent(ch *amqp.Channel, exchange, rkey string, payload events.PedidoPayload) {
	body, _ := json.Marshal(payload)
	privateKey, err := helpers.ReadPrivateKeyPEM("./pkg/private-keys/payments.pem")
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
