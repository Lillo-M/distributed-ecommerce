package main

import (
	"bytes"
	"eCommerce/pkg/requests"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/joho/godotenv"
)
type Charge struct {
	OrderId    string
	WebhookUrl string
	Status     string
}

type PaymentMockService struct {
	ListenURL  string
	PublicURL  string
	Charges    map[string]*Charge
	mutex      sync.RWMutex
	httpClient *http.Client
}

func CreatePaymentMockService() (*PaymentMockService, error) {
	_ = godotenv.Load()

	var paymentMock *PaymentMockService = &PaymentMockService{
		ListenURL: os.Getenv("PAYMENT_MOCK_URL"),
		PublicURL: os.Getenv("PAYMENT_MOCK_PUBLIC_URL"),
		Charges:   make(map[string]*Charge),
		httpClient: &http.Client{Timeout: 10 * time.Second},
	}

	if paymentMock.ListenURL == "" {
		return nil, fmt.Errorf("PAYMENT_MOCK_URL is required")
	}
	if paymentMock.PublicURL == "" {
		paymentMock.PublicURL = paymentMock.ListenURL
	}

	return paymentMock, nil
}

func (this *PaymentMockService) PostCharge(context *gin.Context) {
	var request requests.CreateCobrancaRequest
	if err := context.ShouldBindJSON(&request); err != nil {
		context.IndentedJSON(http.StatusBadRequest, gin.H{
			"success": false,
			"error":   err.Error(),
		})
		return
	}
	if request.OrderId == "" || request.WebhookUrl == "" {
		context.IndentedJSON(http.StatusBadRequest, gin.H{
			"success": false,
			"error":   "orderId e webhookUrl são obrigatórios",
		})
		return
	}

	this.mutex.Lock()
	this.Charges[request.OrderId] = &Charge{
		OrderId:    request.OrderId,
		WebhookUrl: request.WebhookUrl,
		Status:     "PENDENTE",
	}
	this.mutex.Unlock()

	context.IndentedJSON(http.StatusCreated, requests.CreateCobrancaResponse{
		OrderId:     request.OrderId,
		CheckoutUrl: "http://" + this.PublicURL + "/checkout/" + request.OrderId,
	})
}

func (this *PaymentMockService) GetCheckout(context *gin.Context) {
	orderId := context.Param("orderId")

	this.mutex.RLock()
	storedCharge, found := this.Charges[orderId]
	var charge Charge
	if found {
		charge = *storedCharge
	}
	this.mutex.RUnlock()

	if !found {
		context.String(http.StatusNotFound, "Cobrança não encontrada")
		return
	}

	context.Header("Content-Type", "text/html; charset=utf-8")
	if err := checkoutTemplate.Execute(context.Writer, charge); err != nil {
		log.Printf("Failed to render checkout page: %v\n", err)
	}
}
func (this *PaymentMockService) PostApprove(context *gin.Context) {
	this.decideCharge(context, "APROVADO")
}

func (this *PaymentMockService) PostRefuse(context *gin.Context) {
	this.decideCharge(context, "RECUSADO")
}

func (this *PaymentMockService) decideCharge(context *gin.Context, status string) {
	orderId := context.Param("orderId")

	this.mutex.Lock()
	charge, found := this.Charges[orderId]
	if !found {
		this.mutex.Unlock()
		context.String(http.StatusNotFound, "Cobrança não encontrada")
		return
	}
	if charge.Status != "PENDENTE" {
		this.mutex.Unlock()
		context.Redirect(http.StatusSeeOther, "/checkout/"+orderId)
		return
	}
	charge.Status = "PROCESSANDO"
	webhookUrl := charge.WebhookUrl
	this.mutex.Unlock()

	err := this.sendWebhook(webhookUrl, requests.PagamentoWebhookRequest{
		OrderId: orderId,
		Status:  status,
	})

	this.mutex.Lock()
	if err != nil {
		charge.Status = "PENDENTE"
	} else {
		charge.Status = status
	}
	this.mutex.Unlock()

	if err != nil {
		log.Printf("Failed to send webhook for order '%s': %v\n", orderId, err)
		context.String(http.StatusBadGateway, "Falha ao notificar o MS Pagamento: %v", err)
		return
	}

	log.Printf("Webhook sent for order '%s' with status: %s\n", orderId, status)
	context.Redirect(http.StatusSeeOther, "/checkout/"+orderId)
}

func (this *PaymentMockService) sendWebhook(webhookUrl string, payload requests.PagamentoWebhookRequest) error {
	body, err := json.Marshal(payload)
	if err != nil {
		return err
	}

	resp, err := this.httpClient.Post(webhookUrl, "application/json", bytes.NewBuffer(body))
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		responseBody, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("status %d: %s", resp.StatusCode, string(responseBody))
	}

	return nil
}