package main

import (
	"fmt"
	"io"
	"log"
	"os"
	"strconv"
	"strings"
	"text/tabwriter"
	"time"

	"github.com/chzyer/readline"

	"context"
	"eCommerce/pkg/config"
	"eCommerce/pkg/events"
	"eCommerce/pkg/exchanges"
	"eCommerce/pkg/helpers"
	"eCommerce/pkg/mock"

	amqp "github.com/rabbitmq/amqp091-go"
)

type TerminalUserInterface struct {
	Config     *config.Config
	Connection *amqp.Connection
	Channel    *amqp.Channel
	Queue      amqp.Queue
	Messages   <-chan amqp.Delivery
	isOpen     bool
}

func (tui *TerminalUserInterface) DestroyTUI() {
	if tui.Channel != nil {
		tui.Channel.Close()
	}
	if tui.Connection != nil {
		tui.Connection.Close()
	}
}

func CreateTUI() (*TerminalUserInterface, error) {
	var tui *TerminalUserInterface = &TerminalUserInterface{}
	var err error
	tui.Config, err = config.Load()
	if err != nil {
		helpers.LogErrorMessage(err, "Failed to load configuration")
		return nil, err
	}

	tui.Connection, err = amqp.Dial(tui.Config.RabbitMQURL)
	if err != nil {
		helpers.LogErrorMessage(err, "Failed to connect to RabbitMQ")
		return nil, err
	}

	tui.Channel, err = tui.Connection.Channel()
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
		tui.DestroyTUI()
		return nil, err
	}

	tui.Queue, err = tui.Channel.QueueDeclare(
		"",    // name
		false, // durability
		false, // delete when unused
		true,  // exclusive
		false, // no-wait
		nil,
	)
	if err != nil {
		helpers.LogErrorMessage(err, "Failed to declare a queue")
		tui.DestroyTUI()
		return nil, err
	}

	routingKeys := []string{
		events.PaymentEventApproved.String(),
		events.PaymentEventRefused.String(),
		events.OrderEventSent.String(),
		events.OrderEventStoreOk.String(),
		events.StoreEventUnavailable.String(),
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
			tui.DestroyTUI()
			return nil, err
		}
	}

	tui.Messages, err = tui.Channel.Consume(
		tui.Queue.Name, // queue
		"",             // consumer
		true,           // auto-ack
		false,          // exclusive
		false,          // no-local
		false,          // no-wait
		nil,            // args
	)
	if err != nil {
		helpers.LogErrorMessage(err,
			fmt.Sprintf("Failed to register a consumer to queue '%v'",
				tui.Queue.Name))
		tui.DestroyTUI()
		return nil, err
	}

	go tui.handleMessage()

	return tui, err
}

func (tui *TerminalUserInterface) handleMessage() {
	for message := range tui.Messages {
		log.Printf("Received a message: %s", message.Body)
	}
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
		),
		readline.PcItem("create",
			readline.PcItem("order",
				readline.PcItem("-p",
					readline.PcItemDynamic(func(string) []string {
						names := make([]string, len(products))
						for i, product := range products {
							names[i] = product.Name
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

func (tui *TerminalUserInterface) consoleLoop(l *readline.Instance, completer *readline.PrefixCompleter, products []mock.Product) {
	for tui.isOpen {
		line, err := l.Readline()
		if err == readline.ErrInterrupt {
			if len(line) == 0 {
				break
			} else {
				continue
			}
		} else if err == io.EOF {
			break
		}

		line = strings.TrimSpace(line)
		switch {
		case line == "help" || line == "?":
			usage(l.Stderr(), completer)
		case strings.HasPrefix(line, "show"):
			line := strings.TrimSpace(line[4:])
			switch line {
			case "orders":

			case "products":
				writer := tabwriter.NewWriter(os.Stdout, 1, 1, 1, ' ', 0)
				for _, product := range products {
					fmt.Fprintf(writer, " | %v:\t$ %v\n", product.Name, product.Price)
				}
				writer.Flush()
			default:
				log.Println("invalid argument ",
					strconv.Quote(line))
			}

		case line == "quit" || line == "exit":
			tui.isOpen = false
		case line == "clear":
			readline.ClearScreen(l.Stderr())
		case strings.HasPrefix(line, "create"):
			line := strings.TrimSpace(line[6:])
			if strings.HasPrefix(line, "order") {
				line := strings.TrimSpace(line[5:])
				if strings.HasPrefix(line, "-p") {
					line := strings.TrimSpace(line[2:])
					if !strings.Contains(line, "-q") {
						log.Println("to create an order specify the quantity with -q")
					}
					productName, quantityString, _ := strings.CutLast(line, "-q")
					productName = strings.TrimSpace(productName)
					quantityString = strings.TrimSpace(quantityString)
					quantity, err := strconv.Atoi(strings.TrimSpace(quantityString))
					if err != nil {
						log.Println("invalid quantity argument " + strconv.Quote(quantityString))
						break
					}
					log.Println("ordering: ", quantity, productName)

				}
			}

		case line == "":
		default:
			log.Println("command ",
				strconv.Quote(line),
				" not found")
		}
	}
}

func (tui *TerminalUserInterface) TestSend(body string) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	var err = tui.Channel.PublishWithContext(ctx,
		exchanges.GetEcommerceExchangeInfo().Name, // exchange
		events.OrderEventCreated.String(),         // routing key
		false,                                     // mandatory
		false,                                     // immediate
		amqp.Publishing{
			ContentType: "text/plain",
			Body:        []byte(body),
		})
	helpers.FailOnError(err, "Failed to publish a message")
	log.Printf(" [x] Sent %s\n", body)
}
