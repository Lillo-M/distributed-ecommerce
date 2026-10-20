package main

import "html/template"

const checkoutPage = `<!DOCTYPE html>
<html lang="pt-BR">
<head>
  <meta charset="UTF-8">
  <title>Mock de Pagamento</title>
  <style>
    body { font-family: sans-serif; max-width: 420px; margin: 60px auto; text-align: center; }
    .box { border: 1px solid #ccc; border-radius: 8px; padding: 24px; }
    button { width: 100%; padding: 12px; margin-top: 12px; font-size: 16px; border: 0; border-radius: 6px; color: #fff; cursor: pointer; }
    .ok { background: #2e7d32; }
    .no { background: #c62828; }
  </style>
</head>
<body>
  <div class="box">
    <h2>Mock de Pagamento</h2>
    <p>Pedido: <strong>{{.OrderId}}</strong></p>
    {{if eq .Status "PENDENTE"}}
      <form method="post" action="/checkout/{{.OrderId}}/approve">
        <button class="ok" type="submit">Pagamento Aprovado</button>
      </form>
      <form method="post" action="/checkout/{{.OrderId}}/refuse">
        <button class="no" type="submit">Pagamento Recusado</button>
      </form>
    {{else}}
      <p>Esta cobrança já foi decidida: <strong>{{.Status}}</strong></p>
    {{end}}
  </div>
</body>
</html>`

var checkoutTemplate = template.Must(template.New("checkout").Parse(checkoutPage))
