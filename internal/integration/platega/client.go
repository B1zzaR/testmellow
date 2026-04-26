// Package platega implements the Platega.io payment API client.
// Based on: https://docs.platega.io/
//
// Authentication: headers X-MerchantId and X-Secret on every request.
// Payment creation: POST https://app.platega.io/transaction/process
// Webhook: POST on your endpoint; Platega sends X-MerchantId + X-Secret + JSON body.
package platega

import (
	"bytes"
	"context"
	"crypto/subtle"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/vpnplatform/internal/config"
	"go.uber.org/zap"
)

// PaymentMethod codes as per Platega docs
const (
	MethodSBPQR       = 2  // СБП QR
	MethodERIP        = 3  // ЕРИП
	MethodCardAcquire = 11 // Карточный эквайринг
	MethodIntlCard    = 12 // Международная оплата
	MethodCrypto      = 13 // Криптовалюта
)

// PaymentStatus values returned in callback and status check
type PaymentStatus string

const (
	StatusPending      PaymentStatus = "PENDING"
	StatusConfirmed    PaymentStatus = "CONFIRMED"
	StatusCanceled     PaymentStatus = "CANCELED"
	StatusChargebacked PaymentStatus = "CHARGEBACKED"
)

// CreatePaymentRequest mirrors POST /transaction/process body.
// Per Platega docs, description/return/failedUrl/payload are top-level fields;
// paymentDetails only carries amount and currency.
type CreatePaymentRequest struct {
	PaymentMethod  int            `json:"paymentMethod"`
	PaymentDetails PaymentDetails `json:"paymentDetails"`
	Description    string         `json:"description"`         // required
	Return         string         `json:"return,omitempty"`    // redirect on success
	FailedURL      string         `json:"failedUrl,omitempty"` // redirect on failure
	Payload        string         `json:"payload,omitempty"`   // our internal reference
}

type PaymentDetails struct {
	Amount   float64 `json:"amount"`   // in RUB (rubles, not kopecks)
	Currency string  `json:"currency"` // "RUB"
}

// CreatePaymentResponse mirrors success response of POST /transaction/process
type CreatePaymentResponse struct {
	PaymentMethod  string        `json:"paymentMethod"`
	TransactionID  string        `json:"transactionId"`
	Redirect       string        `json:"redirect"`
	Return         string        `json:"return"`
	PaymentDetails interface{}   `json:"paymentDetails"`
	Status         PaymentStatus `json:"status"`
	ExpiresIn      string        `json:"expiresIn"`
	MerchantID     string        `json:"merchantId"`
	USDTRate       float64       `json:"usdtRate"`
}

// CallbackPayload is the webhook request body Platega sends to our endpoint.
// Platega also sends X-MerchantId and X-Secret headers for verification.
type CallbackPayload struct {
	ID            string        `json:"id"` // transactionId (UUID)
	Amount        float64       `json:"amount"`
	Currency      string        `json:"currency"`
	Status        PaymentStatus `json:"status"`
	PaymentMethod int           `json:"paymentMethod"`
	Payload       string        `json:"payload,omitempty"`
}

// Client is the Platega API client
type Client struct {
	httpClient *http.Client
	cfg        config.PlategalConfig
	log        *zap.Logger
}

func NewClient(cfg config.PlategalConfig, log *zap.Logger) *Client {
	return &Client{
		httpClient: &http.Client{Timeout: 30 * time.Second},
		cfg:        cfg,
		log:        log,
	}
}

// CreatePayment calls POST /transaction/process
func (c *Client) CreatePayment(ctx context.Context, req CreatePaymentRequest) (*CreatePaymentResponse, error) {
	body, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("platega: marshal request: %w", err)
	}

	c.log.Info("platega: sending CreatePayment request",
		zap.String("url", c.cfg.BaseURL+"/transaction/process"),
	)

	httpReq, err := http.NewRequestWithContext(ctx,
		http.MethodPost,
		c.cfg.BaseURL+"/transaction/process",
		bytes.NewReader(body),
	)
	if err != nil {
		return nil, err
	}
	c.setHeaders(httpReq)

	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("platega: http do: %w", err)
	}
	defer resp.Body.Close()

	respBody, _ := io.ReadAll(resp.Body)

	c.log.Info("platega: received response",
		zap.Int("status", resp.StatusCode),
	)

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("platega: unexpected status %d: %s", resp.StatusCode, string(respBody))
	}

	var result CreatePaymentResponse
	if err := json.Unmarshal(respBody, &result); err != nil {
		return nil, fmt.Errorf("platega: decode response: %w", err)
	}
	return &result, nil
}

// GetPaymentStatus calls GET /transaction/{transactionId}
func (c *Client) GetPaymentStatus(ctx context.Context, transactionID string) (*CreatePaymentResponse, error) {
	httpReq, err := http.NewRequestWithContext(ctx,
		http.MethodGet,
		c.cfg.BaseURL+"/transaction/"+transactionID,
		nil,
	)
	if err != nil {
		return nil, err
	}
	c.setHeaders(httpReq)

	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("platega: http do: %w", err)
	}
	defer resp.Body.Close()

	respBody, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("platega: status check returned %d: %s", resp.StatusCode, string(respBody))
	}

	var result CreatePaymentResponse
	if err := json.Unmarshal(respBody, &result); err != nil {
		return nil, fmt.Errorf("platega: decode response: %w", err)
	}
	return &result, nil
}

// VerifyWebhookHeaders validates the webhook came from Platega using
// constant-time comparison to prevent timing-based credential guessing (H-3).
//
// Both PLATEGA_SECRET (current) and PLATEGA_SECRET_PREV (previous) are
// accepted, so an operator can rotate the secret in the Platega dashboard
// without losing in-flight callbacks. After 24–48 h PLATEGA_SECRET_PREV
// must be cleared so a leaked old secret stops working.
func (c *Client) VerifyWebhookHeaders(merchantID, secret string) bool {
	if subtle.ConstantTimeCompare([]byte(merchantID), []byte(c.cfg.MerchantID)) != 1 {
		return false
	}
	if subtle.ConstantTimeCompare([]byte(secret), []byte(c.cfg.Secret)) == 1 {
		return true
	}
	if c.cfg.SecretPrev != "" &&
		subtle.ConstantTimeCompare([]byte(secret), []byte(c.cfg.SecretPrev)) == 1 {
		return true
	}
	return false
}

func (c *Client) setHeaders(req *http.Request) {
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-MerchantId", c.cfg.MerchantID)
	req.Header.Set("X-Secret", c.cfg.Secret)
}
