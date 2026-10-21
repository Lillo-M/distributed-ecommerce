package main

import (
	"context"
	"crypto"
	crand "crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"eCommerce/pkg/events"
	"eCommerce/pkg/exchanges"
	"eCommerce/pkg/helpers"
	"eCommerce/pkg/interfaces"
	"eCommerce/pkg/notifications"
	"encoding/json"
	"log"
	"time"
	"bytes"
	"io"
	"net/http"
	"strings"
	"eCommerce/pkg/requests"
	
	"github.com/gin-gonic/gin"
	amqp "github.com/rabbitmq/amqp091-go"
)

type PaymentService struct {
	interfaces.Service
	httpClient *http.Client
}

const (
	paymentPrivateKeyDirectory = "./services/payment/payment.pem"
)

func CreatePaymentService() (*PaymentService, error) {
	var payment *PaymentService = &PaymentService{}
	var err error

	err = payment.InitRMQChannel()
	if err != nil {
		helpers.LogErrorMessage(err, "Failed to init RabbitMQ Channel")
		return nil, err
	}

	var ecommerceExchange = exchanges.GetEcommerceExchangeInfo()
	err = payment.Channel.ExchangeDeclare(
		ecommerceExchange.Name, // name
		ecommerceExchange.Type, // type
		false,                  // durability
		false,                  // auto-deleted
		false,                  // internal
		false,                  // no-wait
		nil,                    // arguments
	)

	if err != nil {
		helpers.LogErrorMessage(err, "Failed to declare a exchange")
		payment.CloseRMQChannel()
		return nil, err
	}

	routingKeys := []string{
		events.OrderStoreOkEvent.String(),
	}

	err = payment.BindRoutingKeys(routingKeys, ecommerceExchange)
	if err != nil {
		helpers.LogErrorMessage(err, "Failed to declare a exchange")
		payment.CloseRMQChannel()
		return nil, err
	}
	payment.httpClient = &http.Client{
		Timeout: 10 * time.Second,
	}
	payment.StartConsuming(payment.HandleMessages)

	return payment, nil
}

func (this *PaymentService) HandleMessages(message amqp.Delivery) {
	producerKey, err := helpers.GetProducerPublicKey(message)
	if err != nil {
		log.Printf("Failed to read the producer's public key routing key '%v': %v", message.RoutingKey, err)
		return
	}
	err = helpers.VerifyMessage(message, producerKey)
	if err != nil {
		log.Printf("Failed to validate the producer's public key of the event '%v': %v", message.RoutingKey, err)
		return
	}

	switch message.RoutingKey {
	case events.OrderStoreOkEvent.String():
		this.handleStoreOk(message)
		log.Printf("Received a store ok: %s", message.Body)
	default:
		log.Fatalf("Unknown routing key '%v'\n", message.RoutingKey)
	}
}

func (this *PaymentService) handleStoreOk(message amqp.Delivery) {
	notification := notifications.StoreAvailability{}
	err := json.Unmarshal(message.Body, &notification)
	if err != nil {
		log.Printf("Failed to decode notification '%v': %v\n", string(message.Body), err)
		return
	}
	// 1. Monta o payload de cobrança avisando onde está o nosso Webhook
	chargeRequest := requests.CreateCobrancaRequest{
		OrderId:    notification.OrderId,
		WebhookUrl: "http://" + this.Config.PaymentURL + "/webhook",
	}
	requestBytes, err := json.Marshal(chargeRequest)
	if err != nil {
		log.Printf("Failed to serialize charge request: %v\n", err)
		return
	}
	// 2. Dispara o POST no Mock de Pagamento para criar a cobrança
	mockEndpoint := "http://" + this.Config.PaymentMockURL + "/charges"
	resp, err := this.httpClient.Post(mockEndpoint, "application/json", bytes.NewBuffer(requestBytes))
	if err != nil {
		log.Printf("Failed to call payment mock at '%s': %v\n", mockEndpoint, err)
		return
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		log.Printf("Failed to read payment mock response: %v\n", err)
		return
	}
	if resp.StatusCode != http.StatusCreated && resp.StatusCode != http.StatusOK {
		log.Printf("Payment mock returned status %d: %s\n", resp.StatusCode, string(body))
		return
	}
	// 3. Lê a resposta do Mock (que contém a CheckoutUrl)
	var chargeResponse requests.CreateCobrancaResponse
	if err := json.Unmarshal(body, &chargeResponse); err != nil {
		log.Printf("Failed to decode payment mock response: %v\n", err)
		return
	}
	log.Printf("Checkout created for order '%s': %s\n", chargeResponse.OrderId, chargeResponse.CheckoutUrl)
	// 4. Publica no RabbitMQ avisando que a URL de checkout está pronta para o cliente abrir
	this.sendNotification(
		notifications.PaymentStatus{
			OrderId:     notification.OrderId,
			CheckoutUrl: chargeResponse.CheckoutUrl,
		},
		events.PaymentEventCheckoutCreated,
	)
}

func (this *PaymentService) PostWebhook(context *gin.Context) {
	var request requests.PagamentoWebhookRequest
	if err := context.BindJSON(&request); err != nil {
		context.IndentedJSON(http.StatusBadRequest, gin.H{
			"success": false,
			"error": err.Error(),
		})
		return
	}
	log.Printf("Webhook received for order '%s' with status: %s\n", request.OrderId, request.Status)

	switch strings.ToUpper(strings.TrimSpace(request.Status)) {
	case "APROVADO":
		this.sendNotification(
			notifications.PaymentStatus{OrderId: request.OrderId},
			events.PaymentEventApproved,
		)
		context.IndentedJSON(http.StatusOK, gin.H{
			"success": true,
			"status":  "APROVADO",
		})
	case "RECUSADO":
		this.sendNotification(
			notifications.PaymentStatus{OrderId: request.OrderId},
			events.PaymentEventRefused,
		)
		context.IndentedJSON(http.StatusOK, gin.H{
			"success": true,
			"status":  "RECUSADO",
		})
	
	default:
		context.IndentedJSON(http.StatusBadRequest, gin.H{
			"success": false,
			"error":   "status tem que ser APROVADO ou RECUSADO",
		})
	}
}

func (this *PaymentService) sendNotification(object any, event interfaces.Event) {
	notification, err := json.Marshal(object)
	if err != nil {
		log.Printf("failed to serialize request: %v\n", err)
		return
	}

	privateKey, err := helpers.ReadPrivateKeyPEM(paymentPrivateKeyDirectory)
	if err != nil {
		log.Fatalf("Failed to read private key: %v", err)
	}

	checksum := sha256.Sum256(notification)

	signature, err := rsa.SignPSS(crand.Reader, privateKey, crypto.SHA256, checksum[:], nil)
	if err != nil {
		log.Fatalf("Failed to sign payload: %v", err)
	}
	this.SendToECommerceExchange(
		amqp.Publishing{
			ContentType: "application/json",
			Headers:     amqp.Table{"x-signature": signature},
			Body:        notification,
		},
		event,
	)
}

func (this *PaymentService) SendToECommerceExchange(publishing amqp.Publishing, routingKey interfaces.Event) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	var err = this.Channel.PublishWithContext(ctx,
		exchanges.GetEcommerceExchangeInfo().Name, // exchange
		routingKey.String(),                       // routing key
		false,                                     // mandatory
		false,                                     // immediate
		publishing,
	)
	helpers.FailOnError(err, "Failed to publish a message")
	log.Printf(" [x] Sent %s\n", publishing.Body)
}
