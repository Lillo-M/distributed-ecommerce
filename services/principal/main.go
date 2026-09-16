package main

import (
	"bufio"
	"encoding/json"
	"fmt"
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

var (
	pedidosMap = make(map[string]*events.PedidoPayload)
	mu         sync.Mutex
)

func main() {
	cfg, err := config.Load()
	helpers.FailOnError(err, "Erro ao carregar configurações")

	conn, err := amqp.Dial(cfg.RabbitMQURL)
	helpers.FailOnError(err, "Erro ao conectar ao RabbitMQ")
	defer conn.Close()

	ch, err := conn.Channel()
	helpers.FailOnError(err, "Erro ao abrir canal")
	defer ch.Close()

	ecommerceEx := exchanges.GetEcommerceExchangeInfo()
	_ = ch.ExchangeDeclare(ecommerceEx.Name, ecommerceEx.Type, false, false, false, false, nil)

	q, err := ch.QueueDeclare("fila.principal", false, false, false, false, nil)
	helpers.FailOnError(err, "Erro ao declarar fila")

	keysToConsume := []string{
		events.RoutingPagamentoAprovado,
		events.RoutingPagamentoRecusado,
		events.RoutingPedidoEnviado,
		events.RoutingPedidoEstoqueOk,
		events.RoutingEstoqueIndisponivel,
	}
	for _, k := range keysToConsume {
		_ = ch.QueueBind(q.Name, k, ecommerceEx.Name, false, nil)
	}

	msgs, err := ch.Consume(q.Name, "", true, false, false, false, nil)
	helpers.FailOnError(err, "Erro no consumer")

	go func() {
		for d := range msgs {
			var p events.PedidoPayload
			if err := json.Unmarshal(d.Body, &p); err != nil {
				continue
			}

			mu.Lock()
			ped, exists := pedidosMap[p.ID]
			if !exists {
				ped = &p
				pedidosMap[p.ID] = ped
			}

			switch d.RoutingKey {
			case events.RoutingPedidoEstoqueOk:
				ped.Status = "Estoque Reservado / Pagamento Pendente"
			case events.RoutingEstoqueIndisponivel:
				ped.Status = "Cancelado (Estoque Indisponível)"
				publishEvent(ch, ecommerceEx.Name, events.RoutingPedidoExcluido, ped)
			case events.RoutingPagamentoAprovado:
				ped.Status = "Pagamento Aprovado"
			case events.RoutingPagamentoRecusado:
				ped.Status = "Cancelado (Pagamento Recusado)"
				publishEvent(ch, ecommerceEx.Name, events.RoutingPedidoExcluido, ped)
			case events.RoutingPedidoEnviado:
				ped.Status = "Enviado"
			}
			log.Printf("[Principal] Pedido %s atualizado: %s", ped.ID, ped.Status)
			mu.Unlock()
		}
	}()

	scanner := bufio.NewScanner(os.Stdin)
	for {
		fmt.Println("\n--- MENU PRINCIPAL ---")
		fmt.Println("1. Criar Pedido")
		fmt.Println("2. Consultar Pedidos")
		fmt.Println("3. Cancelar Pedido")
		fmt.Print("Escolha: ")
		if !scanner.Scan() {
			break
		}

		switch strings.TrimSpace(scanner.Text()) {
		case "1":
			fmt.Print("ID do Pedido: ")
			scanner.Scan()
			id := scanner.Text()
			fmt.Print("ID do Produto (UUID): ")
			scanner.Scan()
			pID := scanner.Text()

			p := events.PedidoPayload{
				ID:     id,
				Status: "Criado",
				Itens:  []events.ItemPedido{{ProductID: pID, Quantity: 1}},
			}
			mu.Lock()
			pedidosMap[id] = &p
			mu.Unlock()

			publishEvent(ch, ecommerceEx.Name, events.RoutingPedidoCriado, p)
			fmt.Println("Pedido criado e enviado!")
		case "2":
			mu.Lock()
			fmt.Println("\n--- Pedidos ---")
			for id, p := range pedidosMap {
				fmt.Printf("ID: %s | Status: %s\n", id, p.Status)
			}
			mu.Unlock()
		case "3":
			fmt.Print("ID do Pedido para cancelar: ")
			scanner.Scan()
			id := scanner.Text()
			mu.Lock()
			if p, ok := pedidosMap[id]; ok {
				p.Status = "Cancelado Manualmente"
				publishEvent(ch, ecommerceEx.Name, events.RoutingPedidoExcluido, p)
				fmt.Println("Solicitação de exclusão enviada!")
			}
			mu.Unlock()
		}
	}
}

func publishEvent(ch *amqp.Channel, exchange, rkey string, payload interface{}) {
	body, _ := json.Marshal(payload)
	_ = ch.Publish(exchange, rkey, false, false, amqp.Publishing{
		ContentType: "application/json",
		Body:        body,
	})
}