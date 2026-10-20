package main

import "html/template"

// página de checkout do Mock de Pagamento (mesma identidade visual do frontend)
const checkoutPage = `<!DOCTYPE html>
<html lang="pt-BR">
<head>
  <meta charset="UTF-8">
  <meta name="viewport" content="width=device-width, initial-scale=1">
  <title>Mock de Pagamento</title>
  <link rel="stylesheet" href="https://fonts.googleapis.com/css2?family=Oswald:wght@500;700&family=Rubik:wght@400;600&display=swap">
  <style>
    :root {
      --fundo: #fafafa; --card: #ffffff; --marinho: #0a1628; --destaque: #FFDF00;
      --acento: #009C3B; --erro: #c62828; --texto: #1a1a1a; --suave: #666666; --borda: #e6e6e6;
    }
    @media (prefers-color-scheme: dark) {
      :root { --fundo: #080f1a; --card: #111d30; --marinho: #060c16; --acento: #10b981;
              --erro: #ef5350; --texto: #e6edf3; --suave: #9aa5b1; --borda: #1e2e4a; }
    }
    * { box-sizing: border-box; }
    body { margin: 0; min-height: 100vh; font-family: 'Rubik', sans-serif; background: var(--fundo); color: var(--texto); }
    header { background: var(--marinho); border-bottom: 3px solid var(--acento); padding: 18px; text-align: center; }
    header h1 { margin: 0; font-family: 'Oswald', sans-serif; color: #fff; font-size: 26px; }
    header h1 span { color: var(--destaque); }
    main { max-width: 420px; margin: 48px auto; padding: 0 16px; }
    .card { background: var(--card); border: 1px solid var(--borda); border-radius: 10px; padding: 28px; text-align: center;
            box-shadow: 0 2px 8px rgba(0, 0, 0, 0.06); }
    .card h2 { margin: 0 0 6px; font-family: 'Oswald', sans-serif; }
    .pedido { margin: 0 0 22px; color: var(--suave); font-family: monospace; word-break: break-all; }
    button { width: 100%; margin-top: 12px; padding: 12px; border: 0; border-radius: 6px; color: #fff; cursor: pointer;
             font-family: 'Rubik', sans-serif; font-size: 16px; font-weight: 600; transition: filter 0.2s; }
    button:hover { filter: brightness(1.1); }
    .aprovar { background: var(--acento); }
    .recusar { background: var(--erro); }
    .resultado { font-size: 1.3rem; font-weight: 600; margin: 8px 0; }
    .APROVADO { color: var(--acento); }
    .RECUSADO { color: var(--erro); }
    .dica { color: var(--suave); font-size: 0.9rem; margin: 18px 0 0; }
  </style>
</head>
<body>
  <header><h1>Mock de <span>Pagamento</span></h1></header>
  <main>
    <div class="card">
      <h2>Confirmar pagamento</h2>
      <p class="pedido">Pedido {{.OrderId}}</p>
      {{if eq .Status "PENDENTE"}}
        <form method="post" action="/checkout/{{.OrderId}}/approve">
          <button class="aprovar" type="submit">Pagamento Aprovado</button>
        </form>
        <form method="post" action="/checkout/{{.OrderId}}/refuse">
          <button class="recusar" type="submit">Pagamento Recusado</button>
        </form>
      {{else}}
        <p class="resultado {{.Status}}">{{.Status}}</p>
        <p class="dica">O MS Pagamento foi notificado. Você já pode fechar esta aba e acompanhar o pedido na loja.</p>
      {{end}}
    </div>
  </main>
</body>
</html>`

var checkoutTemplate = template.Must(template.New("checkout").Parse(checkoutPage))
