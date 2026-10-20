package main

import (
	"context"
	"crypto"
	"crypto/rand"
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
	"math/big"
	"slices"
	"strings"
	"time"

	amqp "github.com/rabbitmq/amqp091-go"
	resend "github.com/resend/resend-go/v4"
)

type MailConsumer struct {
	Email string
}

type SalesService struct {
	interfaces.Service
	MailConsumers map[string][]MailConsumer
}

const (
	salesPrivateKeyDirectory = "./services/sales/sales.pem"
)

func CreateSalesService() (*SalesService, error) {
	var sales *SalesService = &SalesService{}
	var err error

	err = sales.InitRMQChannel()
	if err != nil {
		helpers.LogErrorMessage(err, "Failed to init RabbitMQ Channel")
		return nil, err
	}

	var salesExchange = exchanges.GetSalesExchangeInfo()
	err = sales.Channel.ExchangeDeclare(
		salesExchange.Name, // name
		salesExchange.Type, // type
		false,              // durability
		false,              // auto-deleted
		false,              // internal
		false,              // no-wait
		nil,                // arguments
	)

	if err != nil {
		helpers.LogErrorMessage(err, "Failed to declare a exchange")
		sales.CloseRMQChannel()
		return nil, err
	}

	routingKeys := []string{
		events.SaleInterest.String(),
	}

	err = sales.BindRoutingKeys(routingKeys, salesExchange)
	if err != nil {
		helpers.LogErrorMessage(err, "Failed to declare a exchange")
		sales.CloseRMQChannel()
		return nil, err
	}

	sales.StartConsuming(sales.HandleMessages)

	sales.MailConsumers = make(map[string][]MailConsumer)

	return sales, nil
}

func (this *SalesService) RegisterMailConsumer(notification notifications.TrackSaleNotification) {
	// TODO: sanitize and validate this notification (check if the category exists)
	if this.MailConsumers[notification.Category] == nil {
		this.MailConsumers[notification.Category] = make([]MailConsumer, 0)
	}
	this.MailConsumers[notification.Category] = append(this.MailConsumers[notification.Category], MailConsumer{notification.Email})
}

func (this *SalesService) UnregisterMailConsumer(notification notifications.TrackSaleNotification) {
	// TODO: sanitize and validate this notification (check if the category exists)
	if len(this.MailConsumers[notification.Category]) <= 0 {
		return
	}
	s := slices.IndexFunc(this.MailConsumers[notification.Category],
		func(el MailConsumer) bool {
			return el.Email == notification.Email
		})
	if s == -1 {
		return
	}
	this.MailConsumers[notification.Category] = append(this.MailConsumers[notification.Category][:s],
		this.MailConsumers[notification.Category][s+1:]...)
}

func (this *SalesService) HandleMessages(message amqp.Delivery) {
	switch message.RoutingKey {
	case events.SaleInterest.String():
		log.Printf("Received an track sale: %s", message.Body)
		notification := notifications.TrackSaleNotification{}
		err := json.Unmarshal(message.Body, &notification)
		if err != nil {
			log.Fatalf("Failed to decode notification '%v'\n", message.Body)
		}
		if notification.IsRegisting {
			this.RegisterMailConsumer(notification)
		} else {
			this.UnregisterMailConsumer(notification)
		}
	default:
		log.Fatalf("Unknown routing key '%v'\n", message.RoutingKey)
	}
}

func (this *SalesService) generateSales() {
	for true {
		time.Sleep(time.Second)
		value, err := rand.Int(rand.Reader, big.NewInt(100))
		if err != nil {
			log.Fatalf("Failed to generate a random int '%v'\n", err)
			return
		}
		v := value.Int64()
		var notification notifications.SaleNotification
		switch {
		case v < 33:
			notification = notifications.SaleNotification{Category: strings.Split(events.SaleCategoryA.String(), ".")[2]}
			this.sendNotification(
				notification,
				events.SaleCategoryA,
			)
		case v >= 33 && v < 67:
			notification = notifications.SaleNotification{Category: strings.Split(events.SaleCategoryB.String(), ".")[2]}
			this.sendNotification(
				notification,
				events.SaleCategoryB,
			)
		case v >= 67 && v < 100:
			notification = notifications.SaleNotification{Category: strings.Split(events.SaleCategoryC.String(), ".")[2]}
			this.sendNotification(
				notification,
				events.SaleCategoryC,
			)
		default:
			log.Fatalf("Invalid rand output")
		}
		this.NotifyMailConsumers(notification)
	}
}

func (this *SalesService) NotifyMailConsumers(notification notifications.SaleNotification) {
	client := resend.NewClient(this.Config.EMailApiKey)

	for _, consumer := range this.MailConsumers[notification.Category] {
		params := &resend.SendEmailRequest{
			From:    "DistributedECommerce <onboarding@resend.dev>",
			To:      []string{consumer.Email},
			Subject: "Sale " + "Category " + notification.Category,
			Html:    "<p>The category <strong>" + notification.Category + "</strong> is on sale!</p>",
		}

		sent, err := client.Emails.Send(params)
		if err != nil {
			log.Fatalf("failed to send email: %v", err)
		}

		log.Printf("email sent: %s", sent.Id)
	}
}

func (this *SalesService) sendNotification(object any, event interfaces.Event) {
	notification, err := json.Marshal(object)
	if err != nil {
		log.Printf("failed to serialize request: %v\n", err)
		return
	}

	privateKey, err := helpers.ReadPrivateKeyPEM(salesPrivateKeyDirectory)
	if err != nil {
		log.Fatalf("Failed to read private key: %v", err)
	}

	checksum := sha256.Sum256(notification)

	signature, err := rsa.SignPSS(crand.Reader, privateKey, crypto.SHA256, checksum[:], nil)
	if err != nil {
		log.Fatalf("Failed to sign payload: %v", err)
	}
	this.SendToSalesExchange(
		amqp.Publishing{
			ContentType: "application/json",
			Headers:     amqp.Table{"x-signature": signature},
			Body:        notification,
		},
		event,
	)
}

func (this *SalesService) SendToSalesExchange(publishing amqp.Publishing, routingKey interfaces.Event) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	var err = this.Channel.PublishWithContext(ctx,
		exchanges.GetSalesExchangeInfo().Name, // exchange
		routingKey.String(),                   // routing key
		false,                                 // mandatory
		false,                                 // immediate
		publishing,
	)
	helpers.FailOnError(err, "Failed to publish a message")
	log.Printf(" [x] Sent %s\n", publishing.Body)
}
