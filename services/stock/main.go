package main

import (
	"encoding/json"
	"log"
	"os"
	"strings"
	"sync"

	"eCommerce/pkg/config"
	"eCommerce/pkg/events"
	"eCommerce/pkg/exchanges"
	"eCommerce/pkg/helpers"

	amqp "github.com/rabbitmq/amqp091-go"
)

type Product = events.Product

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
	err = ecommerceEx.Declare(ch)
	helpers.FailOnError(err, "Erro ao declarar exchange eCommerce")

	q, err := ch.QueueDeclare("fila.estoque", false, false, false, false, nil)
	helpers.FailOnError(err, "Erro na fila")

	for _, key := range []string{events.RoutingPedidoCriado, events.RoutingPedidoExcluido, events.RoutingProdutosConsultar} {
		err = ch.QueueBind(q.Name, key, ecommerceEx.Name, false, nil)
		helpers.FailOnError(err, "Erro ao associar evento "+key)
	}

	msgs, err := ch.Consume(q.Name, "", true, false, false, false, nil)
	helpers.FailOnError(err, "Erro no consume")

	log.Println("[Estoque] Serviço pronto e aguardando eventos...")

	for d := range msgs {
		if d.RoutingKey == events.RoutingProdutosConsultar {
			if !strings.HasPrefix(d.ReplyTo, events.RoutingProdutosListados+".") || d.CorrelationId == "" {
				log.Println("[Estoque] Consulta de produtos sem destino de resposta válido")
				continue
			}
			mu.Lock()
			body, err := json.Marshal(events.ProductsPayload{Products: products})
			mu.Unlock()
			if err == nil {
				err = ch.Publish(ecommerceEx.Name, d.ReplyTo, false, false, amqp.Publishing{
					ContentType:   "application/json",
					CorrelationId: d.CorrelationId,
					Body:          body,
				})
			}
			if err != nil {
				log.Printf("[Estoque] Erro ao responder consulta de produtos: %v", err)
			}
			continue
		}

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
