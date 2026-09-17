package main

import (
	"crypto"
	crand "crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"encoding/json"
	"fmt"
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
	rand.NewSource(0)
	cfg, err := config.Load()
	helpers.FailOnError(err, "Erro de config")

	conn, err := amqp.Dial(cfg.RabbitMQURL)
	helpers.FailOnError(err, "Erro na conexão")
	defer conn.Close()

	ch, err := conn.Channel()
	helpers.FailOnError(err, "Erro no canal")
	defer ch.Close()

	salesEx := exchanges.GetSalesExchangeInfo()
	err = salesEx.Declare(ch)
	helpers.FailOnError(err, "Erro ao declarar exchange de promoções")

	cats := []string{"A", "B", "C"}

	for {
		cat := cats[rand.Intn(len(cats))]
		routingKey := fmt.Sprintf("promocao.categoria.%s", cat)

		promo := events.PromoPayload{
			ID:        fmt.Sprintf("%d", rand.Intn(1000)),
			Categoria: cat,
			Produto:   fmt.Sprintf("Produto Exemplo %s", cat),
			Desconto:  float64(rand.Intn(50) + 10),
		}

		body, _ := json.Marshal(promo)

		privateKey, err := helpers.ReadPrivateKeyPEM("./pkg/private-keys/stock.pem")
		if err != nil {
			log.Fatalf("Erro ao ler chave privada: %v", err)
		}

		checksum := sha256.Sum256(body)

		signature, err := rsa.SignPSS(crand.Reader, privateKey, crypto.SHA256, checksum[:], nil)
		if err != nil {
			log.Fatalf("preparar assinatura: %v", err)
		}

		_ = ch.Publish(salesEx.Name, routingKey, false, false, amqp.Publishing{
			ContentType: "application/json",
			Headers:     amqp.Table{"x-signature": signature},
			Body:        body,
		})

		log.Printf("[Promoções] Publicada oferta na key '%s'", routingKey)
		time.Sleep(5 * time.Second)
	}
}
