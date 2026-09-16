package main

import (
	"bufio"
	"crypto/rand"
	"eCommerce/pkg/events"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"strconv"
	"strings"
)

var (
	ErrActionUnavailable = errors.New("esta ação ainda não está disponível")
	errMenuCancelled     = errors.New("operação cancelada")
	errMenuInputClosed   = errors.New("entrada do terminal encerrada")
)

// MenuCallbacks separa a leitura do terminal da implementação das ações.
// As consultas poderão ser conectadas aos serviços sem alterar o menu.
type MenuCallbacks struct {
	ListProducts func() error
	CreateOrder  func(events.CreateOrderRequest) error
	DeleteOrder  func(events.DeleteOrderRequest) error
	ListOrders   func(customerID string) error
}

type terminalMenu struct {
	input      *bufio.Scanner
	output     io.Writer
	callbacks  MenuCallbacks
	customerID string
}

func RunMenu(input io.Reader, output io.Writer, callbacks MenuCallbacks) error {
	menu := terminalMenu{input: bufio.NewScanner(input), output: output, callbacks: callbacks}
	fmt.Fprintln(output, "Bem-vindo ao e-commerce!")
	customerID, err := menu.readRequired("Seu identificador (ou /cancelar para sair): ")
	if errors.Is(err, errMenuInputClosed) || errors.Is(err, errMenuCancelled) {
		return nil
	}
	if err != nil {
		return err
	}
	menu.customerID = customerID

	for {
		fmt.Fprintf(output, "\nE-commerce — usuário: %s\n", customerID)
		fmt.Fprintln(output, "1 - Visualizar produtos")
		fmt.Fprintln(output, "2 - Realizar pedido")
		fmt.Fprintln(output, "3 - Excluir pedido")
		fmt.Fprintln(output, "4 - Consultar meus pedidos e status")
		fmt.Fprintln(output, "0 - Sair")
		fmt.Fprint(output, "Escolha uma opção: ")
		choice, err := menu.readLine()
		if errors.Is(err, errMenuInputClosed) {
			fmt.Fprintln(output, "\nAté logo!")
			return nil
		}
		if errors.Is(err, errMenuCancelled) {
			continue
		}
		if err != nil {
			return err
		}

		switch choice {
		case "0":
			fmt.Fprintln(output, "Até logo!")
			return nil
		case "1":
			if callbacks.ListProducts == nil {
				err = ErrActionUnavailable
			} else {
				err = callbacks.ListProducts()
			}
		case "2":
			err = menu.createOrder()
		case "3":
			err = menu.deleteOrder()
		case "4":
			if callbacks.ListOrders == nil {
				err = ErrActionUnavailable
			} else {
				err = callbacks.ListOrders(customerID)
			}
		default:
			fmt.Fprintln(output, "Opção inválida. Escolha um número de 0 a 4.")
		}

		switch {
		case errors.Is(err, errMenuInputClosed):
			fmt.Fprintln(output, "\nEntrada encerrada. A solicitação incompleta não foi enviada.")
			return nil
		case errors.Is(err, errMenuCancelled):
			fmt.Fprintln(output, "Operação cancelada. Nenhuma solicitação foi enviada.")
		case errors.Is(err, ErrActionUnavailable):
			fmt.Fprintln(output, "Esta ação ainda não está disponível. Escolha outra opção.")
		case err != nil:
			fmt.Fprintf(output, "Não foi possível concluir a solicitação: %v\n", err)
		}
	}
}

func (menu *terminalMenu) createOrder() error {
	if menu.callbacks.CreateOrder == nil {
		return ErrActionUnavailable
	}
	fmt.Fprintln(menu.output, "\nNovo pedido. Digite /cancelar em qualquer campo para voltar ao menu.")
	request := events.CreateOrderRequest{CustomerID: menu.customerID}
	products := make(map[string]bool)
	for {
		productID, err := menu.readRequired("Identificador do produto: ")
		if err != nil {
			return err
		}
		if products[productID] {
			fmt.Fprintln(menu.output, "Esse produto já foi adicionado. Informe outro produto ou /cancelar para refazer o pedido.")
			continue
		}
		quantity, err := menu.readQuantity()
		if err != nil {
			return err
		}
		request.Items = append(request.Items, events.OrderItem{ProductID: productID, Quantity: quantity})
		products[productID] = true
		more, err := menu.confirm("Adicionar outro produto? (s/n): ")
		if err != nil {
			return err
		}
		if !more {
			break
		}
	}

	fmt.Fprintln(menu.output, "\nConfira os itens do pedido:")
	for _, item := range request.Items {
		fmt.Fprintf(menu.output, "- Produto %s: %d unidade(s)\n", item.ProductID, item.Quantity)
	}
	confirmed, err := menu.confirm("Enviar pedido? (s/n): ")
	if err != nil {
		return err
	}
	if !confirmed {
		return errMenuCancelled
	}
	identifier := make([]byte, 16)
	if _, err := rand.Read(identifier); err != nil {
		return fmt.Errorf("gerar identificador do pedido: %w", err)
	}
	request.OrderID = hex.EncodeToString(identifier)
	if err := menu.callbacks.CreateOrder(request); err != nil {
		return err
	}
	fmt.Fprintf(menu.output, "Solicitação de criação enviada. Identificador do pedido: %s\n", request.OrderID)
	return nil
}

func (menu *terminalMenu) deleteOrder() error {
	if menu.callbacks.DeleteOrder == nil {
		return ErrActionUnavailable
	}
	fmt.Fprintln(menu.output, "\nExcluir pedido. Digite /cancelar para voltar ao menu.")
	orderID, err := menu.readRequired("Identificador do pedido: ")
	if err != nil {
		return err
	}
	confirmed, err := menu.confirm(fmt.Sprintf("Solicitar exclusão do pedido %s? (s/n): ", orderID))
	if err != nil {
		return err
	}
	if !confirmed {
		return errMenuCancelled
	}
	request := events.DeleteOrderRequest{OrderID: orderID, CustomerID: menu.customerID}
	if err := menu.callbacks.DeleteOrder(request); err != nil {
		return err
	}
	fmt.Fprintln(menu.output, "Solicitação de exclusão enviada. Aguarde o processamento do pedido.")
	return nil
}

func (menu *terminalMenu) readLine() (string, error) {
	if !menu.input.Scan() {
		if err := menu.input.Err(); err != nil {
			return "", fmt.Errorf("ler entrada do terminal: %w", err)
		}
		return "", errMenuInputClosed
	}
	text := strings.TrimSpace(menu.input.Text())
	if text == "/cancelar" {
		return "", errMenuCancelled
	}
	return text, nil
}

func (menu *terminalMenu) readRequired(prompt string) (string, error) {
	for {
		fmt.Fprint(menu.output, prompt)
		text, err := menu.readLine()
		if err != nil {
			return "", err
		}
		if text != "" {
			return text, nil
		}
		fmt.Fprintln(menu.output, "Este campo é obrigatório.")
	}
}

func (menu *terminalMenu) readQuantity() (int, error) {
	for {
		text, err := menu.readRequired("Quantidade (inteiro maior que zero): ")
		if err != nil {
			return 0, err
		}
		quantity, err := strconv.Atoi(text)
		if err == nil && quantity > 0 {
			return quantity, nil
		}
		fmt.Fprintln(menu.output, "Quantidade inválida. Informe um número inteiro maior que zero.")
	}
}

func (menu *terminalMenu) confirm(prompt string) (bool, error) {
	for {
		text, err := menu.readRequired(prompt)
		if err != nil {
			return false, err
		}
		switch strings.ToLower(text) {
		case "s", "sim":
			return true, nil
		case "n", "nao", "não":
			return false, nil
		default:
			fmt.Fprintln(menu.output, "Responda s para sim ou n para não.")
		}
	}
}
