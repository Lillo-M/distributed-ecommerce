package main

import (
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"log"
	"os"
	"slices"
	"strconv"
	"strings"
	"text/tabwriter"
	"time"
	"uuid"

	"github.com/chzyer/readline"

	"context"
	"eCommerce/pkg/dtos"
	"eCommerce/pkg/events"
	"eCommerce/pkg/exchanges"
	"eCommerce/pkg/helpers"
	"eCommerce/pkg/interfaces"
	"eCommerce/pkg/mock"
	"eCommerce/pkg/notifications"
	"eCommerce/pkg/requests"

	amqp "github.com/rabbitmq/amqp091-go"
)

type TerminalUserInterface struct {
	interfaces.Service
	Orders map[string]mock.OrderInfo
	Cart   []mock.Product
	isOpen bool
}

func CreateTUI() (*TerminalUserInterface, error) {
	var tui *TerminalUserInterface = &TerminalUserInterface{}
	var err error

	err = tui.InitRMQChannel()
	if err != nil {
		helpers.LogErrorMessage(err, "Failed to open a channel")
		tui.Connection.Close()
		return nil, err
	}

	var ecommerceExchange = exchanges.GetEcommerceExchangeInfo()
	err = tui.Channel.ExchangeDeclare(
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
		tui.CloseRMQChannel()
		return nil, err
	}

	routingKeys := []string{
		events.PaymentEventApproved.String(),
		events.PaymentEventRefused.String(),
		events.OrderSentEvent.String(),
		events.OrderStoreOkEvent.String(),
		events.StoreUnavailableEvent.String(),
	}

	for _, key := range routingKeys {
		if err := tui.Channel.QueueBind(
			tui.Queue.Name,         // queue
			key,                    // routing
			ecommerceExchange.Name, // exchange
			false,                  // no-wait
			nil,                    // arguments
		); err != nil {
			helpers.LogErrorMessage(err, fmt.Sprintf("Failed to bind queue '%s' to key '%s'", tui.Queue.Name, key))
			tui.CloseRMQChannel()
			return nil, err
		}
	}

	tui.StartConsuming(tui.handleMessages)

	return tui, err
}

func (tui *TerminalUserInterface) handleMessages(message amqp.Delivery) {
	log.Printf("Received a message: %s", message.Body)
	switch message.RoutingKey {
	case events.PaymentEventApproved.String():
	case events.PaymentEventRefused.String():
	case events.OrderSentEvent.String():
	case events.OrderStoreOkEvent.String():
		tui.handleStoreOk(message)
	case events.StoreUnavailableEvent.String():
		tui.handleStoreUnavailable(message)
	default:
		log.Printf("routing key '%q' unkown\n", message.RoutingKey)
	}
}

func (tui *TerminalUserInterface) handleStoreOk(message amqp.Delivery) {
	notification := notifications.StoreAvailability{}
	err := json.Unmarshal(message.Body, &notification)
	if err != nil {
		log.Fatalf("Failed to decode notification '%v'\n", message.Body)
	}

	order, ok := tui.Orders[notification.OrderId]
	if !ok {
		log.Fatalf("invalid order id '%v'\n", notification.OrderId)
		return
	}
	order.Status = mock.OrderStatusStockReserved
}

func (tui *TerminalUserInterface) handleStoreUnavailable(message amqp.Delivery) {
	notification := notifications.StoreAvailability{}
	err := json.Unmarshal(message.Body, &notification)
	if err != nil {
		log.Fatalf("Failed to decode notification '%v'\n", message.Body)
	}

	requestBytes, err := json.Marshal(requests.DeleteOrderRequest{
		OrderId: notification.OrderId,
	})
	if err != nil {
		log.Printf("failed to serialize request: %v\n", err)
		return
	}

	tui.SendToECommerceExchange(requestBytes, events.OrderDeletedEvent)
	order, ok := tui.Orders[notification.OrderId]
	if !ok {
		log.Fatalf("invalid order id '%v'\n", notification.OrderId)
		return
	}
	order.Status = mock.OrderStatusOutOfStock
}

func usage(w io.Writer, completer *readline.PrefixCompleter) {
	io.WriteString(w, "commands:\n")
	io.WriteString(w, completer.Tree("    "))
}

// Function constructor - constructs new function for listing given directory
func listFiles(path string) func(string) []string {
	return func(line string) []string {
		names := make([]string, 0)
		files, _ := os.ReadDir(path)
		for _, f := range files {
			names = append(names, f.Name())
		}
		return names
	}
}

func filterInput(r rune) (rune, bool) {
	switch r {
	// block CtrlZ feature
	case readline.CharCtrlZ:
		return r, false
	}
	return r, true
}
func (tui *TerminalUserInterface) Start() {
	tui.isOpen = true

	products, err := mock.GetProducts("store.json")
	if err != nil {
		panic(err)
	}

	var completer = readline.NewPrefixCompleter(
		readline.PcItem("help"),
		readline.PcItem("?"),
		readline.PcItem("show",
			readline.PcItem("products"),
			readline.PcItem("orders"),
			readline.PcItem("cart"),
		),
		readline.PcItem("delete",
			readline.PcItem("order",
				readline.PcItem("-id",
					readline.PcItemDynamic(func(s string) []string { return tui.GetOrdersIds() }),
				),
				readline.PcItem("-n",
					readline.PcItemDynamic(func(s string) []string { return tui.GetOrdersIndexes() }),
				),
			),
		),
		readline.PcItem("cart",
			readline.PcItem("remove",
				readline.PcItem("-p",
					readline.PcItemDynamic(func(string) []string {
						names := make([]string, len(tui.Cart))
						for i, product := range tui.Cart {
							names[i] = strconv.Quote(product.Name)
						}
						return names
					},
					),
				),
			),
			readline.PcItem("add",
				readline.PcItem("-p",
					readline.PcItemDynamic(func(string) []string {
						names := make([]string, len(products))
						for i, product := range products {
							names[i] = strconv.Quote(product.Name)
						}
						return names
					},
						readline.PcItem("-q",
							readline.PcItemDynamic(func(string) []string {
								return []string{"1", "2", "5", "10"}
							}),
						),
					),
				),
			),
		),
		readline.PcItem("create",
			readline.PcItem("order"),
		),
		readline.PcItem("clear"),
	)

	l, err := readline.NewEx(&readline.Config{
		Prompt:          "\033[31m»\033[0m ",
		HistoryFile:     "/tmp/readline.tmp",
		AutoComplete:    completer,
		InterruptPrompt: "^C",
		EOFPrompt:       "exit",

		HistorySearchFold:   true,
		FuncFilterInputRune: filterInput,
	})
	if err != nil {
		panic(err)
	}
	defer l.Close()
	l.CaptureExitSignal()

	log.Println("-- Main Microserice TUI ---")
	log.Println("Type '?' for help")
	log.SetOutput(l.Stderr())
	tui.consoleLoop(l, completer, products)
}

func (tui *TerminalUserInterface) GetOrdersIds() []string {
	var ids []string = make([]string, len(tui.Orders))
	i := 0
	for _, order := range tui.Orders {
		ids[i] = order.UUID
		i += 1
	}
	return ids
}
func (tui *TerminalUserInterface) GetOrdersIndexes() []string {
	arr := make([]string, len(tui.Orders))
	for i := range arr {
		arr[i] = strconv.Itoa(i)
	}
	return arr
}

// * Made by Gemini
func splitInArguments(line string) []string {
	var args []string
	var current strings.Builder
	inQuotes := false
	var quoteChar rune

	for _, r := range line {
		switch {
		// Start of a quoted section
		case (r == '"' || r == '\'') && !inQuotes:
			inQuotes = true
			quoteChar = r

		// End of a quoted section
		case r == quoteChar && inQuotes:
			inQuotes = false
			quoteChar = 0

		// Space outside of quotes triggers token emission
		case r == ' ' && !inQuotes:
			if current.Len() > 0 {
				args = append(args, current.String())
				current.Reset()
			}

		// Regular character inside or outside quotes
		default:
			current.WriteRune(r)
		}
	}

	if current.Len() > 0 {
		args = append(args, current.String())
	}

	return args
}

func (tui *TerminalUserInterface) consoleLoop(l *readline.Instance, completer *readline.PrefixCompleter, products []mock.Product) {
	for tui.isOpen {
		line, err := l.Readline()
		if errors.Is(err, readline.ErrInterrupt) {
			if len(line) == 0 {
				break
			}
			continue
		} else if errors.Is(err, io.EOF) {
			break
		}

		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		args := splitInArguments(line)
		switch {
		case slices.Contains([]string{"help", "?"}, args[0]):
			usage(l.Stderr(), completer)
		case slices.Contains([]string{"quit", "exit"}, args[0]):
			tui.isOpen = false
		case args[0] == "clear":
			readline.ClearScreen(l.Stderr())
		case args[0] == "show":
			tui.handleShowCommand(args[1:], products)
		case args[0] == "create":
			tui.handleCreateCommand()
		case args[0] == "cart":
			tui.handleCartCommand(args[1:], products)
		default:
			log.Printf("command %q not found\n", args[0])
		}
	}
}

func (tui *TerminalUserInterface) handleShowCommand(args []string, products []mock.Product) {
	if len(args) == 0 {
		log.Println("usage: show [orders|products|cart]")
		return
	}

	switch args[0] {
	case "orders":
		tui.renderOrdersTable()
	case "cart":
		tui.renderCartTable()
	case "products":
		renderProductsTable(products)
	default:
		log.Printf("invalid argument %q\n", args[0])
	}
}

func (tui *TerminalUserInterface) handleCartCommand(args []string, products []mock.Product) {
	if len(args) == 0 || (args[0] != "add" && args[0] != "remove") {
		log.Println("usage: cart [add|remove] -p <product_name> -q <quantity>")
		return
	}

	switch args[0] {
	case "add":
		err := tui.handleCartAddCommand(args, products)
		if err != nil {
			log.Printf("failed add product to cart: %v\n", err)
		}
	case "remove":
		err := tui.handleCartRemoveCommand(args, products)
		if err != nil {
			log.Printf("failed remove product from cart: %v\n", err)
		}
	default:
		log.Printf("invalid argument %q\n", args[0])
	}
}

func (tui *TerminalUserInterface) handleCartRemoveCommand(args []string, products []mock.Product) error {
	fs := flag.NewFlagSet("cart remove", flag.ContinueOnError)
	productName := fs.String("p", "", "Product name")

	if err := fs.Parse(args[1:]); err != nil {
		return err
	}

	if *productName == "" {
		log.Println("-p (product name) is required")
		return nil
	}

	productIdx := slices.IndexFunc(products, func(p mock.Product) bool {
		return p.Name == *productName
	})

	if productIdx == -1 {
		log.Printf("the product '%q' is not in the cart\n", *productName)
		return nil
	}

	tui.Cart = slices.Delete(tui.Cart, productIdx-1, productIdx)
	return nil
}

func (tui *TerminalUserInterface) handleCartAddCommand(args []string, products []mock.Product) error {
	fs := flag.NewFlagSet("cart add", flag.ContinueOnError)
	productName := fs.String("p", "", "Product name")
	quantity := fs.Int("q", 0, "Quantity")

	if err := fs.Parse(args[1:]); err != nil {
		return err
	}

	if *productName == "" || *quantity <= 0 {
		log.Println("both -p (product name) and -q (quantity > 0) are required")
		return nil
	}

	productIdx := slices.IndexFunc(products, func(p mock.Product) bool {
		return p.Name == *productName
	})

	if productIdx == -1 {
		log.Printf("invalid product '%q'\n", *productName)
		return nil
	}

	selected := products[productIdx]
	selected.Quantity = *quantity
	tui.Cart = append(tui.Cart, selected)
	return nil
}

func (tui *TerminalUserInterface) handleCreateCommand() {
	if len(tui.Cart) <= 0 {
		log.Printf("can't order an empty cart\n")
		return
	}
	orderId := uuid.New().String()
	requestBytes, err := json.Marshal(requests.CreateOrderRequest{
		Id: orderId,
		Products: func() []dtos.ProductDto {
			productDtos := make([]dtos.ProductDto, len(tui.Cart))
			for i, product := range tui.Cart {
				productDtos[i] = dtos.ProductDto{ProductId: product.UUID, Quantity: product.Quantity}
			}
			return productDtos
		}(),
	})
	if err != nil {
		log.Printf("failed to serialize request: %v\n", err)
		return
	}

	cart := make([]mock.Product, len(tui.Cart))
	copy(cart, tui.Cart)
	order := mock.OrderInfo{
		UUID:     orderId,
		Products: cart,
		Status:   mock.OrderStatusCreated,
	}

	tui.Cart = nil
	tui.Orders[order.UUID] = order
	tui.SendToECommerceExchange(requestBytes, events.OrderCreatedEvent)
}

func (tui *TerminalUserInterface) renderOrdersTable() {
	if len(tui.Orders) <= 0 {
		log.Println("There are no ongoing orders")
		return
	}

	w := tabwriter.NewWriter(os.Stdout, 1, 1, 1, ' ', 0)
	defer w.Flush()

	for idx, order := range tui.Orders {
		fmt.Fprintf(w, "Order N° %v\tid %v\tStatus %v\n", idx, order.UUID, order.Status)
		for _, product := range order.Products {
			fmt.Fprintf(w, "- %d\t%s\t$ %v\n",
				product.Quantity, product.Name, product.Price)
		}
	}
}

func (tui *TerminalUserInterface) renderCartTable() {
	if len(tui.Cart) <= 0 {
		log.Println("There are no products in the cart")
		return
	}
	w := tabwriter.NewWriter(os.Stdout, 1, 1, 1, ' ', 0)
	defer w.Flush()

	for _, p := range tui.Cart {
		fmt.Fprintf(w, " | %v:\t%s\n", p.Quantity, p.Name)
	}
}

func renderProductsTable(products []mock.Product) {
	if len(products) <= 0 {
		log.Println("There are no products available")
		return
	}

	w := tabwriter.NewWriter(os.Stdout, 1, 1, 1, ' ', 0)
	defer w.Flush()

	for _, p := range products {
		fmt.Fprintf(w, " | %s:\t$ %v\n", p.Name, p.Price)
	}
}

func (tui *TerminalUserInterface) SendToECommerceExchange(body []byte, routingKey events.OrderEvent) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	var err = tui.Channel.PublishWithContext(ctx,
		exchanges.GetEcommerceExchangeInfo().Name, // exchange
		routingKey.String(),                       // routing key
		false,                                     // mandatory
		false,                                     // immediate
		amqp.Publishing{
			ContentType: "application/json",
			Body:        []byte(body),
		})
	helpers.FailOnError(err, "Failed to publish a message")
	log.Printf(" [x] Sent %s\n", body)
}
