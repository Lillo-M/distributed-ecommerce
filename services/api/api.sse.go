package main

import (
	"eCommerce/pkg/mock"
	"io"
	"log"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
)

const (
	sseClientBufferSize = 16
	sseHeartbeatPeriod  = 15 * time.Second
)

// registra um novo cliente SSE e retorna o canal por onde ele recebe as atualizações dos pedidos
func (this *MainApiService) subscribeSSE() chan mock.OrderInfo {
	client := make(chan mock.OrderInfo, sseClientBufferSize)

	this.sseMutex.Lock()
	if this.sseClients == nil {
		this.sseClients = make(map[chan mock.OrderInfo]struct{})
	}
	this.sseClients[client] = struct{}{}
	this.sseMutex.Unlock()

	return client
}

func (this *MainApiService) unsubscribeSSE(client chan mock.OrderInfo) {
	this.sseMutex.Lock()
	delete(this.sseClients, client)
	this.sseMutex.Unlock()
}

// envia a nova versão do pedido para todos os clientes SSE conectados
func (this *MainApiService) broadcastOrder(order mock.OrderInfo) {
	this.sseMutex.Lock()
	defer this.sseMutex.Unlock()

	for client := range this.sseClients {
		select {
		case client <- order:
		default:
			log.Printf("SSE client too slow, dropping update of order '%v'\n", order.UUID)
		}
	}
}

// GET /events?orderId=<id>&orderId=<id>...
// mantém uma conexão SSE com o frontend notificando toda mudança de estado
// apenas dos pedidos daquele cliente (os informados em orderId)
func (this *MainApiService) GetEvents(context *gin.Context) {
	clientOrders := make(map[string]bool)
	for _, orderId := range context.QueryArray("orderId") {
		clientOrders[orderId] = true
	}

	client := this.subscribeSSE()
	defer this.unsubscribeSSE(client)

	context.Header("Content-Type", "text/event-stream")
	context.Header("Cache-Control", "no-cache")
	context.Header("Connection", "keep-alive")
	context.Status(http.StatusOK)

	// estado atual primeiro, para o cliente não perder eventos anteriores à conexão
	this.ordersMutex.RLock()
	for orderId := range clientOrders {
		if order, ok := this.Orders[orderId]; ok {
			context.SSEvent("order", order)
		}
	}
	this.ordersMutex.RUnlock()
	context.Writer.Flush()

	heartbeat := time.NewTicker(sseHeartbeatPeriod)
	defer heartbeat.Stop()

	context.Stream(func(w io.Writer) bool {
		select {
		case <-context.Request.Context().Done():
			return false
		case <-heartbeat.C:
			context.SSEvent("ping", time.Now().Unix())
			return true
		case order := <-client:
			if clientOrders[order.UUID] {
				context.SSEvent("order", order)
			}
			return true
		}
	})
}

// libera o acesso do frontend (servido em outra origem) à API REST e ao SSE
func CorsMiddleware() gin.HandlerFunc {
	return func(context *gin.Context) {
		context.Header("Access-Control-Allow-Origin", "*")
		context.Header("Access-Control-Allow-Methods", "GET, POST, DELETE, OPTIONS")
		context.Header("Access-Control-Allow-Headers", "Content-Type")

		if context.Request.Method == http.MethodOptions {
			context.AbortWithStatus(http.StatusNoContent)
			return
		}
		context.Next()
	}
}
