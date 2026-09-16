package main

import (
	"encoding/json"
	"log"
	"os"
	"sync"

	"eCommerce/pkg/config"
	"eCommerce/pkg/events"
	"eCommerce/pkg/exchanges"
	"eCommerce/pkg/helpers"

	amqp "github.com/rabbitmq/amqp091-go"
)

type Product struct {
	UUID     string  `json:"uuid"`
	Name     string  `json:"name"`
	Price    float64 `json:"price"`
	Quantity int     `json:"quantity"`
}

var (
	stockFile = "stock.json"
	products  []Product
	mu        sync.Mutex
)

func loadStock() {
	data, err := os.ReadFile(stockFile)
	if err == nil {
		_ = json.Unmarshal(data, &products)
	}
}

func saveStock() {
	data, _ := json.MarshalIndent(products, "", "  ")
	_ = os.WriteFile(stockFile, data, 0644)
}

func main() {
	cfg, err := config.Load()
	helpers.FailOnError(err, "Erro ao carregar configurações")

	loadStock()

	conn, err := amqp.Dial(cfg.RabbitMQURL)
	helpers.FailOnError(err, "Erro no RabbitMQ")
	defer conn.Close()

	ch, err := conn.Channel()
	helpers.FailOnError(err, "Erro no canal")
	defer ch.Close()

	ecommerceEx := exchanges.GetEcommerceExchangeInfo()
	q, err := ch.QueueDeclare("fila.estoque", false, false, false, false, nil)
	helpers.FailOnError(err, "Erro na fila")

	_ = ch.QueueBind(q.Name, events.RoutingPedidoCriado, ecommerceEx.Name, false, nil)
	_ = ch.QueueBind(q.Name, events.RoutingPedidoExcluido, ecommerceEx.Name, false, nil)

	msgs, err := ch.Consume(q.Name, "", true, false, false, false, nil)
	helpers.FailOnError(err, "Erro no consume")

	log.Println("[Estoque] Serviço pronto e aguardando eventos...")

	for d := range msgs {
		var p events.PedidoPayload
		if err := json.Unmarshal(d.Body, &p); err != nil {
			continue
		}

		mu.Lock()
		if d.RoutingKey == events.RoutingPedidoCriado {
			available := true
			for _, item := range p.Itens {
				found := false
				for _, prod := range products {
					if prod.UUID == item.ProductID && prod.Quantity >= item.Quantity {
						found = true
						break
					}
				}
				if !found {
					available = false
					break
				}
			}

			if available {
				for _, item := range p.Itens {
					for i := range products {
						if products[i].UUID == item.ProductID {
							products[i].Quantity -= item.Quantity
						}
					}
				}
				saveStock()
				log.Printf("[Estoque] Pedido %s - Estoque OK", p.ID)
				publishEvent(ch, ecommerceEx.Name, events.RoutingPedidoEstoqueOk, p)
			} else {
				log.Printf("[Estoque] Pedido %s - Estoque Indisponível", p.ID)
				publishEvent(ch, ecommerceEx.Name, events.RoutingEstoqueIndisponivel, p)
			}
		} else if d.RoutingKey == events.RoutingPedidoExcluido {
			for _, item := range p.Itens {
				for i := range products {
					if products[i].UUID == item.ProductID {
						products[i].Quantity += item.Quantity
					}
				}
			}
			saveStock()
			log.Printf("[Estoque] Pedido %s - Produtos estornados ao estoque", p.ID)
		}
		mu.Unlock()
	}
}

func publishEvent(ch *amqp.Channel, exchange, rkey string, payload interface{}) {
	body, _ := json.Marshal(payload)
	_ = ch.Publish(exchange, rkey, false, false, amqp.Publishing{
		ContentType: "application/json",
		Body:        body,
	})
}