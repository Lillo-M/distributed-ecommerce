package main

import (
	"eCommerce/pkg/events"
	"eCommerce/pkg/helpers"
	"log"
	"time"

	amqp "github.com/rabbitmq/amqp091-go"
)

func main() {
	sales, err := CreateSalesService()
	helpers.FailOnError(err, "Failed to create DeliveryService")
	defer sales.CloseRMQChannel()
	sales.generateSales()
	select {}
}

func test_registering_unregistering() {
	log.Println("Initializing Sales Service...")

	salesService, err := CreateSalesService()
	if err != nil {
		log.Fatalf("Failed to create sales service: %v", err)
	}

	log.Println("Simulating an incoming AMQP message to register a consumer...")

	// 1. Simulate an incoming RabbitMQ message to test HandleMessages & Registration
	mockNotification := `{"Category": "A", "Email": "muroni2015@gmail.com", "IsRegisting": true}`

	// Create a dummy AMQP delivery
	mockDelivery := amqp.Delivery{
		RoutingKey: events.SaleInterest.String(), // Assuming events.SaleInterest.String() evaluates to this
		Body:       []byte(mockNotification),
	}

	// Route the message
	salesService.HandleMessages(mockDelivery)
	log.Printf("Current Consumers for Category A: %+v\n", salesService.MailConsumers["A"])

	// 2. Start the automated sales generator in a background goroutine
	log.Println("Starting automated sales generator...")
	go salesService.generateSales()

	// 3. Block the main thread so the program doesn't exit immediately
	log.Println("Service is running. Press CTRL+C to exit.")
	time.Sleep(10 * time.Second)
	mockNotification = `{"Category": "A", "Email": "muroni2015@gmail.com", "IsRegisting": false}`

	// Create a dummy AMQP delivery
	mockDelivery = amqp.Delivery{
		RoutingKey: events.SaleInterest.String(), // Assuming events.SaleInterest.String() evaluates to this
		Body:       []byte(mockNotification),
	}

	// Route the message
	salesService.HandleMessages(mockDelivery)
	select {}
}
