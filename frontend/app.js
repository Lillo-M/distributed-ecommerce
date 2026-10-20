// =============================================================================
// Frontend do e-commerce distribuído
// Conversa apenas com o MS Principal (API Gateway): REST para comandos e
// consultas, SSE (GET /events) para acompanhar o status dos pedidos.
// =============================================================================

// Endereço do API Gateway. Padrão: docker-compose (porta 80).
// Para outro endereço, abra a página com ?api=http://localhost:8080
const API_URL = new URLSearchParams(location.search).get("api") || "http://localhost";

// Etapas exibidas na linha do tempo de cada pedido (status vindos do backend)
const ETAPAS = [
    { status: "OrderCreated",    rotulo: "Pedido criado" },
    { status: "StockReserved",   rotulo: "Estoque confirmado" },
    { status: "PaymentApproved", rotulo: "Pagamento aprovado" },
    { status: "OrderSent",       rotulo: "Pedido enviado" },
];

// Status de falha e a etapa (índice em ETAPAS) em que o pedido parou
const FALHAS = {
    OutOfStock:    { etapa: 1, rotulo: "Sem estoque" },
    PaymentDenied: { etapa: 2, rotulo: "Pagamento recusado" },
};

const MENSAGENS = {
    StockReserved:   "Estoque confirmado! Abrindo o pagamento…",
    PaymentApproved: "Pagamento aprovado.",
    OrderSent:       "Pedido enviado!",
    OutOfStock:      "Pedido cancelado: produto sem estoque.",
    PaymentDenied:   "Pagamento recusado. Pedido cancelado.",
};

const estado = {
    produtos: [],              // catálogo vindo de GET /products
    carrinho: new Map(),       // uuid do produto -> quantidade
    pedidos: new Map(),        // uuid do pedido -> último OrderInfo recebido
    meusPedidos: carregarMeusPedidos(), // uuids dos pedidos feitos neste navegador
    abasPagamento: new Map(),  // uuid do pedido -> aba aberta aguardando a URL de checkout
    fonteEventos: null,        // conexão SSE atual
};

// -----------------------------------------------------------------------------
// Utilitários
// -----------------------------------------------------------------------------
const $ = (seletor) => document.querySelector(seletor);

const formatarPreco = (valor) =>
    valor.toLocaleString("pt-BR", { style: "currency", currency: "BRL" });

function escaparHtml(texto) {
    const div = document.createElement("div");
    div.textContent = texto;
    return div.innerHTML;
}

const MAX_AVISOS = 3;

function avisar(mensagem, tipo = "sucesso") {
    const toast = document.createElement("div");
    toast.className = `toast ${tipo}`;
    toast.textContent = mensagem;
    $("#toasts").append(toast);
    setTimeout(() => toast.remove(), 4000);

    // mantém só os avisos mais recentes na tela
    const avisos = $("#toasts").children;
    while (avisos.length > MAX_AVISOS) avisos[0].remove();
}

async function api(metodo, caminho, corpo) {
    const resposta = await fetch(API_URL + caminho, {
        method: metodo,
        headers: corpo ? { "Content-Type": "application/json" } : {},
        body: corpo ? JSON.stringify(corpo) : undefined,
    });
    if (!resposta.ok) throw new Error(`${metodo} ${caminho} respondeu ${resposta.status}`);
    return resposta.json();
}

function carregarMeusPedidos() {
    try {
        return JSON.parse(localStorage.getItem("meusPedidos")) || [];
    } catch {
        return [];
    }
}

function salvarMeusPedidos() {
    try {
        localStorage.setItem("meusPedidos", JSON.stringify(estado.meusPedidos));
    } catch { /* sem localStorage: os pedidos valem só para esta sessão */ }
}

// -----------------------------------------------------------------------------
// Navegação entre as abas (#catalogo, #pedidos, #promocoes)
// -----------------------------------------------------------------------------
function mostrarPagina() {
    const atual = location.hash.slice(1) || "catalogo";
    document.querySelectorAll(".pagina").forEach((secao) => {
        secao.hidden = secao.id !== atual;
    });
    document.querySelectorAll("nav a").forEach((link) => {
        link.classList.toggle("ativo", link.getAttribute("href") === `#${atual}`);
    });
}

// -----------------------------------------------------------------------------
// Catálogo — GET /products
// -----------------------------------------------------------------------------
async function carregarProdutos() {
    try {
        estado.produtos = (await api("GET", "/products")) || [];
        estado.produtos.sort((a, b) => a.name.localeCompare(b.name));
        renderizarProdutos();
        renderizarCarrinho();
    } catch (erro) {
        $("#produtos").innerHTML = `<p class="vazio">Não foi possível carregar o catálogo (${escaparHtml(erro.message)}).</p>`;
    }
}

function renderizarProdutos() {
    $("#produtos").innerHTML = estado.produtos.map((produto) => `
        <article class="produto">
            <h3>${escaparHtml(produto.name)}</h3>
            <span class="preco">${formatarPreco(produto.price)}</span>
            <span class="estoque">${produto.quantity > 0 ? `${produto.quantity} em estoque` : "Esgotado"}</span>
            <button class="btn" data-adicionar="${produto.uuid}" ${produto.quantity > 0 ? "" : "disabled"}>
                Adicionar ao carrinho
            </button>
        </article>
    `).join("");
}

// -----------------------------------------------------------------------------
// Carrinho e criação do pedido — POST /orders
// -----------------------------------------------------------------------------
function alterarCarrinho(uuid, delta) {
    const produto = estado.produtos.find((p) => p.uuid === uuid);
    const quantidade = (estado.carrinho.get(uuid) || 0) + delta;

    if (quantidade <= 0) estado.carrinho.delete(uuid);
    else estado.carrinho.set(uuid, Math.min(quantidade, produto ? produto.quantity : quantidade));

    renderizarCarrinho();
}

function renderizarCarrinho() {
    let total = 0;
    const itens = [...estado.carrinho].map(([uuid, quantidade]) => {
        const produto = estado.produtos.find((p) => p.uuid === uuid);
        if (!produto) return "";
        total += produto.price * quantidade;
        return `
            <li>
                <span>${escaparHtml(produto.name)}</span>
                <span class="carrinho-qtd">
                    <button class="btn btn-secundario btn-pequeno" data-diminuir="${uuid}">−</button>
                    ${quantidade}
                    <button class="btn btn-secundario btn-pequeno" data-aumentar="${uuid}">+</button>
                </span>
            </li>`;
    });

    $("#carrinho-itens").innerHTML = itens.join("") || `<li class="vazio">Carrinho vazio</li>`;
    $("#carrinho-total").textContent = formatarPreco(total);
    $("#btn-finalizar").disabled = estado.carrinho.size === 0;
}

async function finalizarPedido() {
    // A aba é aberta já no clique (senão o navegador bloqueia o pop-up) e recebe
    // a URL do Mock de Pagamento quando o evento de checkout chegar pelo SSE.
    const aba = window.open("", "_blank");
    if (aba) aba.document.write("<title>Pagamento</title><p style='font-family:sans-serif'>Aguardando o link de pagamento…</p>");

    const produtos = [...estado.carrinho].map(([uuid, quantidade]) => ({ ProductId: uuid, Quantity: quantidade }));

    try {
        const pedido = await api("POST", "/orders", { Products: produtos });
        estado.meusPedidos.unshift(pedido.UUID);
        salvarMeusPedidos();
        if (aba) estado.abasPagamento.set(pedido.UUID, aba);

        receberPedido(pedido);
        // passa a acompanhar o novo pedido pelo SSE (o estado atual chega ao reconectar)
        conectarEventos();

        estado.carrinho.clear();
        renderizarCarrinho();
        avisar("Pedido criado! Acompanhe o status em Pedidos.");
        location.hash = "#pedidos";
    } catch (erro) {
        if (aba) aba.close();
        avisar(`Falha ao criar o pedido: ${erro.message}`, "erro");
    }
}

// -----------------------------------------------------------------------------
// Acompanhamento dos pedidos — SSE em GET /events?orderId=...
// A API só transmite os pedidos informados; a cada pedido novo a conexão é refeita
// com a lista atualizada (ao conectar, a API envia o estado atual de cada um).
// -----------------------------------------------------------------------------
const MAX_PEDIDOS_ACOMPANHADOS = 50;

function conectarEventos() {
    if (estado.fonteEventos) estado.fonteEventos.close();

    const ids = estado.meusPedidos.slice(0, MAX_PEDIDOS_ACOMPANHADOS);
    const parametros = new URLSearchParams(ids.map((id) => ["orderId", id]));
    const fonte = new EventSource(`${API_URL}/events?${parametros}`);
    estado.fonteEventos = fonte;
    const indicador = $("#conexao");

    fonte.onopen = () => {
        indicador.textContent = "Tempo real ativo";
        indicador.className = "conexao online";
    };
    // o EventSource reconecta sozinho; ao voltar, a API reenvia o estado atual
    fonte.onerror = () => {
        indicador.textContent = "Reconectando…";
        indicador.className = "conexao offline";
    };
    fonte.addEventListener("order", (evento) => receberPedido(JSON.parse(evento.data)));
}

function receberPedido(pedido) {
    const anterior = estado.pedidos.get(pedido.UUID);
    estado.pedidos.set(pedido.UUID, pedido);

    if (!estado.meusPedidos.includes(pedido.UUID)) return;

    if (anterior && anterior.Status !== pedido.Status && MENSAGENS[pedido.Status]) {
        avisar(MENSAGENS[pedido.Status], FALHAS[pedido.Status] ? "erro" : "sucesso");
    }

    // estoque mudou: atualiza o catálogo
    if (anterior && anterior.Status !== pedido.Status && pedido.Status !== "OrderCreated") {
        carregarProdutos();
    }

    encaminharAbaPagamento(pedido);
    renderizarPedidos();
}

function encaminharAbaPagamento(pedido) {
    const aba = estado.abasPagamento.get(pedido.UUID);
    if (!aba) return;

    if (pedido.CheckoutUrl) {
        aba.location.href = pedido.CheckoutUrl;
        estado.abasPagamento.delete(pedido.UUID);
    } else if (FALHAS[pedido.Status]) {
        aba.close();
        estado.abasPagamento.delete(pedido.UUID);
    }
}

function renderizarPedidos() {
    const pedidos = estado.meusPedidos.map((uuid) => estado.pedidos.get(uuid)).filter(Boolean);

    const contador = $("#contador-pedidos");
    contador.hidden = pedidos.length === 0;
    contador.textContent = pedidos.length;

    if (pedidos.length === 0) {
        $("#lista-pedidos").innerHTML = `<p class="vazio">Nenhum pedido feito neste navegador ainda.</p>`;
        return;
    }
    $("#lista-pedidos").innerHTML = pedidos.map(renderizarPedido).join("");
}

function renderizarPedido(pedido) {
    const falha = FALHAS[pedido.Status];
    const etapaAtual = falha ? falha.etapa - 1 : ETAPAS.findIndex((e) => e.status === pedido.Status);

    const etapas = ETAPAS.map((etapa, i) => {
        if (falha && i === falha.etapa) return `<li class="falhou">${falha.rotulo}</li>`;
        return `<li class="${i <= etapaAtual ? "feita" : ""}">${etapa.rotulo}</li>`;
    }).join("");

    const itens = (pedido.Products || [])
        .map((p) => `${p.quantity}× ${escaparHtml(p.name)}`)
        .join(" · ");
    const total = (pedido.Products || []).reduce((soma, p) => soma + p.price * p.quantity, 0);

    let selo = `<span class="selo">Em andamento</span>`;
    if (falha) selo = `<span class="selo erro">${falha.rotulo}</span>`;
    if (pedido.Status === "OrderSent") selo = `<span class="selo sucesso">Concluído</span>`;

    // o link de pagamento fica disponível enquanto o pagamento não for decidido
    const botaoPagamento = pedido.CheckoutUrl && pedido.Status === "StockReserved"
        ? `<a class="btn" href="${escaparHtml(pedido.CheckoutUrl)}" target="_blank">Ir para o pagamento</a>`
        : "";

    return `
        <article class="secao-bloco pedido">
            <div class="pedido-cabecalho">
                <h3>Pedido <span class="pedido-id">${pedido.UUID.slice(0, 8)}</span></h3>
                ${selo}
            </div>
            <p class="pedido-itens">${itens} — <strong>${formatarPreco(total)}</strong></p>
            <ol class="etapas">${etapas}</ol>
            ${botaoPagamento}
        </article>`;
}

// -----------------------------------------------------------------------------
// Promoções — POST / DELETE /sales/register
// O backend registra uma categoria por requisição, então enviamos uma por categoria marcada.
// -----------------------------------------------------------------------------
async function alterarInteresse(evento, metodo) {
    evento.preventDefault();
    const formulario = $("#form-promocoes");
    if (!formulario.reportValidity()) return;

    const email = $("#email").value.trim();
    const categorias = [...document.querySelectorAll("input[name=categoria]:checked")].map((c) => c.value);
    if (categorias.length === 0) {
        avisar("Selecione ao menos uma categoria.", "erro");
        return;
    }

    try {
        await Promise.all(categorias.map((categoria) =>
            api(metodo, "/sales/register", { Email: email, Category: categoria })));
        const lista = categorias.join(", ");
        avisar(metodo === "POST"
            ? `Inscrito nas promoções das categorias: ${lista}.`
            : `Interesse cancelado nas categorias: ${lista}.`);
    } catch (erro) {
        avisar(`Não foi possível atualizar o interesse: ${erro.message}`, "erro");
    }
}

// -----------------------------------------------------------------------------
// Inicialização
// -----------------------------------------------------------------------------
document.addEventListener("click", (evento) => {
    const { adicionar, aumentar, diminuir } = evento.target.dataset;
    if (adicionar) alterarCarrinho(adicionar, +1);
    if (aumentar) alterarCarrinho(aumentar, +1);
    if (diminuir) alterarCarrinho(diminuir, -1);
});

$("#btn-finalizar").addEventListener("click", finalizarPedido);
$("#form-promocoes").addEventListener("submit", (e) => alterarInteresse(e, "POST"));
$("#btn-cancelar-interesse").addEventListener("click", (e) => alterarInteresse(e, "DELETE"));
window.addEventListener("hashchange", mostrarPagina);

mostrarPagina();
renderizarCarrinho();
renderizarPedidos();
carregarProdutos();
conectarEventos();
