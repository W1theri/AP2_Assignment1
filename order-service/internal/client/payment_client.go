package client

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"order-service/internal/usecase"
	"time"
)

// paymentRequest mirrors the JSON payload expected by the Payment Service.
type paymentRequest struct {
	OrderID string `json:"order_id"`
	Amount  int64  `json:"amount"`
}

// paymentResponse mirrors the JSON response from the Payment Service.
type paymentResponse struct {
	TransactionID string `json:"transaction_id"`
	Status        string `json:"status"`
}

// HTTPPaymentClient is the concrete adapter that implements usecase.PaymentClient.
// It communicates with the Payment Service via REST.
type HTTPPaymentClient struct {
	httpClient     *http.Client
	paymentBaseURL string
}

// NewHTTPPaymentClient creates the client with a hard 2-second timeout
// as required by the assignment (Order Service must not hang indefinitely).
func NewHTTPPaymentClient(paymentBaseURL string) *HTTPPaymentClient {
	return &HTTPPaymentClient{
		// A shared http.Client instance is used (initialised once at the Composition Root).
		// The 2-second timeout ensures that if Payment Service is down or slow,
		// the Order Service trips quickly and marks the order as Failed.
		httpClient: &http.Client{
			Timeout: 2 * time.Second,
		},
		paymentBaseURL: paymentBaseURL,
	}
}

// Authorize sends a POST /payments request to the Payment Service.
// Returns usecase.PaymentResult on success, or an error if the service
// is unavailable, times out, or returns a non-2xx status code.
func (c *HTTPPaymentClient) Authorize(ctx context.Context, orderID string, amount int64) (*usecase.PaymentResult, error) {
	payload, err := json.Marshal(paymentRequest{OrderID: orderID, Amount: amount})
	if err != nil {
		return nil, fmt.Errorf("failed to marshal payment request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost,
		c.paymentBaseURL+"/payments",
		bytes.NewBuffer(payload),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to build payment request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		// Covers: timeout (context.DeadlineExceeded), connection refused, DNS failure.
		return nil, fmt.Errorf("payment service unavailable: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 500 {
		return nil, fmt.Errorf("payment service returned %d", resp.StatusCode)
	}

	var payResp paymentResponse
	if err := json.NewDecoder(resp.Body).Decode(&payResp); err != nil {
		return nil, fmt.Errorf("failed to decode payment response: %w", err)
	}

	return &usecase.PaymentResult{
		TransactionID: payResp.TransactionID,
		Status:        payResp.Status,
	}, nil
}
