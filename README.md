# distributed-ecommerce

## Interface pelo terminal

Com Go compatível com o `go.mod` instalado e RabbitMQ disponível, execute na raiz:

```bash
RABBITMQ_URL=amqp://guest:guest@localhost:5672/ go run ./services/tui
```

Também é possível configurar `RABBITMQ_URL` em `.env`. Execute a pasta completa
`./services/tui`, pois a interface está distribuída entre vários arquivos Go.

Informe seu identificador e selecione uma opção:

```text
1 - Visualizar produtos
2 - Realizar pedido
3 - Excluir pedido
4 - Consultar meus pedidos e status
0 - Sair
```

- **Realizar pedido:** informe os IDs dos produtos e quantidades positivas, confira
  os itens e confirme o envio. O terminal gera e mostra o ID do pedido.
- **Excluir pedido:** informe o ID do pedido e confirme a solicitação.
- **Visualizar produtos / consultar pedidos:** chamam callbacks preparados para
  implementação futura e informam que a ação ainda não está disponível.
- Digite `/cancelar` durante um formulário para voltar ao menu sem enviar. A opção
  `0` e o fim da entrada encerram a interface e fecham seus recursos.

O identificador de usuário apenas acompanha a solicitação; autenticação e
verificação de propriedade do pedido ainda não estão implementadas. Como a
consulta de catálogo está pendente, os IDs de exemplo podem ser obtidos em
`stock.json`, no campo `uuid`.

## Callbacks e eventos

`services/tui/menu.go` cuida da entrada e da validação. `services/tui/callbacks.go`
contém os pontos de integração:

| Callback | Comportamento atual |
|---|---|
| `OnListProducts` | Sem integração; retorna `ErrActionUnavailable`. |
| `OnCreateOrder` | Publica JSON em `eCommerce` com routing key `Order.Created`. |
| `OnDeleteOrder` | Publica JSON em `eCommerce` com routing key `Order.Deleted`. |
| `OnListOrders` | Sem integração; recebe o usuário e retorna `ErrActionUnavailable`. |

Os nomes dos eventos foram preservados para compatibilidade com o protótipo.
Exemplo do corpo de criação:

```json
{
  "order_id": "identificador-gerado",
  "customer_id": "joao",
  "items": [
    {"product_id": "e3a89304-4c28-4e89-9a74-d021c3258bd2", "quantity": 2}
  ]
}
```

A exclusão envia `order_id` e `customer_id`. Os tipos estão em
`pkg/events/order.go`. Não há envio automático de `Hello, World!` ao abrir a TUI.

O envio não significa que o pedido foi aprovado ou excluído pelo backend. O
Estoque atual ainda responde indisponibilidade a toda criação e não consome
exclusões. Assinatura digital, armazenamento de pedidos e atualização de status
continuam pendentes.

Para observar o protótipo, inicie a TUI primeiro (ela declara `eCommerce`), depois
`go run ./services/stock` em outro terminal, e só então envie um pedido pelo menu.
Sem fila vinculada ao evento, o RabbitMQ pode descartar a publicação com a
configuração atual; confirmações de publicação e tratamento de mensagens sem
rota ainda não foram adicionados.

## Verificação

```bash
go test ./...
go vet ./...
```

O teste de integração publica eventos reais; use um broker separado para testes:

```bash
TUI_RABBITMQ_TEST_URL=amqp://guest:guest@localhost:5679/ \
  go test -tags=integration ./services/tui -run TestTerminalPublishesRabbitMQEvents -v
```

O teste declara uma fila temporária, executa o menu com entrada controlada e
verifica os eventos de criação/exclusão recebidos do broker. Os demais testes
validam o menu e os callbacks sem depender de RabbitMQ.
