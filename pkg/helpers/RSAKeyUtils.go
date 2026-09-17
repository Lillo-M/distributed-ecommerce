package helpers

import (
	"crypto"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/x509"
	"eCommerce/pkg/events"
	"encoding/pem"
	"errors"
	"fmt"
	"os"
	"strings"

	amqp "github.com/rabbitmq/amqp091-go"
)

func ReadPrivateKeyPEM(filePath string) (*rsa.PrivateKey, error) {
	// 1. Read the file
	pemBytes, err := os.ReadFile(filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to read key file: %w", err)
	}

	// 2. Decode the PEM block
	block, _ := pem.Decode(pemBytes)
	if block == nil {
		return nil, errors.New("failed to parse PEM block: no valid PEM data found")
	}

	// 3. Parse the binary key data based on key format
	switch block.Type {
	case "RSA PRIVATE KEY": // PKCS#1 format
		return x509.ParsePKCS1PrivateKey(block.Bytes)

	case "PRIVATE KEY": // PKCS#8 format
		key, err := x509.ParsePKCS8PrivateKey(block.Bytes)
		if err != nil {
			return nil, err
		}
		rsaKey, ok := key.(*rsa.PrivateKey)
		if !ok {
			return nil, errors.New("key in PKCS#8 container is not an RSA private key")
		}
		return rsaKey, nil

	default:
		return nil, fmt.Errorf("unsupported key type: %s", block.Type)
	}
}

func ReadPublicKeyPEM(filePath string) (*rsa.PublicKey, error) {
	pemBytes, err := os.ReadFile(filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to read key file: %w", err)
	}

	block, _ := pem.Decode(pemBytes)
	if block == nil {
		return nil, errors.New("failed to parse PEM block")
	}

	switch block.Type {
	case "PUBLIC KEY": // Standard PKIX format
		pubInterface, err := x509.ParsePKIXPublicKey(block.Bytes)
		if err != nil {
			return nil, err
		}
		rsaPub, ok := pubInterface.(*rsa.PublicKey)
		if !ok {
			return nil, errors.New("public key is not of type RSA")
		}
		return rsaPub, nil

	case "RSA PUBLIC KEY": // PKCS#1 format
		return x509.ParsePKCS1PublicKey(block.Bytes)

	default:
		return nil, fmt.Errorf("unsupported public key type: %s", block.Type)
	}
}

func VerifyMessage(delivery amqp.Delivery, pubKey *rsa.PublicKey) error {
	sigRaw, ok := delivery.Headers["x-signature"]
	if !ok {
		return fmt.Errorf("cabeçalho 'x-signature' ausente")
	}

	signature, ok := sigRaw.([]byte)
	if !ok {
		return fmt.Errorf("formato de assinatura inválido")
	}

	hashed := sha256.Sum256(delivery.Body)

	if err := rsa.VerifyPSS(pubKey, crypto.SHA256, hashed[:], signature, nil); err != nil {
		return fmt.Errorf("assinatura RSA inválida: %w", err)
	}

	return nil
}

func GetProducerPublicKey(message amqp.Delivery) (*rsa.PublicKey, error) {
	var producerKey *rsa.PublicKey
	var err error
	if strings.HasPrefix(message.RoutingKey, "promocao") {
		producerKey, err = ReadPublicKeyPEM("./pkg/public-keys/promotions.pem")
		return producerKey, err
	}
	switch message.RoutingKey {
	case events.PaymentEventApproved.String():
		producerKey, err = ReadPublicKeyPEM("./pkg/public-keys/payments.pem")
	case events.PaymentEventRefused.String():
		producerKey, err = ReadPublicKeyPEM("./pkg/public-keys/payments.pem")
	case events.OrderEventCreated.String():
		producerKey, err = ReadPublicKeyPEM("./pkg/public-keys/main.pem")
	case events.OrderEventDeleted.String():
		producerKey, err = ReadPublicKeyPEM("./pkg/public-keys/main.pem")
	case events.RoutingProdutosConsultar:
		producerKey, err = ReadPublicKeyPEM("./pkg/public-keys/main.pem")
	case events.OrderEventSent.String():
		producerKey, err = ReadPublicKeyPEM("./pkg/public-keys/delivery.pem")
	case events.OrderEventStockOk.String():
		producerKey, err = ReadPublicKeyPEM("./pkg/public-keys/stock.pem")
	case events.StockEventUnavailable.String():
		producerKey, err = ReadPublicKeyPEM("./pkg/public-keys/stock.pem")

	default:
		err = fmt.Errorf("Evento %s inválido", message.RoutingKey)
	}
	return producerKey, err
}
