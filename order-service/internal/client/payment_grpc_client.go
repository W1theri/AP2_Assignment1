package client

import (
	"context"
	"fmt"
	"time"

	"github.com/W1theri/ap2-generated/codec/jsoncodec"
	pb "github.com/W1theri/ap2-generated/payment"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"

	"order-service/internal/usecase"
)

// GRPCPaymentClient adapts the payment gRPC API to the use case port.
type GRPCPaymentClient struct {
	client pb.PaymentServiceClient
}

// NewGRPCPaymentClient creates a client using the configured gRPC target.
func NewGRPCPaymentClient(addr string) (*GRPCPaymentClient, error) {
	conn, err := grpc.NewClient(
		addr,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithDefaultCallOptions(grpc.CallContentSubtype(jsoncodec.Name)),
	)
	if err != nil {
		return nil, fmt.Errorf("grpc dial %s: %w", addr, err)
	}

	return &GRPCPaymentClient{
		client: pb.NewPaymentServiceClient(conn),
	}, nil
}

// Authorize preserves the assignment timeout guarantee for synchronous payment calls.
func (c *GRPCPaymentClient) Authorize(
	ctx context.Context,
	orderID string,
	amount int64,
	customerEmail string,
) (*usecase.PaymentResult, error) {
	ctx, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()

	resp, err := c.client.ProcessPayment(ctx, &pb.PaymentRequest{
		OrderId:       orderID,
		Amount:        amount,
		CustomerEmail: customerEmail,
	})
	if err != nil {
		return nil, fmt.Errorf("grpc ProcessPayment: %w", err)
	}

	return &usecase.PaymentResult{
		TransactionID: resp.TransactionId,
		Status:        resp.Status,
	}, nil
}
