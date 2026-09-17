package main

import (
	"context"
	"crypto"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"

	"eCommerce/pkg/events"
	"eCommerce/pkg/exchanges"

	amqp "github.com/rabbitmq/amqp091-go"
)

func (tui *TerminalUserInterface) listProducts(ctx context.Context) ([]events.Product, error) {
	if tui.Connection == nil {
		return nil, fmt.Errorf("conexão de consulta indisponível")
	}
	ch, err := tui.Connection.Channel()
	if err != nil {
		return nil, fmt.Errorf("abrir canal de consulta: %w", err)
	}
	defer ch.Close()

	// Cada consulta recebe sua própria resposta; a fila é removida ao fechar o canal.
	q, err := ch.QueueDeclare("", false, true, true, false, nil)
	if err != nil {
		return nil, fmt.Errorf("criar fila de resposta: %w", err)
	}
	// Também limpa a fila se houver erro antes de registrar o consumidor.
	defer ch.QueueDelete(q.Name, false, false, false)
	exchange := exchanges.GetEcommerceExchangeInfo().Name
	replyKey := events.RoutingProdutosListados + "." + q.Name
	if err := ch.QueueBind(q.Name, replyKey, exchange, false, nil); err != nil {
		return nil, fmt.Errorf("associar fila de resposta: %w", err)
	}
	messages, err := ch.Consume(q.Name, "", true, true, false, false, nil)
	if err != nil {
		return nil, fmt.Errorf("aguardar resposta do estoque: %w", err)
	}
	identifier := make([]byte, 16)
	if _, err := rand.Read(identifier); err != nil {
		return nil, fmt.Errorf("identificar consulta: %w", err)
	}

	correlationID := hex.EncodeToString(identifier)

	checksum := sha256.Sum256(identifier)

	signature, err := rsa.SignPSS(rand.Reader, tui.privateKey, crypto.SHA256, checksum[:], nil)
	if err != nil {
		fmt.Printf("preparar assinatura: %v", err)
	}

	if err := ch.PublishWithContext(ctx, exchange, events.RoutingProdutosConsultar, false, false, amqp.Publishing{
		ContentType:   "application/json",
		Headers:       amqp.Table{"x-signature": signature},
		ReplyTo:       replyKey,
		CorrelationId: correlationID,
		Body:          identifier,
	}); err != nil {
		return nil, fmt.Errorf("consultar produtos: %w", err)
	}

	for {
		select {
		case <-ctx.Done():
			return nil, fmt.Errorf("o estoque não respondeu à consulta; verifique se o serviço está rodando: %w", ctx.Err())
		case message, ok := <-messages:
			if !ok {
				return nil, fmt.Errorf("conexão encerrada durante a consulta de produtos")
			}
			if message.CorrelationId != correlationID {
				continue
			}
			var payload events.ProductsPayload
			if err := json.Unmarshal(message.Body, &payload); err != nil {
				return nil, fmt.Errorf("resposta de produtos inválida: %w", err)
			}
			return payload.Products, nil
		}
	}
}

func displayProducts(output io.Writer, products []events.Product) error {
	if len(products) == 0 {
		_, err := fmt.Fprintln(output, "Nenhum produto cadastrado no estoque.")
		return err
	}
	if _, err := fmt.Fprintln(output, "\n--- Produtos ---"); err != nil {
		return err
	}
	for _, product := range products {
		if _, err := fmt.Fprintf(output, "%s\n  ID: %s | Preço: %.2f | Estoque: %d\n",
			product.Name, product.UUID, product.Price, product.Quantity); err != nil {
			return err
		}
	}
	return nil
}
